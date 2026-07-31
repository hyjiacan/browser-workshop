package serve

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
)

// mockSyncSource is a test-only SyncSource that returns canned versions and
// "downloads" by writing a local file (no network).
type mockSyncSource struct {
	versions  []SyncVersionInfo
	downloads int32 // atomic counter of Download calls
}

func (m *mockSyncSource) ListVersions(browser, channel, platform, arch string) ([]SyncVersionInfo, error) {
	return m.versions, nil
}

func (m *mockSyncSource) Download(u string, destDir string, onProgress func(int64, int64)) (string, error) {
	atomic.AddInt32(&m.downloads, 1)
	// Mirror DefaultDownload: filename is the (un-decoded) base of the URL.
	fname := filepath.Base(u)
	dest := filepath.Join(destDir, fname)
	if err := os.WriteFile(dest, []byte("mock content: "+fname), 0o644); err != nil {
		return "", err
	}
	return dest, nil
}

func newFallbackTestServer(t *testing.T, onlineFallback bool, src SyncSource) *Server {
	t.Helper()
	dir := t.TempDir()
	packagesDir := filepath.Join(dir, "packages")
	if err := os.MkdirAll(packagesDir, 0o755); err != nil {
		t.Fatalf("mkdir packages: %v", err)
	}
	s := NewServerWithOptions(ServerOptions{
		Addr:           ":0",
		Version:        "test",
		PackagesDir:    packagesDir,
		BinDir:         filepath.Join(dir, "bin"),
		OnlineSource:   src,
		OnlineFallback: onlineFallback,
	})
	cache, _ := s.loadCache()
	if err := s.scanPackages(cache); err != nil {
		t.Fatalf("scanPackages: %v", err)
	}
	return s
}

func doManifest(t *testing.T, s *Server) ManifestResponse {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/manifest", nil)
	rec := httptest.NewRecorder()
	s.handleManifest(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("manifest status = %d, want 200", rec.Code)
	}
	var resp ManifestResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal manifest: %v", err)
	}
	return resp
}

// downloadReq builds a GET request for a package download with the filename
// properly URL-escaped in the path (so filenames containing spaces etc. are
// valid request targets). The handler decodes r.URL.Path back to the raw name.
func downloadReq(filename string) *http.Request {
	target := "/api/v1/download/" + url.PathEscape(filename)
	return httptest.NewRequest(http.MethodGet, target, nil)
}

// TestOnlineFallback_ManifestMerge verifies that, with online fallback enabled,
// the manifest endpoint lists online-available versions even when nothing is
// cached locally, and that URL-encoded filenames are decoded.
func TestOnlineFallback_ManifestMerge(t *testing.T) {
	src := &mockSyncSource{
		versions: []SyncVersionInfo{
			{
				Browser:     "firefox",
				Version:     "141.0",
				Channel:     "stable",
				Platform:    "windows",
				Arch:        "amd64",
				DownloadURL: "https://example.com/firefox/141.0/Firefox%20Setup%20141.0.exe",
			},
			{
				Browser:     "chrome",
				Version:     "120.0.6099.109",
				Channel:     "stable",
				Platform:    "windows",
				Arch:        "amd64",
				DownloadURL: "https://example.com/chrome/120.0.6099.109_win64_installer.exe",
			},
		},
	}
	s := newFallbackTestServer(t, true, src)

	// Manifest should return online-cached versions merged with local (0 local).
	resp := doManifest(t, s)

	wantNames := map[string]bool{
		"Firefox Setup 141.0.exe":            false,
		"120.0.6099.109_win64_installer.exe": false,
	}
	for _, f := range resp.Data {
		if _, ok := wantNames[f.Filename]; ok {
			wantNames[f.Filename] = true
		}
	}
	for name, found := range wantNames {
		if !found {
			t.Errorf("manifest missing online file %q; got %d entries", name, len(resp.Data))
		}
	}
}

// TestOnlineFallback_ManifestDisabled verifies that with the flag off, online
// versions are NOT injected into the manifest.
func TestOnlineFallback_ManifestDisabled(t *testing.T) {
	src := &mockSyncSource{
		versions: []SyncVersionInfo{
			{Browser: "firefox", Version: "141.0", DownloadURL: "https://example.com/Firefox%20Setup%20141.0.exe"},
		},
	}
	s := newFallbackTestServer(t, false, src)

	resp := doManifest(t, s)
	for _, f := range resp.Data {
		if f.Filename == "Firefox Setup 141.0.exe" {
			t.Errorf("online file appeared in manifest despite fallback being disabled")
		}
	}
}

// TestOnlineFallback_DownloadFetchAndCache verifies that a download miss is
// fetched on demand, the file is renamed to the canonical (decoded) filename,
// cached locally, and subsequent requests are served from the local cache
// without re-downloading.
func TestOnlineFallback_DownloadFetchAndCache(t *testing.T) {
	src := &mockSyncSource{
		versions: []SyncVersionInfo{
			{
				Browser:     "firefox",
				Version:     "141.0",
				Channel:     "stable",
				Platform:    "windows",
				Arch:        "amd64",
				DownloadURL: "https://example.com/firefox/141.0/Firefox%20Setup%20141.0.exe",
			},
		},
	}
	s := newFallbackTestServer(t, true, src)
	wantFile := "Firefox Setup 141.0.exe"

	// First request: not present locally -> fetched from online source.
	req := downloadReq(wantFile)
	rec := httptest.NewRecorder()
	s.handleDownload(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("first download status = %d, want 200 (body=%q)", rec.Code, rec.Body.String())
	}
	if got := atomic.LoadInt32(&src.downloads); got != 1 {
		t.Fatalf("expected 1 online download, got %d", got)
	}

	// The file must now exist locally under the canonical (decoded) name.
	localPath := filepath.Join(s.packagesDir, wantFile)
	if _, err := os.Stat(localPath); err != nil {
		t.Fatalf("expected cached file %q: %v", localPath, err)
	}

	// Second request: served from local cache, no new online download.
	req2 := downloadReq(wantFile)
	rec2 := httptest.NewRecorder()
	s.handleDownload(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("second download status = %d, want 200", rec2.Code)
	}
	if got := atomic.LoadInt32(&src.downloads); got != 1 {
		t.Errorf("expected still 1 online download (served from cache), got %d", got)
	}
}

// TestOnlineFallback_DownloadDisabled verifies that with the flag off, a miss
// yields 404 instead of an online fetch.
func TestOnlineFallback_DownloadDisabled(t *testing.T) {
	src := &mockSyncSource{
		versions: []SyncVersionInfo{
			{Browser: "firefox", Version: "141.0", DownloadURL: "https://example.com/Firefox%20Setup%20141.0.exe"},
		},
	}
	s := newFallbackTestServer(t, false, src)

	req := downloadReq("Firefox Setup 141.0.exe")
	rec := httptest.NewRecorder()
	s.handleDownload(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 with fallback disabled, got %d", rec.Code)
	}
	if got := atomic.LoadInt32(&src.downloads); got != 0 {
		t.Errorf("expected 0 downloads with fallback disabled, got %d", got)
	}
}

// TestOnlineFallback_DownloadNotFound verifies that a request for a file the
// online source does not know about still returns 404 gracefully.
func TestOnlineFallback_DownloadNotFound(t *testing.T) {
	src := &mockSyncSource{
		versions: []SyncVersionInfo{
			{Browser: "firefox", Version: "141.0", DownloadURL: "https://example.com/Firefox%20Setup%20141.0.exe"},
		},
	}
	s := newFallbackTestServer(t, true, src)

	req := downloadReq("does-not-exist.exe")
	rec := httptest.NewRecorder()
	s.handleDownload(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown file, got %d", rec.Code)
	}
}

// TestOnlineFallback_ManifestLocalPriority verifies that a locally cached file
// (which carries a real checksum) is preferred over the online entry for the
// same filename, and appears only once.
func TestOnlineFallback_ManifestLocalPriority(t *testing.T) {
	src := &mockSyncSource{
		versions: []SyncVersionInfo{
			{
				Browser:     "chrome",
				Version:     "120.0.6099.109",
				DownloadURL: "https://example.com/120.0.6099.109_win64_installer.exe",
			},
		},
	}
	dir := t.TempDir()
	packagesDir := filepath.Join(dir, "packages")
	if err := os.MkdirAll(packagesDir, 0o755); err != nil {
		t.Fatalf("mkdir packages: %v", err)
	}
	// Seed a local package with the same filename as the online version.
	localFile := filepath.Join(packagesDir, "120.0.6099.109_win64_installer.exe")
	if err := os.WriteFile(localFile, []byte("local installer bytes"), 0o644); err != nil {
		t.Fatalf("write local file: %v", err)
	}
	s := NewServerWithOptions(ServerOptions{
		Addr:           ":0",
		Version:        "test",
		PackagesDir:    packagesDir,
		BinDir:         filepath.Join(dir, "bin"),
		OnlineSource:   src,
		OnlineFallback: true,
	})
	cache, _ := s.loadCache()
	if err := s.scanPackages(cache); err != nil {
		t.Fatalf("scanPackages: %v", err)
	}

	resp := doManifest(t, s)

	count := 0
	var entry PackageFile
	for _, f := range resp.Data {
		if f.Filename == "120.0.6099.109_win64_installer.exe" {
			count++
			entry = f
		}
	}
	if count != 1 {
		t.Fatalf("expected local+online dedup to 1 entry, got %d", count)
	}
	// Local entries carry a real xxh3 checksum; online entries have none.
	if entry.Checksum == "" {
		t.Errorf("expected local entry (with checksum) to win over online entry, but checksum is empty")
	}
}

// TestOnlineFallback_ConcurrentDownloadDedup verifies that two concurrent
// download requests for the same missing file trigger only a single online
// fetch.
func TestOnlineFallback_ConcurrentDownloadDedup(t *testing.T) {
	src := &mockSyncSource{
		versions: []SyncVersionInfo{
			{
				Browser:     "firefox",
				Version:     "141.0",
				DownloadURL: "https://example.com/Firefox%20Setup%20141.0.exe",
			},
		},
	}
	s := newFallbackTestServer(t, true, src)
	wantFile := "Firefox Setup 141.0.exe"

	doReq := func(code *int) {
		req := downloadReq(wantFile)
		rec := httptest.NewRecorder()
		s.handleDownload(rec, req)
		*code = rec.Code
	}

	var c1, c2 int
	done := make(chan struct{})
	go func() { doReq(&c1); close(done) }()
	doReq(&c2)
	<-done

	if c1 != http.StatusOK || c2 != http.StatusOK {
		t.Fatalf("expected both requests 200, got %d and %d", c1, c2)
	}
	if got := atomic.LoadInt32(&src.downloads); got != 1 {
		t.Errorf("expected a single online download for concurrent requests, got %d", got)
	}
}

// TestOnlineFilename_Decoding checks the URL-decoding filename helper.
func TestOnlineFilename_Decoding(t *testing.T) {
	cases := []struct {
		in   SyncVersionInfo
		want string
	}{
		{SyncVersionInfo{DownloadURL: "https://x/y/Firefox%20Setup%20141.0.exe"}, "Firefox Setup 141.0.exe"},
		{SyncVersionInfo{DownloadURL: "https://x/y/chrome-120.exe"}, "chrome-120.exe"},
		{SyncVersionInfo{Filename: "explicit.exe", DownloadURL: "https://x/y/ignored.exe"}, "explicit.exe"},
	}
	for _, c := range cases {
		if got := onlineFilename(c.in); got != c.want {
			t.Errorf("onlineFilename(%+v) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestMergeManifest_Dedup checks the manifest merge deduplication.
func TestMergeManifest_Dedup(t *testing.T) {
	local := []PackageFile{
		{Filename: "a.exe", Checksum: "xxh3:local"},
		{Filename: "b.exe", Checksum: "xxh3:local"},
	}
	online := []PackageFile{
		{Filename: "b.exe", Checksum: ""}, // dup of local
		{Filename: "c.exe", Checksum: ""},
	}
	merged := mergeManifest(local, online)
	if len(merged) != 3 {
		t.Fatalf("expected 3 merged entries, got %d", len(merged))
	}
	byName := map[string]PackageFile{}
	for _, f := range merged {
		byName[f.Filename] = f
	}
	if byName["b.exe"].Checksum != "xxh3:local" {
		t.Errorf("expected local b.exe to win, got checksum %q", byName["b.exe"].Checksum)
	}
}
