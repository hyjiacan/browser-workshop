package main

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/bws/bws/internal/config"
)

// JSON-01: --json anywhere before "--" suppresses the banner.
func TestWantsJSON(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want bool
	}{
		{"裸 --json", []string{"ps", "--json"}, true},
		{"--json=value 形式", []string{"run", "chrome@120", "--json=true"}, true},
		{"--json 与其它标志混用", []string{"run", "chrome@120", "--automation", "--json"}, true},
		{"无 --json", []string{"ls"}, false},
		{"空参数", nil, false},
		{"-- 之后的 --json 属于浏览器参数", []string{"run", "chrome@120", "--", "--json"}, false},
		{"前缀相近的其它标志不误判", []string{"ls", "--jsonify"}, false},
	}
	for _, c := range cases {
		if got := wantsJSON(c.args); got != c.want {
			t.Errorf("%s: wantsJSON(%v) = %v, 期望 %v", c.name, c.args, got, c.want)
		}
	}
}

// JSON-02: quiet (JSON) mode suppresses every line of the setup wizard's
// epilogue, so a first-run `--json` invocation emits nothing but the payload.
func TestSaveInitialConfigQuietSuppressesOutput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bws-client.ini")

	if out := captureStderr(t, func() { saveInitialConfig(path, config.Default(), true) }); out != "" {
		t.Errorf("quiet 模式仍向 stderr 输出: %q", out)
	}
	if out := captureStderr(t, func() { saveInitialConfig(path, config.Default(), false) }); out == "" {
		t.Error("非 quiet 模式应输出初始化提示")
	}
}

// captureStderr redirects os.Stderr for the duration of fn and returns
// everything written to it.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()

	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("创建管道失败: %v", err)
	}
	os.Stderr = w

	done := make(chan string, 1)
	go func() {
		b, _ := io.ReadAll(r)
		done <- string(b)
	}()

	fn()

	_ = w.Close()
	os.Stderr = old
	out := <-done
	_ = r.Close()
	return out
}

// AUTH-06: the token is only forwarded to the configured serve origin.
func TestSameOrigin(t *testing.T) {
	base := "http://192.168.1.10:8080"
	cases := []struct {
		name string
		url  string
		want bool
	}{
		{"同源下载路径", base + "/api/v1/download/x.zip", true},
		{"不同端口", "http://192.168.1.10:9090/api/v1/download/x.zip", false},
		{"不同主机", "http://evil.example/x.zip", false},
		{"前缀相似但不同域", "http://192.168.1.10:8080.evil.example/x.zip", false},
		{"不同协议", "https://192.168.1.10:8080/x.zip", false},
		{"第三方源", "https://ftp.mozilla.org/pub/firefox.exe", false},
	}
	for _, c := range cases {
		if got := sameOrigin(base, c.url); got != c.want {
			t.Errorf("%s: sameOrigin(%q, %q) = %v, 期望 %v", c.name, base, c.url, got, c.want)
		}
	}
}