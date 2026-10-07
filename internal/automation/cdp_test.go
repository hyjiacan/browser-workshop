package automation

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// CDP-04: a well-formed DevToolsActivePort file yields the browser websocket URL.
func TestReadDevToolsActivePort_Valid(t *testing.T) {
	dir := t.TempDir()
	content := "9222\n/devtools/browser/abc-123\n"
	if err := os.WriteFile(filepath.Join(dir, devToolsActivePortFile), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := readDevToolsActivePort(dir)
	if err != nil {
		t.Fatalf("readDevToolsActivePort error = %v", err)
	}
	want := "ws://127.0.0.1:9222/devtools/browser/abc-123"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// CDP-07: malformed or incomplete files must error without panicking.
func TestReadDevToolsActivePort_Malformed(t *testing.T) {
	cases := map[string]string{
		"empty":       "",
		"single line": "9222\n",
		"bad port":    "notaport\n/devtools/browser/x\n",
		"zero port":   "0\n/devtools/browser/x\n",
		"out of range": "70000\n/devtools/browser/x\n",
		"bad path":    "9222\ndevtools/browser/x\n",
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, devToolsActivePortFile), []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := readDevToolsActivePort(dir); err == nil {
				t.Errorf("内容 %q 应返回错误", content)
			}
		})
	}
}

// CDP-04: discovery succeeds immediately when the profile file is present.
func TestDiscoverCDP_FromFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, devToolsActivePortFile), []byte("9223\n/devtools/browser/xyz\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := DiscoverCDP(context.Background(), dir, 0, 2*time.Second)
	if err != nil {
		t.Fatalf("DiscoverCDP error = %v", err)
	}
	if got != "ws://127.0.0.1:9223/devtools/browser/xyz" {
		t.Errorf("got %q", got)
	}
}

// CDP-05: when no profile file exists, discovery falls back to GET /json/version.
func TestDiscoverCDP_HTTPFallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/json/version" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"webSocketDebuggerUrl":"ws://127.0.0.1:9999/devtools/browser/http"}`))
	}))
	defer srv.Close()

	host, portStr, err := net.SplitHostPort(strings.TrimPrefix(srv.URL, "http://"))
	if err != nil {
		t.Fatalf("解析测试服务器地址失败: %v", err)
	}
	_ = host
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("解析测试服务器端口失败: %v", err)
	}

	got, err := DiscoverCDP(context.Background(), "", port, 2*time.Second)
	if err != nil {
		t.Fatalf("DiscoverCDP error = %v", err)
	}
	if got != "ws://127.0.0.1:9999/devtools/browser/http" {
		t.Errorf("got %q", got)
	}
}

// CDP-06: neither source available → timeout error, returned within the bound.
func TestDiscoverCDP_Timeout(t *testing.T) {
	start := time.Now()
	_, err := DiscoverCDP(context.Background(), t.TempDir(), 0, 300*time.Millisecond)
	if err == nil {
		t.Fatal("期望超时错误")
	}
	if !strings.Contains(err.Error(), "超时") {
		t.Errorf("error = %v", err)
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Errorf("超时耗时过长: %v", elapsed)
	}
}

// CDP-06: a cancelled context aborts discovery promptly.
func TestDiscoverCDP_ContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	if _, err := DiscoverCDP(ctx, t.TempDir(), 0, 5*time.Second); err == nil {
		t.Error("期望 context 错误")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("取消后应立即返回, 实际 %v", elapsed)
	}
}

// queryVersionEndpoint surfaces non-200 responses and missing fields as errors.
func TestQueryVersionEndpoint_ErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, portStr, _ := net.SplitHostPort(strings.TrimPrefix(srv.URL, "http://"))
	port, _ := strconv.Atoi(portStr)

	if _, err := queryVersionEndpoint(context.Background(), port); err == nil {
		t.Error("非 200 响应应返回错误")
	}
}