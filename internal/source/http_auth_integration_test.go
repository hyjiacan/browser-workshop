package source

import (
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	bmlog "github.com/bws/bws/internal/log"
	"github.com/bws/bws/internal/serve"
)

// startServeServerWithToken starts a real serve instance protected by the given
// bearer token and returns its base URL.
func startServeServerWithToken(t *testing.T, token string) string {
	t.Helper()
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
	if err := os.WriteFile(filepath.Join(packagesDir, "chrome_120.0.6099.109_win64.zip"), []byte("chrome 120"), 0o644); err != nil {
		t.Fatalf("创建测试文件失败: %v", err)
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
		AuthToken:   token,
		Logger:      bmlog.New(bmlog.LevelError, io.Discard),
		ScanWorkers: 1,
	})
	go func() {
		if err := server.Start(); err != nil && err != http.ErrServerClosed {
			t.Errorf("服务器启动失败: %v", err)
		}
	}()
	t.Cleanup(func() { _ = server.Stop() })

	baseURL := "http://" + addr

	// /api/v1/status is behind the auth middleware too, so the readiness probe
	// must present the token; a 200 confirms the server is up and enforcing auth.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		req, _ := http.NewRequest(http.MethodGet, baseURL+"/api/v1/status", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return baseURL
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("服务器启动超时")
	return ""
}

// AUTH-07: the client authenticates against a token-protected serve instance,
// and fails loudly when the token is missing.
func TestHTTPSourceIntegration_AuthToken(t *testing.T) {
	baseURL := startServeServerWithToken(t, "s3cr3t")

	// Without a token the manifest request is rejected.
	if _, err := NewHTTPSource(baseURL).List(context.Background(), &Filter{Browser: "chrome"}); err == nil {
		t.Error("缺少令牌时应返回错误")
	}

	// With the matching token the manifest is returned.
	versions, err := NewHTTPSourceWithOptions(baseURL, "", "s3cr3t").List(context.Background(), &Filter{Browser: "chrome"})
	if err != nil {
		t.Fatalf("携带令牌时 List 失败: %v", err)
	}
	if len(versions) == 0 {
		t.Error("携带令牌时未返回任何版本")
	}
}
