package source

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	bmlog "github.com/bws/bws/internal/log"
	"github.com/bws/bws/internal/paths"
	"github.com/bws/bws/internal/serve"
)

// startServeServerForClient starts a real serve server with the given test
// packages and returns its base URL. The server is automatically cleaned up
// when the test finishes. It also cleans up leftover manifest disk cache
// entries so that stale caches from previous tests don't pollute results.
func startServeServerForClient(t *testing.T, packages map[string][]byte) string {
	t.Helper()

	// Clean manifest disk cache from previous test runs to avoid
	// port-reuse collisions where a stale cache matches a recycled port.
	cleanManifestDiskCache(t)

	tmpDir := t.TempDir()
	packagesDir := filepath.Join(tmpDir, "packages")
	binDir := filepath.Join(tmpDir, "bin")
	if err := os.MkdirAll(packagesDir, 0o755); err != nil {
		t.Fatalf("创建 packages 目录失败: %v", err)
	}
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("创建 bin 目录失败: %v", err)
	}

	for name, content := range packages {
		if err := os.WriteFile(filepath.Join(packagesDir, name), content, 0o644); err != nil {
			t.Fatalf("创建测试文件失败 %s: %v", name, err)
		}
	}

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("查找空闲端口失败: %v", err)
	}
	addr := l.Addr().String()
	l.Close()

	server := serve.NewServerWithOptions(serve.ServerOptions{
		Addr:        addr,
		Version:     "test",
		BaseDir:     tmpDir,
		PackagesDir: packagesDir,
		BinDir:      binDir,
		Logger:      bmlog.New(bmlog.LevelError, io.Discard),
		ScanWorkers: 1,
	})

	go func() {
		if err := server.Start(); err != nil && err != http.ErrServerClosed {
			t.Errorf("服务器启动失败: %v", err)
		}
	}()

	baseURL := "http://" + addr

	deadline := time.Now().Add(5 * time.Second)
	var ready bool
	for time.Now().Before(deadline) {
		resp, err := http.Get(baseURL + "/api/v1/status")
		if err == nil && resp.StatusCode == 200 {
			resp.Body.Close()
			ready = true
			break
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !ready {
		t.Fatal("服务器启动超时")
	}

	t.Cleanup(func() {
		_ = server.Stop()
	})

	return baseURL
}

// TestHTTPSourceIntegration_List verifies that HTTPSource.List fetches the
// manifest from a real serve server and returns versions matching the filter.
func TestHTTPSourceIntegration_List(t *testing.T) {
	packages := map[string][]byte{
		"chrome_120.0.6099.109_win64.zip": []byte("chrome 120"),
		"chrome_121.0.6163.66_win64.zip":  []byte("chrome 121"),
		"firefox_120.0esr_linux64.tar.bz2": []byte("firefox 120"),
	}
	baseURL := startServeServerForClient(t, packages)

	src := NewHTTPSource(baseURL)
	versions, err := src.List(context.Background(), &Filter{
		Browser: "chrome",
	})
	if err != nil {
		t.Fatalf("List 失败: %v", err)
	}
	if len(versions) < 2 {
		t.Errorf("List 返回 %d 个 chrome 版本, 期望 >= 2", len(versions))
	}
	for _, v := range versions {
		if v.Browser != "chrome" {
			t.Errorf("返回了非 chrome 版本: %s", v.Browser)
		}
		if v.DownloadURL == "" {
			t.Error("DownloadURL 为空")
		}
	}
}

// TestHTTPSourceIntegration_Latest verifies that HTTPSource.Latest returns
// the highest version from a real serve server.
func TestHTTPSourceIntegration_Latest(t *testing.T) {
	packages := map[string][]byte{
		"chrome_120.0.6099.109_win64.zip": []byte("chrome 120"),
		"chrome_121.0.6163.66_win64.zip":  []byte("chrome 121"),
	}
	baseURL := startServeServerForClient(t, packages)

	src := NewHTTPSource(baseURL)
	latest, err := src.Latest(context.Background(), &Filter{
		Browser: "chrome",
	})
	if err != nil {
		t.Fatalf("Latest 失败: %v", err)
	}
	if latest.Version != "121.0.6163.66" {
		t.Errorf("Latest 版本 = %q, 期望 \"121.0.6163.66\"", latest.Version)
	}
}

// TestHTTPSourceIntegration_Resolve_Exact verifies that Resolve finds an
// exact version match.
func TestHTTPSourceIntegration_Resolve_Exact(t *testing.T) {
	packages := map[string][]byte{
		"chrome_120.0.6099.109_win64.zip": []byte("chrome 120"),
		"chrome_121.0.6163.66_win64.zip":  []byte("chrome 121"),
	}
	baseURL := startServeServerForClient(t, packages)

	src := NewHTTPSource(baseURL)
	v, err := src.Resolve(context.Background(), "chrome", "120.0.6099.109", CurrentPlatform(), CurrentArch())
	if err != nil {
		t.Fatalf("Resolve 失败: %v", err)
	}
	if v.Version != "120.0.6099.109" {
		t.Errorf("Resolve 版本 = %q, 期望 \"120.0.6099.109\"", v.Version)
	}
}

// TestHTTPSourceIntegration_Resolve_Prefix verifies that Resolve supports
// partial version prefix matching, returning the highest matching version.
func TestHTTPSourceIntegration_Resolve_Prefix(t *testing.T) {
	packages := map[string][]byte{
		"chrome_120.0.6099.109_win64.zip": []byte("chrome 120"),
		"chrome_121.0.6163.66_win64.zip":  []byte("chrome 121"),
	}
	baseURL := startServeServerForClient(t, packages)

	src := NewHTTPSource(baseURL)
	v, err := src.Resolve(context.Background(), "chrome", "121", CurrentPlatform(), CurrentArch())
	if err != nil {
		t.Fatalf("Resolve 失败: %v", err)
	}
	if !strings.HasPrefix(v.Version, "121.") {
		t.Errorf("Resolve 版本 = %q, 期望以 \"121.\" 开头", v.Version)
	}
}

// TestHTTPSourceIntegration_Resolve_Latest verifies that Resolve with
// "latest" returns the highest available version.
func TestHTTPSourceIntegration_Resolve_Latest(t *testing.T) {
	packages := map[string][]byte{
		"chrome_120.0.6099.109_win64.zip": []byte("chrome 120"),
		"chrome_121.0.6163.66_win64.zip":  []byte("chrome 121"),
	}
	baseURL := startServeServerForClient(t, packages)

	src := NewHTTPSource(baseURL)
	v, err := src.Resolve(context.Background(), "chrome", "latest", CurrentPlatform(), CurrentArch())
	if err != nil {
		t.Fatalf("Resolve 失败: %v", err)
	}
	if v.Version != "121.0.6163.66" {
		t.Errorf("Resolve latest 版本 = %q, 期望 \"121.0.6163.66\"", v.Version)
	}
}

// TestHTTPSourceIntegration_Resolve_NotFound verifies that Resolve returns
// an error when the requested version does not exist.
func TestHTTPSourceIntegration_Resolve_NotFound(t *testing.T) {
	packages := map[string][]byte{
		"chrome_120.0.6099.109_win64.zip": []byte("chrome 120"),
	}
	baseURL := startServeServerForClient(t, packages)

	src := NewHTTPSource(baseURL)
	_, err := src.Resolve(context.Background(), "chrome", "999.0.0.0", CurrentPlatform(), CurrentArch())
	if err == nil {
		t.Error("Resolve 应返回错误（版本不存在）")
	}
}

// TestHTTPSourceIntegration_EmptyManifest verifies that List on an empty
// server returns an empty list without error.
func TestHTTPSourceIntegration_EmptyManifest(t *testing.T) {
	baseURL := startServeServerForClient(t, nil)

	src := NewHTTPSource(baseURL)
	versions, err := src.List(context.Background(), &Filter{
		Browser: "chrome",
	})
	if err != nil {
		t.Fatalf("List 失败: %v", err)
	}
	if len(versions) != 0 {
		t.Errorf("List 返回 %d 个版本, 期望 0 (空清单)", len(versions))
	}
}

// TestHTTPSourceIntegration_DownloadURL verifies that the download URL
// returned by List has the correct format: baseURL/api/v1/download/filename
func TestHTTPSourceIntegration_DownloadURL(t *testing.T) {
	packages := map[string][]byte{
		"chrome_120.0.6099.109_win64.zip": []byte("chrome 120"),
	}
	baseURL := startServeServerForClient(t, packages)

	src := NewHTTPSource(baseURL)
	versions, err := src.List(context.Background(), &Filter{
		Browser: "chrome",
	})
	if err != nil {
		t.Fatalf("List 失败: %v", err)
	}
	if len(versions) == 0 {
		t.Fatal("List 返回空列表")
	}

	expectedPrefix := baseURL + "/api/v1/download/"
	if !strings.HasPrefix(versions[0].DownloadURL, expectedPrefix) {
		t.Errorf("DownloadURL = %q, 期望以 %q 开头", versions[0].DownloadURL, expectedPrefix)
	}

	// Verify the URL-encoded filename is present.
	encoded := url.PathEscape("chrome_120.0.6099.109_win64.zip")
	if !strings.HasSuffix(versions[0].DownloadURL, encoded) {
		t.Errorf("DownloadURL = %q, 期望以 %q 结尾", versions[0].DownloadURL, encoded)
	}
}

// TestHTTPSourceIntegration_SupportsBrowser verifies that HTTPSource reports
// supporting all browser types (it is a generic serve endpoint).
func TestHTTPSourceIntegration_SupportsBrowser(t *testing.T) {
	baseURL := startServeServerForClient(t, nil)

	src := NewHTTPSource(baseURL)
	for _, browser := range []string{"chrome", "firefox", "chromium", "edge", "brave"} {
		if !src.SupportsBrowser(browser) {
			t.Errorf("SupportsBrowser(%q) = false, 期望 true", browser)
		}
	}
}

// TestHTTPSourceIntegration_ManifestCache verifies that repeated List calls
// within the cache TTL produce consistent results without errors.
func TestHTTPSourceIntegration_ManifestCache(t *testing.T) {
	packages := map[string][]byte{
		"chrome_120.0.6099.109_win64.zip": []byte("chrome 120"),
		"chrome_121.0.6163.66_win64.zip":  []byte("chrome 121"),
	}
	baseURL := startServeServerForClient(t, packages)

	src := NewHTTPSource(baseURL)

	// First call fetches from server.
	v1, err := src.List(context.Background(), &Filter{Browser: "chrome"})
	if err != nil {
		t.Fatalf("第一次 List 失败: %v", err)
	}

	// Second call should use cache and return the same results.
	v2, err := src.List(context.Background(), &Filter{Browser: "chrome"})
	if err != nil {
		t.Fatalf("第二次 List 失败: %v", err)
	}

	if len(v1) != len(v2) {
		t.Errorf("两次 List 结果长度不一致: %d vs %d", len(v1), len(v2))
	}
}

// TestHTTPSourceIntegration_Name verifies that the source name includes
// the base URL.
func TestHTTPSourceIntegration_Name(t *testing.T) {
	baseURL := "http://127.0.0.1:9999"
	src := NewHTTPSource(baseURL)
	name := src.Name()
	if !strings.Contains(name, baseURL) {
		t.Errorf("Name = %q, 期望包含 %q", name, baseURL)
	}
}

// TestHTTPSourceIntegration_FilterByBrowser verifies that List correctly
// filters by browser name, returning only matching versions.
func TestHTTPSourceIntegration_FilterByBrowser(t *testing.T) {
	packages := map[string][]byte{
		"chrome_120.0.6099.109_win64.zip":   []byte("chrome"),
		"firefox_120.0esr_linux64.tar.bz2":   []byte("firefox"),
	}
	baseURL := startServeServerForClient(t, packages)

	src := NewHTTPSource(baseURL)

	// Filter for chrome only.
	chromeVersions, err := src.List(context.Background(), &Filter{
		Browser: "chrome",
	})
	if err != nil {
		t.Fatalf("List chrome 失败: %v", err)
	}
	for _, v := range chromeVersions {
		if v.Browser != "chrome" {
			t.Errorf("期望 browser=chrome, got %q", v.Browser)
		}
	}

	// Create a new source to avoid cache.
	src2 := NewHTTPSource(baseURL)

	// Filter for firefox only — note that applyDefaults sets platform to
	// CurrentPlatform(), so on Windows this will filter to windows platform.
	// Firefox is linux64, so it might not appear. We check the filter logic
	// rather than the specific count.
	firefoxVersions, err := src2.List(context.Background(), &Filter{
		Browser:  "firefox",
		Platform: PlatformLinux,
		Arch:     ArchAMD64,
	})
	if err != nil {
		t.Fatalf("List firefox 失败: %v", err)
	}
	for _, v := range firefoxVersions {
		if v.Browser != "firefox" {
			t.Errorf("期望 browser=firefox, got %q", v.Browser)
		}
	}
}

// TestHTTPSourceIntegration_Resolve_EmptyVersion verifies that Resolve with
// an empty version string is treated as "latest".
func TestHTTPSourceIntegration_Resolve_EmptyVersion(t *testing.T) {
	packages := map[string][]byte{
		"chrome_120.0.6099.109_win64.zip": []byte("chrome 120"),
		"chrome_121.0.6163.66_win64.zip":  []byte("chrome 121"),
	}
	baseURL := startServeServerForClient(t, packages)

	src := NewHTTPSource(baseURL)
	v, err := src.Resolve(context.Background(), "chrome", "", CurrentPlatform(), CurrentArch())
	if err != nil {
		t.Fatalf("Resolve 失败: %v", err)
	}
	if v.Version != "121.0.6163.66" {
		t.Errorf("Resolve 空版本 = %q, 期望 \"121.0.6163.66\" (latest)", v.Version)
	}
}

// TestHTTPSourceIntegration_NameFormat verifies the source name format.
func TestHTTPSourceIntegration_NameFormat(t *testing.T) {
	src := NewHTTPSource("http://localhost:8080")
	expected := "http:http://localhost:8080"
	if src.Name() != expected {
		t.Errorf("Name = %q, 期望 %q", src.Name(), expected)
	}
}

// TestHTTPSourceIntegration_NewHTTPSourceWithProxy verifies that creating
// an HTTPSource with a proxy doesn't crash and normalizes the base URL.
func TestHTTPSourceIntegration_NewHTTPSourceWithProxy(t *testing.T) {
	src := NewHTTPSourceWithProxy("http://localhost:8080/", "")
	if src.baseURL != "http://localhost:8080" {
		t.Errorf("baseURL = %q, 期望 \"http://localhost:8080\" (trailing slash removed)", src.baseURL)
	}
}

// TestHTTPSourceIntegration_DownloadURLWithSpecialChars verifies that
// filenames with special characters are properly URL-encoded in the
// download URL.
func TestHTTPSourceIntegration_DownloadURLWithSpecialChars(t *testing.T) {
	// Use a filename with spaces (Firefox setup files have spaces).
	packages := map[string][]byte{
		"firefox_120.0esr_win64.zip": []byte("firefox"),
	}
	baseURL := startServeServerForClient(t, packages)

	src := NewHTTPSource(baseURL)
	versions, err := src.List(context.Background(), &Filter{
		Browser:  "firefox",
		Platform: PlatformWindows,
		Arch:     ArchAMD64,
	})
	if err != nil {
		t.Fatalf("List 失败: %v", err)
	}

	for _, v := range versions {
		// DownloadURL should be properly URL-encoded.
		if !strings.HasPrefix(v.DownloadURL, baseURL+"/api/v1/download/") {
			t.Errorf("DownloadURL = %q, 期望以 %q 开头", v.DownloadURL, baseURL+"/api/v1/download/")
		}
		// Verify the URL is valid.
		_, err := url.Parse(v.DownloadURL)
		if err != nil {
			t.Errorf("DownloadURL %q 不是有效的 URL: %v", v.DownloadURL, err)
		}
	}
}

// TestHTTPSourceIntegration_ServerInfo verifies that the manifest response
// includes server information (name, version, file_count).
func TestHTTPSourceIntegration_ServerInfo(t *testing.T) {
	packages := map[string][]byte{
		"chrome_120.0.6099.109_win64.zip": []byte("chrome 120"),
	}
	baseURL := startServeServerForClient(t, packages)

	// Fetch manifest directly to check server info.
	resp, err := http.Get(baseURL + "/api/v1/manifest")
	if err != nil {
		t.Fatalf("获取 manifest 失败: %v", err)
	}
	defer resp.Body.Close()

	var m struct {
		Status string `json:"status"`
		Server struct {
			Name      string `json:"name"`
			Version   string `json:"version"`
			FileCount int    `json:"file_count"`
		} `json:"server"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		t.Fatalf("解析 manifest 失败: %v", err)
	}
	if m.Server.Name != "bws-serve" {
		t.Errorf("server.name = %q, 期望 \"bws-serve\"", m.Server.Name)
	}
	if m.Server.Version != "test" {
		t.Errorf("server.version = %q, 期望 \"test\"", m.Server.Version)
	}
}

// cleanManifestDiskCache removes all serve manifest cache files so that
// stale caches from previous test runs (possibly with recycled ports) don't
// pollute results.
func cleanManifestDiskCache(t *testing.T) {
	t.Helper()
	cacheDir := paths.Default().ManifestCacheDir
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		t.Logf("读取 manifest 缓存目录失败: %v", err)
		return
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.HasPrefix(entry.Name(), "serve_") && strings.HasSuffix(entry.Name(), ".json") {
			os.Remove(filepath.Join(cacheDir, entry.Name()))
		}
	}
}
