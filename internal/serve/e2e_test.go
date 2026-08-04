package serve

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	bmlog "github.com/bws/bws/internal/log"
)

// --- Test helpers ---

// testServer wraps a running Server instance for e2e testing.
type testServer struct {
	server      *Server
	baseURL     string
	packagesDir string
	binDir      string
	baseDir     string
	t           *testing.T
}

// startTestServer creates a temp directory, starts a real Server on a random
// port, and waits until it is ready to accept connections.
func startTestServer(t *testing.T, opts ServerOptions) *testServer {
	t.Helper()

	tmpDir := t.TempDir()
	packagesDir := filepath.Join(tmpDir, "packages")
	binDir := filepath.Join(tmpDir, "bin")
	if err := os.MkdirAll(packagesDir, 0o755); err != nil {
		t.Fatalf("创建 packages 目录失败: %v", err)
	}
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("创建 bin 目录失败: %v", err)
	}

	opts.PackagesDir = packagesDir
	opts.BinDir = binDir
	opts.BaseDir = tmpDir
	if opts.Version == "" {
		opts.Version = "test"
	}
	if opts.Logger == nil {
		opts.Logger = bmlog.New(bmlog.LevelError, io.Discard)
	}
	if opts.ScanWorkers == 0 {
		opts.ScanWorkers = 1
	}

	// Find a free port.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("查找空闲端口失败: %v", err)
	}
	addr := l.Addr().String()
	l.Close()

	opts.Addr = addr
	server := NewServerWithOptions(opts)

	// Start the server in a goroutine.
	go func() {
		if err := server.Start(); err != nil && err != http.ErrServerClosed {
			t.Errorf("服务器启动失败: %v", err)
		}
	}()

	baseURL := "http://" + addr

	// Wait for the server to be ready (poll root "/" which is always open,
	// even when auth is enabled).
	deadline := time.Now().Add(5 * time.Second)
	var ready bool
	for time.Now().Before(deadline) {
		resp, err := http.Get(baseURL + "/")
		if err == nil && (resp.StatusCode == 200 || resp.StatusCode == 401) {
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

	ts := &testServer{
		server:      server,
		baseURL:     baseURL,
		packagesDir: packagesDir,
		binDir:      binDir,
		baseDir:     tmpDir,
		t:           t,
	}

	t.Cleanup(func() {
		_ = server.Stop()
	})

	return ts
}

// get sends a GET request and returns the response and body.
func (ts *testServer) get(path string) (*http.Response, []byte) {
	ts.t.Helper()
	resp, err := http.Get(ts.baseURL + path)
	if err != nil {
		ts.t.Fatalf("HTTP GET %s 失败: %v", path, err)
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		ts.t.Fatalf("读取响应体失败: %v", err)
	}
	return resp, body
}

// getWithAuth sends a GET request with an optional Bearer token.
func (ts *testServer) getWithAuth(path, token string) (*http.Response, []byte) {
	ts.t.Helper()
	req, err := http.NewRequest("GET", ts.baseURL+path, nil)
	if err != nil {
		ts.t.Fatalf("创建请求失败: %v", err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		ts.t.Fatalf("HTTP GET %s 失败: %v", path, err)
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		ts.t.Fatalf("读取响应体失败: %v", err)
	}
	return resp, body
}

// post sends a POST request and returns the response and body.
func (ts *testServer) post(path string) (*http.Response, []byte) {
	ts.t.Helper()
	resp, err := http.Post(ts.baseURL+path, "application/json", nil)
	if err != nil {
		ts.t.Fatalf("HTTP POST %s 失败: %v", path, err)
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		ts.t.Fatalf("读取响应体失败: %v", err)
	}
	return resp, body
}

// postWithAuth sends a POST request with an optional Bearer token.
func (ts *testServer) postWithAuth(path, token string) (*http.Response, []byte) {
	ts.t.Helper()
	req, err := http.NewRequest("POST", ts.baseURL+path, nil)
	if err != nil {
		ts.t.Fatalf("创建请求失败: %v", err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		ts.t.Fatalf("HTTP POST %s 失败: %v", path, err)
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		ts.t.Fatalf("读取响应体失败: %v", err)
	}
	return resp, body
}

// createTestFile writes a file with the given content into dir.
func createTestFile(t *testing.T, dir, name string, content []byte) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("创建测试文件失败 %s: %v", name, err)
	}
}

// sha256Hex computes the hex-encoded SHA-256 hash of data.
func sha256Hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// --- Mock SyncSource for online fallback tests ---

type mockFile struct {
	content  []byte
	filename string
}

type e2eMockSyncSource struct {
	versions map[string][]SyncVersionInfo
	files    map[string]mockFile // key: DownloadURL
}

func newE2EMockSyncSource() *e2eMockSyncSource {
	return &e2eMockSyncSource{
		versions: make(map[string][]SyncVersionInfo),
		files:    make(map[string]mockFile),
	}
}

// addVersion adds a version to the mock source with its downloadable content.
// sha 参数为原始 SHA256 哈希（无算法前缀），为空时表示无校验和。
func (m *e2eMockSyncSource) addVersion(browser, ver, channel, platform, arch, filename string, content []byte, sha string) {
	url := "http://mock/" + filename
	checksum := ""
	if sha != "" {
		checksum = "sha256:" + sha
	}
	m.versions[browser] = append(m.versions[browser], SyncVersionInfo{
		Browser:     browser,
		Version:     ver,
		Channel:     channel,
		Platform:    platform,
		Arch:        arch,
		DownloadURL: url,
		Filename:    filename,
		Size:        int64(len(content)),
		Checksum:    checksum,
	})
	m.files[url] = mockFile{content: content, filename: filename}
}

func (m *e2eMockSyncSource) ListVersions(_ context.Context, browser, channel, platform, arch string) ([]SyncVersionInfo, error) {
	return m.versions[browser], nil
}

func (m *e2eMockSyncSource) Download(url string, destDir string, onProgress func(int64, int64)) (string, error) {
	mf, ok := m.files[url]
	if !ok {
		return "", fmt.Errorf("file not found for URL: %s", url)
	}
	destPath := filepath.Join(destDir, mf.filename)
	if err := os.WriteFile(destPath, mf.content, 0o644); err != nil {
		return "", err
	}
	return destPath, nil
}

func (m *e2eMockSyncSource) GetChecksum(_ context.Context, browser, version, platform, arch string) (string, error) {
	for _, v := range m.versions[browser] {
		if v.Version == version && v.Platform == platform && v.Arch == arch {
			return v.Checksum, nil
		}
	}
	return "", nil
}

// --- Manifest API tests ---

func TestE2E_Manifest_Empty(t *testing.T) {
	ts := startTestServer(t, ServerOptions{})

	resp, body := ts.get("/api/v1/manifest")
	if resp.StatusCode != 200 {
		t.Fatalf("状态码 = %d, 期望 200", resp.StatusCode)
	}

	var m ManifestResponse
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("解析 manifest 失败: %v", err)
	}
	if m.Status != "ok" {
		t.Errorf("status = %q, 期望 \"ok\"", m.Status)
	}
	if len(m.Data) != 0 {
		t.Errorf("data 长度 = %d, 期望 0", len(m.Data))
	}
	if m.Server.FileCount != 0 {
		t.Errorf("file_count = %d, 期望 0", m.Server.FileCount)
	}
}

func TestE2E_Manifest_WithPackages(t *testing.T) {
	ts := startTestServer(t, ServerOptions{})

	createTestFile(t, ts.packagesDir, "chrome_120.0.6099.109_win64.zip", []byte("chrome content"))
	createTestFile(t, ts.packagesDir, "firefox_120.0esr_linux64.tar.bz2", []byte("firefox content"))

	// Re-scan to pick up the new files.
	cache, _ := ts.server.loadCache()
	if err := ts.server.scanPackages(cache); err != nil {
		t.Fatalf("重新扫描失败: %v", err)
	}

	resp, body := ts.get("/api/v1/manifest")
	if resp.StatusCode != 200 {
		t.Fatalf("状态码 = %d, 期望 200", resp.StatusCode)
	}

	var m ManifestResponse
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("解析 manifest 失败: %v", err)
	}

	// Should contain at least the two files we created.
	filenames := make(map[string]bool)
	for _, f := range m.Data {
		filenames[f.Filename] = true
	}
	if !filenames["chrome_120.0.6099.109_win64.zip"] {
		t.Error("manifest 缺少 chrome_120.0.6099.109_win64.zip")
	}
	if !filenames["firefox_120.0esr_linux64.tar.bz2"] {
		t.Error("manifest 缺少 firefox_120.0esr_linux64.tar.bz2")
	}
}

func TestE2E_Manifest_ServerInfo(t *testing.T) {
	ts := startTestServer(t, ServerOptions{})

	_, body := ts.get("/api/v1/manifest")

	var m ManifestResponse
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("解析 manifest 失败: %v", err)
	}
	if m.Server.Name != serverName {
		t.Errorf("server.name = %q, 期望 %q", m.Server.Name, serverName)
	}
	if m.Server.Version != "test" {
		t.Errorf("server.version = %q, 期望 \"test\"", m.Server.Version)
	}
}

func TestE2E_Manifest_MethodNotAllowed(t *testing.T) {
	ts := startTestServer(t, ServerOptions{})

	resp, _ := ts.post("/api/v1/manifest")
	if resp.StatusCode != 405 {
		t.Errorf("状态码 = %d, 期望 405", resp.StatusCode)
	}
}

// --- Download API tests ---

func TestE2E_Download_LocalFile(t *testing.T) {
	ts := startTestServer(t, ServerOptions{})

	content := []byte("test package content for download")
	createTestFile(t, ts.packagesDir, "chrome_120.0.6099.109_win64.zip", content)

	// Re-scan to pick up the new file.
	cache, _ := ts.server.loadCache()
	_ = ts.server.scanPackages(cache)

	resp, body := ts.get("/api/v1/download/chrome_120.0.6099.109_win64.zip")
	if resp.StatusCode != 200 {
		t.Fatalf("状态码 = %d, 期望 200", resp.StatusCode)
	}
	if !bytes.Equal(body, content) {
		t.Errorf("下载内容不匹配: got %q, want %q", body, content)
	}
	if cd := resp.Header.Get("Content-Disposition"); !strings.Contains(cd, "chrome_120.0.6099.109_win64.zip") {
		t.Errorf("Content-Disposition 不包含文件名: %s", cd)
	}
}

func TestE2E_Download_NotFound(t *testing.T) {
	ts := startTestServer(t, ServerOptions{})

	resp, _ := ts.get("/api/v1/download/nonexistent_file.zip")
	if resp.StatusCode != 404 {
		t.Errorf("状态码 = %d, 期望 404", resp.StatusCode)
	}
}

func TestE2E_Download_PathTraversal(t *testing.T) {
	ts := startTestServer(t, ServerOptions{})

	// Use a simple relative path traversal attempt.
	resp, _ := ts.get("/api/v1/download/..%2F..%2Fsecret.txt")
	if resp.StatusCode != 400 && resp.StatusCode != 404 {
		t.Errorf("状态码 = %d, 期望 400 或 404", resp.StatusCode)
	}
}

func TestE2E_Download_EmptyFilename(t *testing.T) {
	ts := startTestServer(t, ServerOptions{})

	resp, _ := ts.get("/api/v1/download/")
	if resp.StatusCode != 400 {
		t.Errorf("状态码 = %d, 期望 400", resp.StatusCode)
	}
}

func TestE2E_Download_MethodNotAllowed(t *testing.T) {
	ts := startTestServer(t, ServerOptions{})

	resp, _ := ts.post("/api/v1/download/chrome_120.zip")
	if resp.StatusCode != 405 {
		t.Errorf("状态码 = %d, 期望 405", resp.StatusCode)
	}
}

func TestE2E_Download_Range(t *testing.T) {
	ts := startTestServer(t, ServerOptions{})

	content := []byte("0123456789ABCDEF")
	createTestFile(t, ts.packagesDir, "chrome_120.0.6099.109_win64.zip", content)

	cache, _ := ts.server.loadCache()
	_ = ts.server.scanPackages(cache)

	req, err := http.NewRequest("GET", ts.baseURL+"/api/v1/download/chrome_120.0.6099.109_win64.zip", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Range", "bytes=0-4")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 206 {
		t.Errorf("状态码 = %d, 期望 206", resp.StatusCode)
	}
	if string(body) != "01234" {
		t.Errorf("Range 内容 = %q, 期望 \"01234\"", body)
	}
}

// --- Status API tests ---

func TestE2E_Status(t *testing.T) {
	ts := startTestServer(t, ServerOptions{})

	createTestFile(t, ts.packagesDir, "chrome_120.0.6099.109_win64.zip", []byte("x"))
	cache, _ := ts.server.loadCache()
	_ = ts.server.scanPackages(cache)

	resp, body := ts.get("/api/v1/status")
	if resp.StatusCode != 200 {
		t.Fatalf("状态码 = %d, 期望 200", resp.StatusCode)
	}

	var s StatusResponse
	if err := json.Unmarshal(body, &s); err != nil {
		t.Fatalf("解析 status 失败: %v", err)
	}
	if s.Status != "ok" {
		t.Errorf("status = %q, 期望 \"ok\"", s.Status)
	}
	if s.Server.Uptime < 0 {
		t.Errorf("uptime = %d, 应为非负", s.Server.Uptime)
	}
	if s.Server.FileCount < 1 {
		t.Errorf("file_count = %d, 期望 >= 1", s.Server.FileCount)
	}
	if s.Server.Name != serverName {
		t.Errorf("name = %q, 期望 %q", s.Server.Name, serverName)
	}
}

func TestE2E_Status_MethodNotAllowed(t *testing.T) {
	ts := startTestServer(t, ServerOptions{})

	resp, _ := ts.post("/api/v1/status")
	if resp.StatusCode != 405 {
		t.Errorf("状态码 = %d, 期望 405", resp.StatusCode)
	}
}

// --- Sync API tests ---

func TestE2E_SyncStatus_Disabled(t *testing.T) {
	ts := startTestServer(t, ServerOptions{})

	resp, body := ts.get("/api/v1/sync/status")
	if resp.StatusCode != 200 {
		t.Fatalf("状态码 = %d, 期望 200", resp.StatusCode)
	}

	var result struct {
		Status string     `json:"status"`
		Data   SyncStatus `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("解析 sync status 失败: %v", err)
	}
	if result.Data.Running {
		t.Error("running = true, 期望 false (同步未启用)")
	}
}

func TestE2E_SyncStatus_Enabled(t *testing.T) {
	mock := newE2EMockSyncSource()
	ts := startTestServer(t, ServerOptions{
		SyncSource: mock,
		SyncConfig: SyncConfig{
			Interval: 1 * time.Hour,
			Channels:  []string{"stable"},
		},
	})

	resp, body := ts.get("/api/v1/sync/status")
	if resp.StatusCode != 200 {
		t.Fatalf("状态码 = %d, 期望 200", resp.StatusCode)
	}

	var result struct {
		Status string     `json:"status"`
		Data   SyncStatus `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("解析 sync status 失败: %v", err)
	}
	if result.Status != "ok" {
		t.Errorf("status = %q, 期望 \"ok\"", result.Status)
	}
}

func TestE2E_SyncTrigger_Disabled(t *testing.T) {
	ts := startTestServer(t, ServerOptions{})

	resp, _ := ts.post("/api/v1/sync/trigger")
	if resp.StatusCode != 503 {
		t.Errorf("状态码 = %d, 期望 503", resp.StatusCode)
	}
}

func TestE2E_SyncTrigger_Enabled(t *testing.T) {
	mock := newE2EMockSyncSource()
	ts := startTestServer(t, ServerOptions{
		SyncSource: mock,
		SyncConfig: SyncConfig{
			Interval: 1 * time.Hour,
			Channels:  []string{"stable"},
		},
	})

	resp, body := ts.post("/api/v1/sync/trigger")
	if resp.StatusCode != 200 {
		t.Fatalf("状态码 = %d, 期望 200", resp.StatusCode)
	}

	var result struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("解析 sync trigger 响应失败: %v", err)
	}
	if result.Status != "ok" {
		t.Errorf("status = %q, 期望 \"ok\"", result.Status)
	}
	if result.Message != "同步已触发" {
		t.Errorf("message = %q, 期望 \"同步已触发\"", result.Message)
	}
}

func TestE2E_SyncTrigger_MethodNotAllowed(t *testing.T) {
	ts := startTestServer(t, ServerOptions{})

	resp, _ := ts.get("/api/v1/sync/trigger")
	if resp.StatusCode != 405 {
		t.Errorf("状态码 = %d, 期望 405", resp.StatusCode)
	}
}

// --- Bin Download API tests ---

func TestE2E_Bin_Download(t *testing.T) {
	ts := startTestServer(t, ServerOptions{})

	content := []byte("binary content")
	createTestFile(t, ts.binDir, "bws_windows_amd64.zip", content)

	resp, body := ts.get("/api/v1/bin/bws_windows_amd64.zip")
	if resp.StatusCode != 200 {
		t.Fatalf("状态码 = %d, 期望 200", resp.StatusCode)
	}
	if !bytes.Equal(body, content) {
		t.Errorf("下载内容不匹配: got %q, want %q", body, content)
	}
}

func TestE2E_Bin_NotFound(t *testing.T) {
	ts := startTestServer(t, ServerOptions{})

	resp, _ := ts.get("/api/v1/bin/nonexistent.zip")
	if resp.StatusCode != 404 {
		t.Errorf("状态码 = %d, 期望 404", resp.StatusCode)
	}
}

func TestE2E_Bin_PathTraversal(t *testing.T) {
	ts := startTestServer(t, ServerOptions{})

	resp, _ := ts.get("/api/v1/bin/..%2F..%2Fsecret.txt")
	if resp.StatusCode != 400 && resp.StatusCode != 404 {
		t.Errorf("状态码 = %d, 期望 400 或 404", resp.StatusCode)
	}
}

func TestE2E_Bin_EmptyFilename(t *testing.T) {
	ts := startTestServer(t, ServerOptions{})

	resp, _ := ts.get("/api/v1/bin/")
	if resp.StatusCode != 404 {
		t.Errorf("状态码 = %d, 期望 404", resp.StatusCode)
	}
}

// --- Root HTML tests ---

func TestE2E_Root_HTML(t *testing.T) {
	ts := startTestServer(t, ServerOptions{})

	createTestFile(t, ts.packagesDir, "chrome_120.0.6099.109_win64.zip", []byte("x"))
	cache, _ := ts.server.loadCache()
	_ = ts.server.scanPackages(cache)

	resp, body := ts.get("/")
	if resp.StatusCode != 200 {
		t.Fatalf("状态码 = %d, 期望 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Errorf("Content-Type = %q, 期望包含 text/html", ct)
	}
	if !strings.Contains(string(body), serverName) {
		t.Error("HTML 页面不包含服务器名称")
	}
}

func TestE2E_Root_NotFound(t *testing.T) {
	ts := startTestServer(t, ServerOptions{})

	resp, _ := ts.get("/unknown-path")
	if resp.StatusCode != 404 {
		t.Errorf("状态码 = %d, 期望 404", resp.StatusCode)
	}
}

// --- Auth middleware tests ---

func TestE2E_Auth_Required(t *testing.T) {
	ts := startTestServer(t, ServerOptions{
		AuthToken: "secret-token",
	})

	resp, _ := ts.get("/api/v1/status")
	if resp.StatusCode != 401 {
		t.Errorf("状态码 = %d, 期望 401", resp.StatusCode)
	}
	if wwwAuth := resp.Header.Get("WWW-Authenticate"); !strings.Contains(wwwAuth, "Bearer") {
		t.Errorf("WWW-Authenticate = %q, 期望包含 Bearer", wwwAuth)
	}
}

func TestE2E_Auth_CorrectToken(t *testing.T) {
	ts := startTestServer(t, ServerOptions{
		AuthToken: "secret-token",
	})

	resp, _ := ts.getWithAuth("/api/v1/status", "secret-token")
	if resp.StatusCode != 200 {
		t.Errorf("状态码 = %d, 期望 200", resp.StatusCode)
	}
}

func TestE2E_Auth_WrongToken(t *testing.T) {
	ts := startTestServer(t, ServerOptions{
		AuthToken: "secret-token",
	})

	resp, _ := ts.getWithAuth("/api/v1/status", "wrong-token")
	if resp.StatusCode != 401 {
		t.Errorf("状态码 = %d, 期望 401", resp.StatusCode)
	}
}

func TestE2E_Auth_RootOpen(t *testing.T) {
	ts := startTestServer(t, ServerOptions{
		AuthToken: "secret-token",
	})

	resp, _ := ts.get("/")
	if resp.StatusCode != 200 {
		t.Errorf("状态码 = %d, 期望 200 (根页面无需认证)", resp.StatusCode)
	}
}

func TestE2E_Auth_NoTokenOpen(t *testing.T) {
	ts := startTestServer(t, ServerOptions{})

	resp, _ := ts.get("/api/v1/status")
	if resp.StatusCode != 200 {
		t.Errorf("状态码 = %d, 期望 200 (未配置 token 时 API 开放)", resp.StatusCode)
	}
}

// --- Online fallback tests ---

func TestE2E_OnlineFallback_DownloadMissing(t *testing.T) {
	mock := newE2EMockSyncSource()
	content := []byte("online firefox content")
	mock.addVersion("firefox", "120.0esr", "esr", "windows", "amd64",
		"firefox_120.0esr_win64.zip", content, "")

	ts := startTestServer(t, ServerOptions{
		OnlineSource:   mock,
		OnlineFallback: true,
	})

	resp, body := ts.get("/api/v1/download/firefox_120.0esr_win64.zip")
	if resp.StatusCode != 200 {
		t.Fatalf("状态码 = %d, 期望 200 (在线回退应成功)", resp.StatusCode)
	}
	if !bytes.Equal(body, content) {
		t.Errorf("下载内容不匹配: got %q, want %q", body, content)
	}
}

func TestE2E_OnlineFallback_ManifestMerged(t *testing.T) {
	mock := newE2EMockSyncSource()
	content := []byte("online firefox content")
	mock.addVersion("firefox", "120.0esr", "esr", "windows", "amd64",
		"firefox_120.0esr_win64.zip", content, "")

	ts := startTestServer(t, ServerOptions{
		OnlineSource:   mock,
		OnlineFallback: true,
	})

	// Wait for online cache preload (poll manifest until it contains the online entry).
	deadline := time.Now().Add(5 * time.Second)
	var found bool
	for time.Now().Before(deadline) {
		_, body := ts.get("/api/v1/manifest")
		var m ManifestResponse
		if json.Unmarshal(body, &m) == nil {
			for _, f := range m.Data {
				if f.Filename == "firefox_120.0esr_win64.zip" {
					found = true
					break
				}
			}
		}
		if found {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !found {
		t.Fatal("manifest 中未找到在线缓存条目 firefox_120.0esr_win64.zip")
	}
}

func TestE2E_OnlineFallback_SHA256Fail(t *testing.T) {
	mock := newE2EMockSyncSource()
	content := []byte("online firefox content")
	// Set wrong SHA256 to trigger verification failure.
	mock.addVersion("firefox", "120.0esr", "esr", "windows", "amd64",
		"firefox_120.0esr_sha_fail.zip", content, "0000000000000000000000000000000000000000000000000000000000000000")

	ts := startTestServer(t, ServerOptions{
		OnlineSource:   mock,
		OnlineFallback: true,
	})

	resp, _ := ts.get("/api/v1/download/firefox_120.0esr_sha_fail.zip")
	if resp.StatusCode != 404 {
		t.Errorf("状态码 = %d, 期望 404 (SHA256 校验失败应返回 404)", resp.StatusCode)
	}
}

func TestE2E_OnlineFallback_NotFoundInOnline(t *testing.T) {
	mock := newE2EMockSyncSource()
	// Only has firefox versions, not chrome.
	mock.addVersion("firefox", "120.0esr", "esr", "windows", "amd64",
		"firefox_120.0esr_win64.zip", []byte("x"), "")

	ts := startTestServer(t, ServerOptions{
		OnlineSource:   mock,
		OnlineFallback: true,
	})

	resp, _ := ts.get("/api/v1/download/chrome_999.0_nonexistent.zip")
	if resp.StatusCode != 404 {
		t.Errorf("状态码 = %d, 期望 404 (在线源中也不存在)", resp.StatusCode)
	}
}

func TestE2E_OnlineFallback_AfterDownloadInManifest(t *testing.T) {
	mock := newE2EMockSyncSource()
	content := []byte("online firefox content after download")
	mock.addVersion("firefox", "121.0esr", "esr", "windows", "amd64",
		"firefox_121.0esr_win64.zip", content, sha256Hex(content))

	ts := startTestServer(t, ServerOptions{
		OnlineSource:   mock,
		OnlineFallback: true,
	})

	// Step 1: Download the file via online fallback.
	resp, body := ts.get("/api/v1/download/firefox_121.0esr_win64.zip")
	if resp.StatusCode != 200 {
		t.Fatalf("首次下载失败: 状态码 = %d", resp.StatusCode)
	}
	if !bytes.Equal(body, content) {
		t.Errorf("下载内容不匹配")
	}

	// Step 2: Verify the file still appears in manifest (via online cache,
	// no local rescan needed).
	_, mbody := ts.get("/api/v1/manifest")
	var m ManifestResponse
	if err := json.Unmarshal(mbody, &m); err != nil {
		t.Fatalf("解析 manifest 失败: %v", err)
	}

	var found bool
	for _, f := range m.Data {
		if f.Filename == "firefox_121.0esr_win64.zip" {
			found = true
			break
		}
	}
	if !found {
		t.Error("在线回退下载后，manifest 中未找到文件")
	}
}

// --- Package scanning tests ---

func TestE2E_Scan_SupportedExtensions(t *testing.T) {
	ts := startTestServer(t, ServerOptions{})

	createTestFile(t, ts.packagesDir, "chrome_120.0.6099.109_win64.zip", []byte("zip"))
	createTestFile(t, ts.packagesDir, "firefox_120.0esr_win64.exe", []byte("exe"))
	createTestFile(t, ts.packagesDir, "chromium_119.0_linux64.tar.bz2", []byte("bz2"))

	cache, _ := ts.server.loadCache()
	if err := ts.server.scanPackages(cache); err != nil {
		t.Fatalf("扫描失败: %v", err)
	}

	_, body := ts.get("/api/v1/manifest")
	var m ManifestResponse
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("解析 manifest 失败: %v", err)
	}

	filenames := make(map[string]bool)
	for _, f := range m.Data {
		filenames[f.Filename] = true
	}
	if !filenames["chrome_120.0.6099.109_win64.zip"] {
		t.Error("manifest 缺少 .zip 文件")
	}
	if !filenames["firefox_120.0esr_win64.exe"] {
		t.Error("manifest 缺少 .exe 文件")
	}
	if !filenames["chromium_119.0_linux64.tar.bz2"] {
		t.Error("manifest 缺少 .tar.bz2 文件")
	}
}

func TestE2E_Scan_UnsupportedSkipped(t *testing.T) {
	ts := startTestServer(t, ServerOptions{})

	createTestFile(t, ts.packagesDir, "readme.txt", []byte("text"))
	createTestFile(t, ts.packagesDir, "config.json", []byte("{}"))
	createTestFile(t, ts.packagesDir, "notes.md", []byte("# Notes"))

	cache, _ := ts.server.loadCache()
	if err := ts.server.scanPackages(cache); err != nil {
		t.Fatalf("扫描失败: %v", err)
	}

	_, body := ts.get("/api/v1/manifest")
	var m ManifestResponse
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("解析 manifest 失败: %v", err)
	}
	if len(m.Data) != 0 {
		t.Errorf("不支持的扩展名应被跳过, 但 manifest 返回了 %d 个文件", len(m.Data))
	}
}

func TestE2E_Scan_ChecksumFormat(t *testing.T) {
	ts := startTestServer(t, ServerOptions{})

	createTestFile(t, ts.packagesDir, "chrome_120.0.6099.109_win64.zip", []byte("checksum test"))
	cache, _ := ts.server.loadCache()
	_ = ts.server.scanPackages(cache)

	_, body := ts.get("/api/v1/manifest")
	var m ManifestResponse
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("解析 manifest 失败: %v", err)
	}
	if len(m.Data) == 0 {
		t.Fatal("manifest 为空")
	}
	if !strings.HasPrefix(m.Data[0].Checksum, "xxh3:") {
		t.Errorf("checksum = %q, 期望以 \"xxh3:\" 开头", m.Data[0].Checksum)
	}
}
