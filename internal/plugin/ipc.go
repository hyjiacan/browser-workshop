package plugin

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"time"
)

// IPCRequest is the JSON message sent to an IPC plugin via stdin.
type IPCRequest struct {
	Event      string `json:"event"`
	Browser    string `json:"browser"`
	Version    string `json:"version"`
	Profile    string `json:"profile"`
	ProfileDir string `json:"profileDir"`
}

// IPCResponse is the JSON message returned by an IPC plugin via stdout.
type IPCResponse struct {
	ExtraArgs []string          `json:"extraArgs,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	Error     string            `json:"error,omitempty"`
}

// ipcReadResult holds the outcome of reading a response from the plugin process.
// It is sent over a channel so the main goroutine can safely consume it
// without racing with the timeout path.
type ipcReadResult struct {
	resp IPCResponse
	err  error
}

// RunIPCPlugin launches an external process plugin and communicates via stdin/stdout JSON.
// The plugin receives an IPCRequest on stdin and must write an IPCResponse to stdout.
// The plugin may output log lines before/after the JSON response; only the first JSON object is parsed.
// If the plugin writes an error field, it is returned as a Go error.
func RunIPCPlugin(execPath string, ctx *ScriptContext) (*IPCResponse, error) {
	// Build request
	req := IPCRequest{
		Event:      "pre_run",
		Browser:    ctx.Browser,
		Version:    ctx.Version,
		Profile:    ctx.Profile,
		ProfileDir: ctx.ProfileDir,
	}

	cmd := exec.Command(execPath)
	cmd.Stderr = os.Stderr // inherit stderr for plugin logging

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("ipc plugin: stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("ipc plugin: stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("ipc plugin: start: %w", err)
	}

	// Send request
	if err := json.NewEncoder(stdin).Encode(req); err != nil {
		stdin.Close()
		cmd.Wait()
		return nil, fmt.Errorf("ipc plugin: write request: %w", err)
	}
	stdin.Close()

	// Read response with timeout.
	// The goroutine sends the result over a channel; on timeout, we kill
	// the process and discard the goroutine's result to avoid a data race.
	resultCh := make(chan ipcReadResult, 1)

	go func() {
		var resp IPCResponse
		decoder := json.NewDecoder(stdout)
		if err := decoder.Decode(&resp); err != nil {
			resultCh <- ipcReadResult{err: fmt.Errorf("ipc plugin: parse response: %w", err)}
			return
		}
		resultCh <- ipcReadResult{resp: resp}
	}()

	select {
	case r := <-resultCh:
		// Response received successfully
		_ = cmd.Wait()
		if r.err != nil {
			return nil, r.err
		}
		if r.resp.Error != "" {
			return nil, fmt.Errorf("ipc plugin: %s", r.resp.Error)
		}
		return &r.resp, nil

	case <-time.After(10 * time.Second):
		// Timeout — kill the process. The goroutine may still be running
		// but its result is discarded via the buffered channel, so there
		// is no data race on shared variables.
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()
		return nil, fmt.Errorf("ipc plugin: timeout after 10s")
	}
}
