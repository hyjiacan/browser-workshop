package driver

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"time"

	bmlog "github.com/bws/bws/internal/log"
)

// defaultReadyTimeout is how long Start waits for the driver to accept
// connections before giving up.
const defaultReadyTimeout = 15 * time.Second

// StartOptions configures a driver process start.
type StartOptions struct {
	// Major is the installed driver major version to run (directory key).
	Major string

	// Port is the listen port. 0 lets bws pick a free port.
	Port int

	// AllowedIPs restricts which hosts may talk to the driver.
	// Defaults to 127.0.0.1.
	AllowedIPs string

	// LogFile, when set, receives the driver's stdout/stderr.
	LogFile string

	// Detach starts the process in its own process group so it survives the
	// bws process.
	Detach bool

	// ReadyTimeout overrides the default readiness timeout.
	ReadyTimeout time.Duration
}

// Process represents a running driver process.
type Process struct {
	Cmd  *exec.Cmd
	Pid  int
	Port int
	// URL is the WebDriver endpoint, e.g. "http://127.0.0.1:9515".
	URL string

	exitCh chan error
}

// Wait blocks until the driver process exits.
func (p *Process) Wait() error {
	if p.exitCh == nil {
		return nil
	}
	return <-p.exitCh
}

// Kill terminates the driver process.
func (p *Process) Kill() error {
	if p.Cmd == nil || p.Cmd.Process == nil {
		return fmt.Errorf("驱动进程未启动")
	}
	return p.Cmd.Process.Kill()
}

// Start launches the installed driver for the given major version and waits
// until it accepts connections.
func (m *Manager) Start(ctx context.Context, opts StartOptions) (*Process, error) {
	binary, err := m.BinaryPath(opts.Major)
	if err != nil {
		return nil, err
	}

	port := opts.Port
	if port == 0 {
		port, err = freePort()
		if err != nil {
			return nil, fmt.Errorf("分配端口失败: %w", err)
		}
	}

	allowedIPs := opts.AllowedIPs
	if allowedIPs == "" {
		allowedIPs = "127.0.0.1"
	}

	args := []string{
		"--port=" + strconv.Itoa(port),
		"--allowed-ips=" + allowedIPs,
	}

	cmd := exec.Command(binary, args...)

	if opts.LogFile != "" {
		f, err := os.OpenFile(opts.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return nil, fmt.Errorf("打开驱动日志文件失败: %w", err)
		}
		cmd.Stdout = f
		cmd.Stderr = f
	}

	if opts.Detach {
		setDetached(cmd)
	}

	bmlog.Debug("[driver] 启动驱动: %s %v", binary, args)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("启动驱动失败: %w", err)
	}

	proc := &Process{
		Cmd:    cmd,
		Pid:    cmd.Process.Pid,
		Port:   port,
		URL:    fmt.Sprintf("http://127.0.0.1:%d", port),
		exitCh: make(chan error, 1),
	}
	go func() {
		proc.exitCh <- cmd.Wait()
	}()

	timeout := opts.ReadyTimeout
	if timeout == 0 {
		timeout = defaultReadyTimeout
	}
	if err := waitReady(ctx, port, timeout, proc.exitCh); err != nil {
		_ = proc.Kill()
		return nil, err
	}

	bmlog.Info("[driver] 驱动已就绪: %s (PID %d)", proc.URL, proc.Pid)
	return proc, nil
}

// freePort asks the OS for an unused TCP port on loopback.
func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// waitReady polls the driver port until it accepts connections, the process
// exits, or the timeout elapses.
func waitReady(ctx context.Context, port int, timeout time.Duration, exited <-chan error) error {
	deadline := time.Now().Add(timeout)
	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-exited:
			return fmt.Errorf("驱动进程提前退出: %v", err)
		default:
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("等待驱动就绪超时 (%v)", timeout)
		}

		conn, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
}