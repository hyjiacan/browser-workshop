package serve

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/bws/bws/internal/driver"
	bmlog "github.com/bws/bws/internal/log"
	"github.com/bws/bws/internal/paths"
)

// errDriverUnavailable stands in for an unreachable upstream manifest.
var errDriverUnavailable = errors.New("测试：上游不可用")

// fakeDriverSource resolves a fixed driver build (or fails) without network.
type fakeDriverSource struct {
	info *driver.Info
	err  error
}

func (f *fakeDriverSource) Name() string { return "fake" }

func (f *fakeDriverSource) Resolve(_ context.Context, _, _, _ string) (*driver.Info, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.info, nil
}

// newDriverTestServer builds a Server whose driver resolver uses an injected
// source, so tests never touch the real Chrome for Testing endpoints.
func newDriverTestServer(t *testing.T, src driver.Source) *Server {
	t.Helper()
	base := t.TempDir()
	packagesDir := filepath.Join(base, "packages")
	if err := os.MkdirAll(packagesDir, 0o755); err != nil {
		t.Fatalf("创建 packages 目录失败: %v", err)
	}

	s := NewServerWithOptions(ServerOptions{
		Addr:        ":0",
		Version:     "test",
		BaseDir:     base,
		PackagesDir: packagesDir,
		BinDir:      filepath.Join(base, "bin"),
		Logger:      bmlog.New(bmlog.LevelError, io.Discard),
	})

	cacheDir := filepath.Join(base, "cache", "serve")
	s.driverResolver = &DriverResolver{
		mgr:       driver.NewManager(paths.New(base), driver.Options{Sources: []driver.Source{src}}),
		dir:       filepath.Join(cacheDir, "drivers"),
		indexPath: filepath.Join(cacheDir, driverIndexFileName),
		logger:    s.logger,
		index:     make(map[string]driverIndexEntry),
		inflight:  make(map[string]chan struct{}),
	}
	return s
}

func doDriverManifestRequest(t *testing.T, s *Server, query string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, driverAPIBase+"/manifest"+query, nil)
	rec := httptest.NewRecorder()
	s.handleDriverManifest(rec, req)
	return rec
}

func decodeManifest(t *testing.T, rec *httptest.ResponseRecorder) driverManifestResponse {
	t.Helper()
	var resp driverManifestResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v (body=%s)", err, rec.Body.String())
	}
	return resp
}

func TestDriverManifest_Success(t *testing.T) {
	src := &fakeDriverSource{info: &driver.Info{
		Name:         driver.NameChromedriver,
		Version:      "120.0.6099.109",
		MajorVersion: "120",
		Platform:     "windows",
		Arch:         "amd64",
		DownloadURL:  "https://upstream.example/chromedriver-win64.zip",
		Filename:     "chromedriver-win64.zip",
		Source:       "chrome-for-testing",
	}}
	s := newDriverTestServer(t, src)

	rec := doDriverManifestRequest(t, s, "?chrome=120.0.6099.109&platform=windows&arch=amd64")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	resp := decodeManifest(t, rec)
	if resp.Data == nil {
		t.Fatal("data = nil")
	}
	want := driverAPIBase + "/download/chromedriver-win64.zip"
	if resp.Data.DownloadURL != want {
		t.Errorf("DownloadURL = %q, 期望 %q", resp.Data.DownloadURL, want)
	}
	if resp.Data.Source != "serve" {
		t.Errorf("Source = %q, 期望 serve", resp.Data.Source)
	}
	if _, ok := s.driverResolver.lookup("chromedriver-win64.zip"); !ok {
		t.Error("解析结果未记入驱动索引")
	}
}

func TestDriverManifest_MissingChromeParam(t *testing.T) {
	s := newDriverTestServer(t, &fakeDriverSource{})
	rec := doDriverManifestRequest(t, s, "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, 期望 400", rec.Code)
	}
}

func TestDriverManifest_Disabled(t *testing.T) {
	s := newDriverTestServer(t, &fakeDriverSource{})
	s.driverResolver = nil

	rec := doDriverManifestRequest(t, s, "?chrome=120")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, 期望 503", rec.Code)
	}
}

func TestDriverManifest_MethodNotAllowed(t *testing.T) {
	s := newDriverTestServer(t, &fakeDriverSource{})
	req := httptest.NewRequest(http.MethodPost, driverAPIBase+"/manifest", nil)
	rec := httptest.NewRecorder()
	s.handleDriverManifest(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, 期望 405", rec.Code)
	}
}

func TestDriverManifest_UnknownPath(t *testing.T) {
	s := newDriverTestServer(t, &fakeDriverSource{})
	req := httptest.NewRequest(http.MethodGet, driverAPIBase+"/other", nil)
	rec := httptest.NewRecorder()
	s.handleDriverManifest(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, 期望 404", rec.Code)
	}
}

func TestDriverManifest_OfflineFallbackToIndex(t *testing.T) {
	s := newDriverTestServer(t, &fakeDriverSource{err: errDriverUnavailable})
	s.driverResolver.index["chromedriver-win64.zip"] = driverIndexEntry{
		Filename:     "chromedriver-win64.zip",
		Version:      "120.0.6099.109",
		MajorVersion: "120",
		Platform:     "windows",
		Arch:         "amd64",
		DownloadURL:  "https://upstream.example/chromedriver-win64.zip",
	}

	rec := doDriverManifestRequest(t, s, "?chrome=120.0.6099.109&platform=windows&arch=amd64")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	resp := decodeManifest(t, rec)
	if resp.Data == nil {
		t.Fatal("data = nil")
	}
	// The fake source always fails, so a 200 here can only come from the local
	// index. The response still advertises the local download URL.
	if resp.Data.Version != "120.0.6099.109" {
		t.Errorf("Version = %q, 期望 120.0.6099.109", resp.Data.Version)
	}
	want := driverAPIBase + "/download/chromedriver-win64.zip"
	if resp.Data.DownloadURL != want {
		t.Errorf("DownloadURL = %q, 期望 %q", resp.Data.DownloadURL, want)
	}
}

func TestDriverManifest_NotFound(t *testing.T) {
	s := newDriverTestServer(t, &fakeDriverSource{err: errDriverUnavailable})
	rec := doDriverManifestRequest(t, s, "?chrome=120&platform=windows&arch=amd64")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, 期望 404", rec.Code)
	}
	resp := decodeManifest(t, rec)
	if resp.Status != "error" || resp.Error == "" {
		t.Errorf("响应 = %+v, 期望包含错误信息", resp)
	}
}

func doDriverDownloadRequest(t *testing.T, s *Server, filename string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, driverAPIBase+"/download/"+url.PathEscape(filename), nil)
	rec := httptest.NewRecorder()
	s.handleDriverDownload(rec, req)
	return rec
}

func TestDriverDownload_LocalHit(t *testing.T) {
	s := newDriverTestServer(t, &fakeDriverSource{})
	if err := os.MkdirAll(s.driverResolver.dir, 0o755); err != nil {
		t.Fatalf("创建驱动目录失败: %v", err)
	}
	content := []byte("hosted-driver-archive")
	if err := os.WriteFile(filepath.Join(s.driverResolver.dir, "chromedriver-win64.zip"), content, 0o644); err != nil {
		t.Fatalf("写入驱动归档失败: %v", err)
	}

	rec := doDriverDownloadRequest(t, s, "chromedriver-win64.zip")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !bytes.Equal(rec.Body.Bytes(), content) {
		t.Errorf("body = %q, 期望 %q", rec.Body.Bytes(), content)
	}
}

func TestDriverDownload_ProxiesAndCaches(t *testing.T) {
	content := []byte("upstream-driver-archive")
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(content)
	}))
	defer upstream.Close()

	s := newDriverTestServer(t, &fakeDriverSource{})
	s.driverResolver.index["chromedriver-win64.zip"] = driverIndexEntry{
		Filename:    "chromedriver-win64.zip",
		DownloadURL: upstream.URL + "/chromedriver-win64.zip",
	}

	rec := doDriverDownloadRequest(t, s, "chromedriver-win64.zip")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !bytes.Equal(rec.Body.Bytes(), content) {
		t.Errorf("body = %q, 期望 %q", rec.Body.Bytes(), content)
	}
	if _, err := os.Stat(filepath.Join(s.driverResolver.dir, "chromedriver-win64.zip")); err != nil {
		t.Errorf("归档未被缓存: %v", err)
	}
}

func TestDriverDownload_NotFound(t *testing.T) {
	s := newDriverTestServer(t, &fakeDriverSource{})
	rec := doDriverDownloadRequest(t, s, "missing.zip")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, 期望 404", rec.Code)
	}
}

func TestDriverDownload_EmptyFilename(t *testing.T) {
	s := newDriverTestServer(t, &fakeDriverSource{})
	req := httptest.NewRequest(http.MethodGet, driverAPIBase+"/download/", nil)
	rec := httptest.NewRecorder()
	s.handleDriverDownload(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, 期望 400", rec.Code)
	}
}

func TestDriverDownload_InvalidFilename(t *testing.T) {
	s := newDriverTestServer(t, &fakeDriverSource{})
	req := httptest.NewRequest(http.MethodGet, driverAPIBase+"/download/x.zip", nil)
	// Bypass httptest's path cleaning to exercise the traversal guard.
	req.URL.Path = driverAPIBase + "/download/../secret.zip"
	rec := httptest.NewRecorder()
	s.handleDriverDownload(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, 期望 400", rec.Code)
	}
}

func TestDriverDownload_Disabled(t *testing.T) {
	s := newDriverTestServer(t, &fakeDriverSource{})
	s.driverResolver = nil
	rec := doDriverDownloadRequest(t, s, "x.zip")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, 期望 503", rec.Code)
	}
}

func TestDriverDownload_MethodNotAllowed(t *testing.T) {
	s := newDriverTestServer(t, &fakeDriverSource{})
	req := httptest.NewRequest(http.MethodPost, driverAPIBase+"/download/x.zip", nil)
	rec := httptest.NewRecorder()
	s.handleDriverDownload(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, 期望 405", rec.Code)
	}
}

func TestDriverIndexPersistence(t *testing.T) {
	base := t.TempDir()
	cacheDir := filepath.Join(base, "cache", "serve")
	logger := bmlog.New(bmlog.LevelError, io.Discard)

	r := newDriverResolver(cacheDir, logger)
	r.remember(&driver.Info{
		Name:         driver.NameChromedriver,
		Version:      "120.0.6099.109",
		MajorVersion: "120",
		Platform:     "windows",
		Arch:         "amd64",
		DownloadURL:  "https://upstream.example/chromedriver-win64.zip",
		Filename:     "chromedriver-win64.zip",
	})

	reloaded := newDriverResolver(cacheDir, logger)
	entry, ok := reloaded.lookup("chromedriver-win64.zip")
	if !ok {
		t.Fatal("驱动索引未持久化")
	}
	if entry.MajorVersion != "120" {
		t.Errorf("MajorVersion = %q, 期望 120", entry.MajorVersion)
	}
	if _, ok := reloaded.findLocal("120.0.6099.109", "windows", "amd64"); !ok {
		t.Error("重新加载后 findLocal 未命中")
	}
	if _, ok := reloaded.findLocal("121.0.0.0", "windows", "amd64"); ok {
		t.Error("findLocal 对不匹配主版本不应命中")
	}
}
