package automation

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// DefaultEndpointTimeout bounds how long endpoint discovery waits before giving
// up. Endpoint failures never block the browser launch (design principle 13).
const DefaultEndpointTimeout = 10 * time.Second

// devToolsActivePortFile is written by Chromium into the user-data directory
// once the debugging server is listening. Its first line is the port, its
// second line is the browser-level websocket path.
const devToolsActivePortFile = "DevToolsActivePort"

// DiscoverCDP waits for the browser's CDP websocket endpoint.
//
// It first reads <profileDir>/DevToolsActivePort, which works for both
// OS-assigned (port 0) and fixed ports, then falls back to querying
// http://127.0.0.1:<port>/json/version when a fixed port is known.
func DiscoverCDP(ctx context.Context, profileDir string, port int, timeout time.Duration) (string, error) {
	if timeout <= 0 {
		timeout = DefaultEndpointTimeout
	}
	deadline := time.Now().Add(timeout)

	for {
		if profileDir != "" {
			if endpoint, err := readDevToolsActivePort(profileDir); err == nil {
				return endpoint, nil
			}
		}
		if port > 0 {
			if endpoint, err := queryVersionEndpoint(ctx, port); err == nil {
				return endpoint, nil
			}
		}

		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}

		if time.Now().After(deadline) {
			return "", fmt.Errorf("发现 CDP 端点超时 (%v)", timeout)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// readDevToolsActivePort parses the DevToolsActivePort file.
func readDevToolsActivePort(profileDir string) (string, error) {
	data, err := os.ReadFile(filepath.Join(profileDir, devToolsActivePortFile))
	if err != nil {
		return "", err
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) < 2 {
		return "", fmt.Errorf("DevToolsActivePort 内容不完整")
	}
	portStr := strings.TrimSpace(lines[0])
	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 || port > 65535 {
		return "", fmt.Errorf("DevToolsActivePort 端口无效: %q", portStr)
	}
	path := strings.TrimSpace(lines[1])
	if path == "" || !strings.HasPrefix(path, "/") {
		return "", fmt.Errorf("DevToolsActivePort 路径无效: %q", path)
	}
	return fmt.Sprintf("ws://127.0.0.1:%d%s", port, path), nil
}

// queryVersionEndpoint asks the HTTP debugging endpoint for its websocket URL.
func queryVersionEndpoint(ctx context.Context, port int) (string, error) {
	url := fmt.Sprintf("http://127.0.0.1:%d/json/version", port)
	reqCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("调试端点返回状态 %d", resp.StatusCode)
	}

	var payload struct {
		WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}
	if payload.WebSocketDebuggerURL == "" {
		return "", fmt.Errorf("调试端点未返回 webSocketDebuggerUrl")
	}
	return payload.WebSocketDebuggerURL, nil
}
