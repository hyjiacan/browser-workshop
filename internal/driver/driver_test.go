package driver

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bws/bws/internal/paths"
)

// errNoDriver stands in for a source that cannot resolve a build.
var errNoDriver = errors.New("测试：未找到驱动")

func TestCFTPlatformMapping(t *testing.T) {
	cases := []struct {
		platform string
		arch     string
		want     string
	}{
		{"windows", "amd64", "win64"},
		{"windows", "386", "win32"},
		{"windows", "arm64", ""},
		{"darwin", "amd64", "mac-x64"},
		{"darwin", "arm64", "mac-arm64"},
		{"linux", "amd64", "linux64"},
		{"linux", "arm64", ""},
		{"android", "arm64", ""},
	}
	for _, c := range cases {
		if got := CFTPlatform(c.platform, c.arch); got != c.want {
			t.Errorf("CFTPlatform(%q, %q) = %q, want %q", c.platform, c.arch, got, c.want)
		}
	}
}

func TestLegacyPlatformMapping(t *testing.T) {
	cases := []struct {
		platform string
		arch     string
		want     string
	}{
		{"windows", "amd64", "win32"},
		{"windows", "386", "win32"},
		{"darwin", "amd64", "mac64"},
		{"darwin", "arm64", "mac64_m1"},
		{"linux", "amd64", "linux64"},
		{"linux", "386", ""},
	}
	for _, c := range cases {
		if got := LegacyPlatform(c.platform, c.arch); got != c.want {
			t.Errorf("LegacyPlatform(%q, %q) = %q, want %q", c.platform, c.arch, got, c.want)
		}
	}
}

func TestMajorKey(t *testing.T) {
	cases := map[string]string{
		"120.0.6099.109": "120",
		"120":            "120",
		"":               "",
	}
	for in, want := range cases {
		if got := majorKey(in); got != want {
			t.Errorf("majorKey(%q) = %q, want %q", in, got, want)
		}
	}
}

// fakeSource resolves a fixed driver build, standing in for the network.
type fakeSource struct {
	info *Info
	err  error
}

func (f *fakeSource) Name() string { return "fake" }

func (f *fakeSource) Resolve(_ context.Context, _, _, _ string) (*Info, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.info, nil
}

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	return NewManager(paths.New(t.TempDir()), Options{})
}

func writeCFTManifest(t *testing.T, versions ...map[string]any) []byte {
	t.Helper()
	body, err := json.Marshal(map[string]any{
		"timestamp": "2026-01-01T00:00:00Z",
		"versions":  versions,
	})
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	return body
}

func cftEntry(version string, url string) map[string]any {
	return map[string]any{
		"version":  version,
		"revision": "1",
		"downloads": map[string]any{
			"chromedriver": []map[string]string{
				{"platform": "win64", "url": url},
			},
		},
	}
}

func TestChromeForTestingResolveExactAndFallback(t *testing.T) {
	var baseURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(writeCFTManifest(t,
			cftEntry("120.0.6099.109", baseURL+"/120/chromedriver-win64.zip"),
			cftEntry("120.0.6099.200", baseURL+"/120b/chromedriver-win64.zip"),
			cftEntry("121.0.6167.85", baseURL+"/121/chromedriver-win64.zip"),
		))
	}))
	defer srv.Close()
	baseURL = srv.URL

	p := paths.New(t.TempDir())
	src := NewChromeForTestingSource(p, srv.Client())
	src.manifestURL = srv.URL + "/known-good.json"
	ctx := context.Background()

	t.Run("exact match", func(t *testing.T) {
		info, err := src.Resolve(ctx, "120.0.6099.109", "windows", "amd64")
		if err != nil {
			t.Fatalf("Resolve: %v", err)
		}
		if info.Version != "120.0.6099.109" {
			t.Errorf("Version = %q, want 120.0.6099.109", info.Version)
		}
		if info.MajorVersion != "120" {
			t.Errorf("MajorVersion = %q, want 120", info.MajorVersion)
		}
		if info.Filename != "chromedriver-win64.zip" {
			t.Errorf("Filename = %q", info.Filename)
		}
		if info.DownloadURL != srv.URL+"/120/chromedriver-win64.zip" {
			t.Errorf("DownloadURL = %q", info.DownloadURL)
		}
	})

	t.Run("same-major fallback picks newest", func(t *testing.T) {
		info, err := src.Resolve(ctx, "120.0.6099.999", "windows", "amd64")
		if err != nil {
			t.Fatalf("Resolve: %v", err)
		}
		if info.Version != "120.0.6099.200" {
			t.Errorf("Version = %q, want 120.0.6099.200", info.Version)
		}
	})

	t.Run("major-only spec", func(t *testing.T) {
		info, err := src.Resolve(ctx, "121", "windows", "amd64")
		if err != nil {
			t.Fatalf("Resolve: %v", err)
		}
		if info.Version != "121.0.6167.85" {
			t.Errorf("Version = %q, want 121.0.6167.85", info.Version)
		}
	})

	t.Run("unsupported platform", func(t *testing.T) {
		if _, err := src.Resolve(ctx, "120.0.6099.109", "linux", "arm64"); err == nil {
			t.Error("expected error for unsupported platform")
		}
	})
}

func TestChromeForTestingResolveLegacy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/LATEST_RELEASE_114" {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte("114.0.5735.90\n"))
	}))
	defer srv.Close()

	p := paths.New(t.TempDir())
	src := NewChromeForTestingSource(p, srv.Client())
	src.legacyBase = srv.URL

	info, err := src.Resolve(context.Background(), "114.0.5735.90", "windows", "amd64")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if info.Version != "114.0.5735.90" {
		t.Errorf("Version = %q", info.Version)
	}
	if info.MajorVersion != "114" {
		t.Errorf("MajorVersion = %q", info.MajorVersion)
	}
	want := srv.URL + "/114.0.5735.90/chromedriver_win32.zip"
	if info.DownloadURL != want {
		t.Errorf("DownloadURL = %q, want %q", info.DownloadURL, want)
	}
	if info.Source != "chromedriver-storage" {
		t.Errorf("Source = %q", info.Source)
	}
}

func TestServeSourceResolvesRelativeDownloadURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/driver/manifest" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok","data":{
			"name":"chromedriver",
			"version":"120.0.6099.109",
			"major_version":"120",
			"platform":"windows",
			"arch":"amd64",
			"download_url":"/api/v1/driver/download/chromedriver-win64.zip",
			"filename":"chromedriver-win64.zip",
			"source":"serve"
		}}`))
	}))
	defer srv.Close()

	src := NewServeSource(srv.URL, srv.Client())
	info, err := src.Resolve(context.Background(), "120.0.6099.109", "windows", "amd64")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	want := srv.URL + "/api/v1/driver/download/chromedriver-win64.zip"
	if info.DownloadURL != want {
		t.Errorf("DownloadURL = %q, want %q", info.DownloadURL, want)
	}
	if info.Source != "serve" {
		t.Errorf("Source = %q, want serve", info.Source)
	}
}

func TestServeSourceErrorsOnNonOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"status":"error","error":"未找到"}`))
	}))
	defer srv.Close()

	src := NewServeSource(srv.URL, srv.Client())
	if _, err := src.Resolve(context.Background(), "120", "windows", "amd64"); err == nil {
		t.Error("expected error when serve returns non-OK")
	}
}

func makeDriverZip(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("chromedriver-win64/chromedriver.exe")
	if err != nil {
		t.Fatalf("zip create: %v", err)
	}
	if _, err := w.Write([]byte("fake-chromedriver-binary")); err != nil {
		t.Fatalf("zip write: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	return buf.Bytes()
}

func TestEnsureInstallListUninstall(t *testing.T) {
	archive := makeDriverZip(t)
	var downloads int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		downloads++
		w.Write(archive)
	}))
	defer srv.Close()

	mgr := newTestManager(t)
	mgr.sources = []Source{&fakeSource{info: &Info{
		Name:         NameChromedriver,
		Version:      "120.0.6099.109",
		MajorVersion: "120",
		Platform:     "windows",
		Arch:         "amd64",
		DownloadURL:  srv.URL + "/chromedriver-win64.zip",
		Filename:     "chromedriver-win64.zip",
		Source:       "test",
	}}}

	ctx := context.Background()
	rec, err := mgr.Ensure(ctx, EnsureOptions{ChromeVersion: "120.0.6099.109"})
	if err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	if rec.Version != "120.0.6099.109" {
		t.Errorf("Version = %q", rec.Version)
	}
	if !fileExists(rec.ExecutablePath()) {
		t.Fatalf("driver binary missing at %s", rec.ExecutablePath())
	}
	if downloads != 1 {
		t.Fatalf("downloads = %d, want 1", downloads)
	}

	// A second Ensure with a matching record must not hit the network.
	rec2, err := mgr.Ensure(ctx, EnsureOptions{ChromeVersion: "120.0.6099.109"})
	if err != nil {
		t.Fatalf("Ensure (cached): %v", err)
	}
	if rec2.Version != rec.Version {
		t.Errorf("cached version = %q, want %q", rec2.Version, rec.Version)
	}
	if downloads != 1 {
		t.Errorf("downloads = %d, want 1 (should reuse installed driver)", downloads)
	}

	list, err := mgr.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("List len = %d, want 1", len(list))
	}

	if _, err := mgr.BinaryPath("120"); err != nil {
		t.Errorf("BinaryPath: %v", err)
	}
	if !mgr.IsInstalled("120", "120.0.6099.109") {
		t.Error("IsInstalled = false, want true")
	}

	if err := mgr.Uninstall("120"); err != nil {
		t.Fatalf("Uninstall: %v", err)
	}
	list, err = mgr.List()
	if err != nil {
		t.Fatalf("List after uninstall: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("List len after uninstall = %d, want 0", len(list))
	}
}

func TestEnsureFallsBackAcrossSources(t *testing.T) {
	mgr := newTestManager(t)
	mgr.sources = []Source{
		&fakeSource{err: errNoDriver},
		&fakeSource{info: &Info{
			Name:         NameChromedriver,
			Version:      "120.0.6099.109",
			MajorVersion: "120",
			Platform:     "windows",
			Arch:         "amd64",
			DownloadURL:  "http://127.0.0.1:1/unused.zip",
			Filename:     "chromedriver-win64.zip",
			Source:       "test",
		}},
	}

	info, err := mgr.Resolve(context.Background(), "120.0.6099.109", "windows", "amd64")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if info.Version != "120.0.6099.109" {
		t.Errorf("Version = %q", info.Version)
	}
}

func TestResolveSourcesAllFail(t *testing.T) {
	mgr := newTestManager(t)
	mgr.sources = []Source{&fakeSource{err: errNoDriver}}

	if _, err := mgr.Resolve(context.Background(), "120", "windows", "amd64"); err == nil {
		t.Error("expected error when every source fails")
	}
}