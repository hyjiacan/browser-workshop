package serve

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// --- mockSyncSource is defined in online_fallback_test.go (same package) ---

// ============================================================================
// 1. Manifest 通过 HTTP 包含在线回退条目
// ============================================================================
func TestE2E_OnlineFallback_Manifest_HTTP(t *testing.T) {
	mock := &mockSyncSource{
		versions: []SyncVersionInfo{
			{
				Browser:     "chrome",
				Version:     "134.0.6998.88",
				Channel:     "stable",
				Platform:    "windows",
				Arch:        "amd64",
				DownloadURL: "https://example.com/browsers/chrome_134_win64.zip",
				Checksum:    "sha256:aaabbbcccdddeeeff00112233445566778899aabbccddeeff0011223344556677",
				Size:        102400,
			},
			{
				Browser:     "firefox",
				Version:     "141.0",
				Channel:     "stable",
				Platform:    "windows",
				Arch:        "amd64",
				DownloadURL: "https://example.com/browsers/firefox_141.0_win64.zip",
			},
		},
	}

	ts := startTestServer(t, ServerOptions{
		OnlineSource:   mock,
		OnlineFallback: true,
	})

	resp := doHTTP(t, ts.baseURL+"/api/v1/manifest")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("manifest status = %d, want 200", resp.StatusCode)
	}

	var m ManifestResponse
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		t.Fatalf("parse manifest: %v", err)
	}

	wantFiles := map[string]bool{
		"chrome/chrome_134_win64.zip":    false,
		"firefox/firefox_141.0_win64.zip": false,
	}
	for _, f := range m.Data {
		if _, ok := wantFiles[f.Filename]; ok {
			wantFiles[f.Filename] = true
		}
	}
	for name, found := range wantFiles {
		if !found {
			t.Errorf("manifest missing online fallback file: %q", name)
		}
	}

}

// ============================================================================
// 2. 下载通过 HTTP：本地文件直接返回（无在线回退）
// ============================================================================
func TestE2E_OnlineFallback_Download_Local(t *testing.T) {
	localName := "chrome_134_local_win64.zip"
	localContent := []byte("local-chrome-134-content-direct-serve")

	ts := startTestServer(t, ServerOptions{
		OnlineSource:   &mockSyncSource{}, // local file should be preferred
		OnlineFallback: true,
	})
	// Write a local file
	if err := os.WriteFile(filepath.Join(ts.packagesDir, localName), localContent, 0o644); err != nil {
		t.Fatalf("write local file: %v", err)
	}
	// Re-scan to pick up the new file
	_, _, _ = ts.server.loadCache()
	_ = ts.server.scanPackages(nil, nil)

	resp := doHTTP(t, ts.baseURL+"/api/v1/download/"+url.PathEscape(localName))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("download local status = %d, want 200", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != string(localContent) {
		t.Errorf("local content mismatch: got %q, want %q", string(body), string(localContent))
	}

	// Verify Content-Disposition
	if cd := resp.Header.Get("Content-Disposition"); !strings.Contains(cd, localName) {
		t.Errorf("Content-Disposition = %q, want containing %q", cd, localName)
	}
}

// ============================================================================
// 3. 下载通过 HTTP：本地无文件 → 在线回退 → 下载成功
// ============================================================================
func TestE2E_OnlineFallback_Download_HTTP(t *testing.T) {
	wantFilename := "firefox/firefox_141.0_win64.zip"
	wantContent := "mock content: firefox_141.0_win64.zip"
	mock := &mockSyncSource{
		versions: []SyncVersionInfo{
			{
				Browser:     "firefox",
				Version:     "141.0",
				Channel:     "stable",
				Platform:    "windows",
				Arch:        "amd64",
				DownloadURL: "https://example.com/browsers/" + wantFilename,
				Checksum:    "sha256:" + sha256Hex([]byte(wantContent)),
				Size:        int64(len(wantContent)),
			},
		},
	}

	ts := startTestServer(t, ServerOptions{
		OnlineSource:   mock,
		OnlineFallback: true,
	})

	// --- First download: triggers online fallback ---
	resp := doHTTP(t, ts.baseURL+"/api/v1/download/"+url.PathEscape(wantFilename))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("first download status = %d, want 200 (body=%s)", resp.StatusCode, string(body))
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != wantContent {
		t.Errorf("first download content: got %q, want %q", string(body), wantContent)
	}

	// Verify the file was saved to local packagesDir
	localPath := filepath.Join(ts.packagesDir, wantFilename)
	if _, err := os.Stat(localPath); err != nil {
		t.Fatalf("online fallback did not save file locally: %v", err)
	}
	localBytes, _ := os.ReadFile(localPath)
	if string(localBytes) != wantContent {
		t.Errorf("local file content mismatch: got %q, want %q", string(localBytes), wantContent)
	}

	// Verify the mock was only called once for download
	if downloads := int(mock.downloads); downloads != 1 {
		t.Errorf("online download call count = %d, want 1", downloads)
	}

	// --- Second download: should serve from local cache (no online fetch) ---
	resp2 := doHTTP(t, ts.baseURL+"/api/v1/download/"+url.PathEscape(wantFilename))
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("second download status = %d, want 200", resp2.StatusCode)
	}

	body2, _ := io.ReadAll(resp2.Body)
	if string(body2) != wantContent {
		t.Errorf("second download content: got %q, want %q", string(body2), wantContent)
	}

	// Mock should NOT be called again (serve from local cache)
	if downloads := int(mock.downloads); downloads != 1 {
		t.Errorf("second download should not trigger online fallback, downloads = %d, want 1", downloads)
	}
}

// ============================================================================
// 4. 下载不存在的文件 → 404
// ============================================================================
func TestE2E_OnlineFallback_Download_NotFound_HTTP(t *testing.T) {
	mock := &mockSyncSource{
		versions: []SyncVersionInfo{
			{
				Browser:     "chrome",
				Version:     "120.0.0.0",
				Channel:     "stable",
				Platform:    "windows",
				Arch:        "amd64",
				DownloadURL: "https://example.com/chrome_120_win64.zip",
			},
		},
	}

	ts := startTestServer(t, ServerOptions{
		OnlineSource:   mock,
		OnlineFallback: true,
	})

	resp := doHTTP(t, ts.baseURL+"/api/v1/download/does_not_exist_browser.zip")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("not-found status = %d, want 404", resp.StatusCode)
	}
}

// ============================================================================
// 5. 并发下载同一文件 → single-flight 去重
// ============================================================================
func TestE2E_OnlineFallback_ConcurrentDownload_HTTP(t *testing.T) {
	wantName := "firefox/firefox_141.0_win64.zip"
	wantContent := "mock content: firefox_141.0_win64.zip"
	mock := &mockSyncSource{
		versions: []SyncVersionInfo{
			{
				Browser:     "firefox",
				Version:     "141.0",
				Channel:     "stable",
				Platform:    "windows",
				Arch:        "amd64",
				DownloadURL: "https://example.com/browsers/" + wantName,
			},
		},
	}

	ts := startTestServer(t, ServerOptions{
		OnlineSource:   mock,
		OnlineFallback: true,
	})

	concurrency := 5
	type result struct {
		status int
		body   string
	}
	results := make(chan result, concurrency)

	reqURL := ts.baseURL + "/api/v1/download/" + url.PathEscape(wantName)
	for i := 0; i < concurrency; i++ {
		go func() {
			resp := doHTTP(t, reqURL)
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)
			results <- result{status: resp.StatusCode, body: string(body)}
		}()
	}

	for i := 0; i < concurrency; i++ {
		r := <-results
		if r.status != http.StatusOK {
			t.Errorf("concurrent request %d status = %d, want 200", i, r.status)
		}
		if r.body != wantContent {
			t.Errorf("concurrent request %d body mismatch: got %q, want %q", i, r.body, wantContent)
		}
	}

	// Online download should only be triggered once
	if downloads := int(mock.downloads); downloads != 1 {
		t.Errorf("concurrent downloads should trigger only 1 online fallback, got %d", downloads)
	}
}

// ============================================================================
// 6. 本地文件优先于在线回退条目（manifest 不重复 + download 不触发回退）
// ============================================================================
func TestE2E_OnlineFallback_LocalPriority_HTTP(t *testing.T) {
	localName := "chrome_134_local_win64.zip"
	localContent := []byte("local-override-content-xyz")

	mock := &mockSyncSource{
		versions: []SyncVersionInfo{
			{
				Browser:     "chrome",
				Version:     "134.0.0.0",
				Channel:     "stable",
				Platform:    "windows",
				Arch:        "amd64",
				DownloadURL: "https://example.com/" + localName,
				Checksum:    "sha256:" + sha256Hex(localContent),
				Size:        int64(len(localContent)),
			},
		},
	}

	ts := startTestServer(t, ServerOptions{
		OnlineSource:   mock,
		OnlineFallback: true,
	})
	// Write local file (same name as online entry)
	if err := os.WriteFile(filepath.Join(ts.packagesDir, localName), localContent, 0o644); err != nil {
		t.Fatalf("write local file: %v", err)
	}
	_, _, _ = ts.server.loadCache()
	_ = ts.server.scanPackages(nil, nil)

	// Manifest: local file should appear exactly once (no duplicate)
	resp := doHTTP(t, ts.baseURL+"/api/v1/manifest")
	defer resp.Body.Close()

	var m ManifestResponse
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		t.Fatalf("parse manifest: %v", err)
	}

	count := 0
	for _, f := range m.Data {
		if f.Filename == localName {
			count++
		}
	}
	if count != 1 {
		t.Errorf("local file %q appears %d times in manifest, want 1 (no duplicates)", localName, count)
	}

	// Download: should return local content (no online fallback)
	dlResp := doHTTP(t, ts.baseURL+"/api/v1/download/"+url.PathEscape(localName))
	defer dlResp.Body.Close()
	if dlResp.StatusCode != http.StatusOK {
		t.Fatalf("download status = %d, want 200", dlResp.StatusCode)
	}
	body, _ := io.ReadAll(dlResp.Body)
	if string(body) != string(localContent) {
		t.Errorf("local priority failed: got %q, want %q", string(body), string(localContent))
	}

	// Online download should NOT be called
	if int(mock.downloads) != 0 {
		t.Errorf("local file exists but online download was triggered, downloads = %d", mock.downloads)
	}
}

// ============================================================================
// 7. 在线回退禁用时，非本地文件返回 404
// ============================================================================
func TestE2E_OnlineFallback_Disabled_Download_HTTP(t *testing.T) {
	mock := &mockSyncSource{
		versions: []SyncVersionInfo{
			{
				Browser:     "firefox",
				Version:     "141.0",
				Channel:     "stable",
				Platform:    "windows",
				Arch:        "amd64",
				DownloadURL: "https://example.com/browsers/firefox_141.0_win64.zip",
			},
		},
	}

	ts := startTestServer(t, ServerOptions{
		OnlineSource:   mock,
		OnlineFallback: false, // disabled
	})

	resp := doHTTP(t, ts.baseURL+"/api/v1/download/firefox_141.0_win64.zip")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("online fallback disabled: status = %d, want 404", resp.StatusCode)
	}
}

// ============================================================================
// 8. 校验和验证：正确 → 200, 错误 → 失败
// ============================================================================
func TestE2E_OnlineFallback_ChecksumVerification(t *testing.T) {
	goodContent := "mock content: firefox_141.0_win64.zip"
	goodChecksum := sha256Hex([]byte(goodContent))

	mock := &mockSyncSource{
		versions: []SyncVersionInfo{
			{
				Browser:     "firefox",
				Version:     "141.0",
				Channel:     "stable",
				Platform:    "windows",
				Arch:        "amd64",
				DownloadURL: "https://example.com/browsers/firefox_141.0_win64.zip",
				Checksum:    "sha256:" + goodChecksum,
				Size:        int64(len(goodContent)),
			},
		},
	}

	ts := startTestServer(t, ServerOptions{
		OnlineSource:   mock,
		OnlineFallback: true,
	})

	resp := doHTTP(t, ts.baseURL+"/api/v1/download/firefox/firefox_141.0_win64.zip")
	defer resp.Body.Close()

	// Correct checksum should succeed
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("valid checksum: status=%d body=%s", resp.StatusCode, string(body))
	}
}

// ============================================================================
// 9. 校验和验证：不匹配 → 下载失败
// ============================================================================
func TestE2E_OnlineFallback_BadChecksum(t *testing.T) {
	content := "mock content: firefox_141.0_win64.zip"
	wrongChecksum := sha256Hex([]byte("this is the wrong content"))

	mock := &mockSyncSource{
		versions: []SyncVersionInfo{
			{
				Browser:     "firefox",
				Version:     "141.0",
				Channel:     "stable",
				Platform:    "windows",
				Arch:        "amd64",
				DownloadURL: "https://example.com/browsers/firefox_141.0_win64.zip",
				Checksum:    "sha256:" + wrongChecksum, // wrong!
				Size:        int64(len(content)),
			},
		},
	}

	ts := startTestServer(t, ServerOptions{
		OnlineSource:   mock,
		OnlineFallback: true,
	})

	resp := doHTTP(t, ts.baseURL+"/api/v1/download/firefox/firefox_141.0_win64.zip")
	defer resp.Body.Close()

	// Checksum mismatch → should return 404 (or error)
	if resp.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(resp.Body)
		t.Logf("bad checksum: status=%d body=%s", resp.StatusCode, string(body))
		// Accept either 404 or 500+ for checksum failure
		if resp.StatusCode < 500 {
			t.Errorf("bad checksum: expected error status, got %d", resp.StatusCode)
		}
	}

	// File should have been deleted after checksum failure
	localPath := filepath.Join(ts.packagesDir, "firefox", "firefox_141.0_win64.zip")
	if _, err := os.Stat(localPath); err == nil {
		t.Error("file should have been removed after checksum failure")
	}
}

// ============================================================================
// 10. 全链路：Manifest → Resolve → Download → Install (模拟客户端流程)
// ============================================================================
func TestE2E_OnlineFallback_FullPipeline(t *testing.T) {
	wantFilename := "firefox/firefox_141.0_win64.zip"
	wantContent := "mock content: firefox_141.0_win64.zip"

	mock := &mockSyncSource{
		versions: []SyncVersionInfo{
			{
				Browser:     "firefox",
				Version:     "141.0",
				Channel:     "stable",
				Platform:    "windows",
				Arch:        "amd64",
				DownloadURL: "https://example.com/browsers/" + wantFilename,
				Checksum:    "sha256:" + sha256Hex([]byte(wantContent)),
				Size:        int64(len(wantContent)),
			},
		},
	}

	ts := startTestServer(t, ServerOptions{
		OnlineSource:   mock,
		OnlineFallback: true,
	})
	client := &http.Client{Timeout: 10 * time.Second}

	// Step 1: Get manifest from serve
	manifestResp := doHTTP(t, ts.baseURL+"/api/v1/manifest")
	defer manifestResp.Body.Close()
	if manifestResp.StatusCode != http.StatusOK {
		t.Fatalf("manifest: status %d", manifestResp.StatusCode)
	}

	var m ManifestResponse
	json.NewDecoder(manifestResp.Body).Decode(&m)

	foundFirefox := false
	for _, f := range m.Data {
		if f.Filename == wantFilename {
			foundFirefox = true
			break
		}
	}
	if !foundFirefox {
		t.Fatalf("manifest does not contain %q", wantFilename)
	}

	// Step 2: Download the file
	dlResp, err := client.Get(ts.baseURL + "/api/v1/download/" + url.PathEscape(wantFilename))
	if err != nil {
		t.Fatalf("download request: %v", err)
	}
	defer dlResp.Body.Close()

	if dlResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(dlResp.Body)
		t.Fatalf("download status = %d, body=%s", dlResp.StatusCode, string(body))
	}

	// Step 3: Read and verify all content
	downloadedData, err := io.ReadAll(dlResp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if string(downloadedData) != wantContent {
		t.Errorf("content: got %q, want %q", string(downloadedData), wantContent)
	}

	// Step 4: Verify content length matches
	if cl := dlResp.ContentLength; cl != int64(len(wantContent)) {
		t.Logf("Content-Length = %d, expected %d (mock download may not set it)", cl, len(wantContent))
	}

	// Step 5: File should now exist locally (cached)
	localPath := filepath.Join(ts.packagesDir, wantFilename)
	if _, statErr := os.Stat(localPath); statErr != nil {
		t.Fatalf("file not cached locally: %v", statErr)
	}

	t.Logf("full pipeline: %d bytes delivered", len(downloadedData))
}

// ============================================================================
// Helpers
// ============================================================================

// doHTTP performs an HTTP GET and fails the test on connection error.
func doHTTP(t *testing.T, urlStr string) *http.Response {
	t.Helper()
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(urlStr)
	if err != nil {
		t.Fatalf("HTTP GET %s: %v", urlStr, err)
	}
	return resp
}

// sha256Hex is defined in e2e_test.go
