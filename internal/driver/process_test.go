package driver

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/bws/bws/internal/paths"
)

// fakeDriverEnv marks a child process spawned as a stand-in chromedriver.
const fakeDriverEnv = "BWS_FAKE_DRIVER"

// TestMain intercepts the fake-driver child before the testing flag parser
// runs, so Start can launch this very test binary as if it were a driver.
func TestMain(m *testing.M) {
	if os.Getenv(fakeDriverEnv) == "1" {
		runFakeDriver()
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// runFakeDriver listens on the --port= passed by Start and blocks until it is
// killed, mimicking chromedriver's readiness behaviour.
func runFakeDriver() {
	port := 0
	for _, a := range os.Args[1:] {
		if strings.HasPrefix(a, "--port=") {
			port, _ = strconv.Atoi(strings.TrimPrefix(a, "--port="))
		}
	}
	if port == 0 {
		os.Exit(2)
	}
	ln, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		os.Exit(3)
	}
	for {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		_ = conn.Close()
	}
}

// installFakeDriver points a driver major version at this test binary so that
// Start launches it in fake-driver mode. No archive is downloaded.
func installFakeDriver(t *testing.T, mgr *Manager, major string) {
	t.Helper()
	self, err := filepath.Abs(os.Args[0])
	if err != nil {
		t.Fatalf("解析测试二进制路径失败: %v", err)
	}
	rec := &Record{
		Name:         NameChromedriver,
		Version:      major + ".0.0.0",
		MajorVersion: major,
		Platform:     paths.Platform(),
		Arch:         paths.Arch(),
		Dir:          filepath.Dir(self),
		Executable:   filepath.Base(self),
		InstalledAt:  time.Now(),
	}
	if err := mgr.writeRecord(rec); err != nil {
		t.Fatalf("写入驱动记录失败: %v", err)
	}
}

func TestStartBinaryMissing(t *testing.T) {
	mgr := newTestManager(t)
	if _, err := mgr.Start(context.Background(), StartOptions{Major: "120"}); err == nil {
		t.Fatal("未安装驱动时应返回错误")
	}
}

func TestStartBadLogFile(t *testing.T) {
	mgr := newTestManager(t)
	installFakeDriver(t, mgr, "120")

	// The parent directory does not exist, so opening the log file must fail
	// before any process is spawned.
	badLog := filepath.Join(t.TempDir(), "missing-dir", "driver.log")
	if _, err := mgr.Start(context.Background(), StartOptions{Major: "120", LogFile: badLog}); err == nil {
		t.Fatal("日志文件不可写时应返回错误")
	}
}

func TestStartAutoPortAndReady(t *testing.T) {
	mgr := newTestManager(t)
	installFakeDriver(t, mgr, "120")
	t.Setenv(fakeDriverEnv, "1")

	proc, err := mgr.Start(context.Background(), StartOptions{Major: "120", ReadyTimeout: 10 * time.Second})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() {
		_ = proc.Kill()
		_ = proc.Wait()
	}()

	if proc.Port == 0 {
		t.Error("Port = 0, 期望自动分配的端口")
	}
	if proc.Pid <= 0 {
		t.Errorf("Pid = %d, 期望 > 0", proc.Pid)
	}
	wantURL := fmt.Sprintf("http://127.0.0.1:%d", proc.Port)
	if proc.URL != wantURL {
		t.Errorf("URL = %q, 期望 %q", proc.URL, wantURL)
	}

	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(proc.Port)), 2*time.Second)
	if err != nil {
		t.Fatalf("连接驱动端口失败: %v", err)
	}
	_ = conn.Close()
}

func TestStartExplicitPortAndDetach(t *testing.T) {
	mgr := newTestManager(t)
	installFakeDriver(t, mgr, "121")
	t.Setenv(fakeDriverEnv, "1")

	port, err := freePort()
	if err != nil {
		t.Fatalf("freePort: %v", err)
	}

	proc, err := mgr.Start(context.Background(), StartOptions{
		Major:        "121",
		Port:         port,
		Detach:       true,
		ReadyTimeout: 10 * time.Second,
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() {
		_ = proc.Kill()
		_ = proc.Wait()
	}()

	if proc.Port != port {
		t.Errorf("Port = %d, 期望 %d", proc.Port, port)
	}
	if proc.URL != fmt.Sprintf("http://127.0.0.1:%d", port) {
		t.Errorf("URL = %q", proc.URL)
	}
}

func TestStartLogFileCapturesOutput(t *testing.T) {
	mgr := newTestManager(t)
	installFakeDriver(t, mgr, "122")
	t.Setenv(fakeDriverEnv, "1")

	logFile := filepath.Join(t.TempDir(), "driver.log")
	proc, err := mgr.Start(context.Background(), StartOptions{
		Major:        "122",
		LogFile:      logFile,
		ReadyTimeout: 10 * time.Second,
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() {
		_ = proc.Kill()
		_ = proc.Wait()
	}()

	if _, err := os.Stat(logFile); err != nil {
		t.Errorf("日志文件未创建: %v", err)
	}
}

func TestFreePort(t *testing.T) {
	port, err := freePort()
	if err != nil {
		t.Fatalf("freePort: %v", err)
	}
	if port <= 0 || port > 65535 {
		t.Fatalf("port = %d, 超出范围", port)
	}
	// The returned port must be immediately bindable.
	ln, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		t.Fatalf("在返回的端口上监听失败: %v", err)
	}
	_ = ln.Close()
}

func TestWaitReadySucceeds(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			_ = conn.Close()
		}
	}()

	exited := make(chan error, 1)
	if err := waitReady(context.Background(), port, 2*time.Second, exited); err != nil {
		t.Fatalf("waitReady: %v", err)
	}
}

func TestWaitReadyTimeout(t *testing.T) {
	port, err := freePort()
	if err != nil {
		t.Fatalf("freePort: %v", err)
	}
	exited := make(chan error, 1)
	err = waitReady(context.Background(), port, 300*time.Millisecond, exited)
	if err == nil {
		t.Fatal("无监听时应超时")
	}
	if !strings.Contains(err.Error(), "超时") {
		t.Errorf("error = %v, 期望超时错误", err)
	}
}

func TestWaitReadyProcessExited(t *testing.T) {
	exited := make(chan error, 1)
	exited <- fmt.Errorf("boom")
	err := waitReady(context.Background(), 1, 2*time.Second, exited)
	if err == nil || !strings.Contains(err.Error(), "提前退出") {
		t.Fatalf("error = %v, 期望进程提前退出错误", err)
	}
}

func TestWaitReadyContextCanceled(t *testing.T) {
	port, err := freePort()
	if err != nil {
		t.Fatalf("freePort: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	exited := make(chan error, 1)
	if err := waitReady(ctx, port, 2*time.Second, exited); err != context.Canceled {
		t.Fatalf("error = %v, 期望 context.Canceled", err)
	}
}
