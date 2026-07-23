package plugin

import (
	"encoding/json"
	"fmt"
	"io"
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

// RunIPCPlugin launches an external process plugin and communicates via stdin/stdout JSON.
// The plugin receives an IPCRequest on stdin and must write an IPCResponse to stdout.
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
	cmd.Stderr = exec.Command("").Stderr // inherit stderr for plugin logging

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

	// Read response with timeout
	done := make(chan struct{})
	var resp IPCResponse
	var readErr error

	go func() {
		data, err := io.ReadAll(stdout)
		if err != nil {
			readErr = fmt.Errorf("ipc plugin: read response: %w", err)
			close(done)
			return
		}
		if err := json.Unmarshal(data, &resp); err != nil {
			readErr = fmt.Errorf("ipc plugin: parse response: %w", err)
		}
		close(done)
	}()

	select {
	case <-done:
		// response received
	case <-time.After(10 * time.Second):
		cmd.Process.Kill()
		cmd.Wait()
		return nil, fmt.Errorf("ipc plugin: timeout after 10s")
	}

	// Wait for process to exit
	cmd.Wait()

	if readErr != nil {
		return nil, readErr
	}

	if resp.Error != "" {
		return nil, fmt.Errorf("ipc plugin: %s", resp.Error)
	}

	return &resp, nil
}