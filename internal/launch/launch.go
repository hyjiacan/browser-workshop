// Package launch handles launching browser versions with proper isolation.
package launch

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/bws/bws/internal/browser"
	"github.com/bws/bws/internal/fingerprint"
	"github.com/bws/bws/internal/install"
	"github.com/bws/bws/internal/log"
	"github.com/bws/bws/internal/paths"
)

// Manager handles launching browser versions.
type Manager struct {
	paths     *paths.Paths
	browsers  *browser.Registry
	installer *install.Manager
}

// NewManager creates a new launch manager.
func NewManager(p *paths.Paths, br *browser.Registry, inst *install.Manager) *Manager {
	return &Manager{
		paths:     p,
		browsers:  br,
		installer: inst,
	}
}

// Options configures a browser launch.
type Options struct {
	Browser string
	Version string

	// URLs to open
	URLs []string

	// Mode flags
	Headless    bool
	Incognito   bool
	NewWindow   bool

	// Profile options
	ProfileName string // named profile (empty = version-default)
	ProfileDir  string // resolved profile directory (set by Launch)
	Clean       bool   // start with a clean profile

	// NativeMode launches the browser without bm's isolation flags.
	// No --user-data-dir, no --no-first-run, etc.
	// Defaults to true for system browsers, false for bm-managed versions.
	NativeMode bool

	// Extra arguments passed directly to the browser
	ExtraArgs []string

	// Working directory
	WorkingDir string

	// Environment variables (added to the current env)
	Env map[string]string

	// Detached: don't wait for the process to exit
	Detached bool

	// Proxy is the proxy URL to pass to the browser.
	// Supported: http://host:port, socks5://host:port, etc.
	// Empty means no proxy.
	Proxy string

	// Fingerprint is the fingerprint isolation config.
	// nil means no fingerprint isolation.
	Fingerprint *fingerprint.Config

	// Plugins lists plugin names to activate for this launch.
	Plugins []string
}

// Process represents a launched browser process.
type Process struct {
	Cmd        *exec.Cmd
	Pid        int
	Executable string
	Args       []string
	ProfileDir string
	IsSystem   bool // true if launched from system-installed browser
	NativeMode bool // true if launched in native mode
}

// Launch starts a browser version with the given options.
func (m *Manager) Launch(opts Options) (*Process, error) {
	if opts.Browser == "" || opts.Version == "" {
		return nil, errors.New("browser and version are required")
	}

	// Find all matching versions and print the list to stdout
	matches, err := m.installer.FindMatchingVersions(opts.Browser, opts.Version)
	if err != nil {
		return nil, fmt.Errorf("resolving version %s@%s: %w. Install it first with 'bws i %s@%s'", opts.Browser, opts.Version, err, opts.Browser, opts.Version)
	}

	// 通过日志输出匹配的版本信息（避免直接写入 stdout）
	if len(matches) == 1 {
		log.Info("使用 %s@%s", opts.Browser, matches[0].Version)
	} else {
		log.Info("%s@%s 的匹配版本:", opts.Browser, opts.Version)
		for i, v := range matches {
			prefix := "  "
			if i == 0 {
				prefix = "> "
			}
			log.Info("%s%s", prefix, v.Version)
		}
	}

	// The first element is the selected (resolved) version
	resolvedVersion := matches[0].Version
	if resolvedVersion != opts.Version {
		log.Debug("解析版本 %s@%s -> %s", opts.Browser, opts.Version, resolvedVersion)
	}

	// Check if installed (locally or system)
	isSystem := m.installer.IsSystemVersion(opts.Browser, resolvedVersion)
	if !m.installer.IsInstalled(opts.Browser, resolvedVersion) && !isSystem {
		return nil, fmt.Errorf("%s@%s 未安装。请先执行 'bws i %s@%s'", opts.Browser, resolvedVersion, opts.Browser, opts.Version)
	}

	// Get browser descriptor
	desc := m.browsers.Get(opts.Browser)
	if desc == nil {
		return nil, fmt.Errorf("unsupported browser: %s", opts.Browser)
	}

	// Get executable path (supports both local and system)
	exePath, found := m.installer.GetExecutableWithSystem(opts.Browser, resolvedVersion)
	if !found {
		return nil, fmt.Errorf("finding executable for %s@%s", opts.Browser, resolvedVersion)
	}

	// System browsers default to native mode
	nativeMode := opts.NativeMode || isSystem

	// Determine profile directory (skip in native mode)
	var profileDir string
	if !nativeMode {
		// Use the original version spec for profile dir to keep profiles stable
		// when user specifies a partial version like "76"
		profileOpts := opts
		profileOpts.Version = resolvedVersion
		profileDir = m.getProfileDir(profileOpts)
		if err := os.MkdirAll(profileDir, 0o755); err != nil {
			return nil, fmt.Errorf("creating profile directory: %w", err)
		}
		opts.ProfileDir = profileDir
	}

	// Build arguments
	args, err := m.buildArgs(desc, opts, profileDir, nativeMode)
	if err != nil {
		return nil, err
	}

	// Clean up Firefox staged updates before launch.
	// Firefox stores downloaded MAR files in <installDir>/updates/ and will
	// try to apply them on every startup, even when app.update.enabled is
	// false. Removing stale staged updates prevents the "we need to restart"
	// page from appearing in a loop on Linux.
	if desc.Name == "firefox" && !nativeMode {
		cleanupFirefoxStagedUpdates(exePath, profileDir)
	}

	// Build command
	cmd := exec.Command(exePath, args...)

	if opts.WorkingDir != "" {
		cmd.Dir = opts.WorkingDir
	}

	// Build environment. Start with the descriptor's declared env vars
	// (lowest priority), layer runtime-detected ones (e.g. Firefox sandbox
	// fallback when the kernel disallows user namespaces), then let the
	// caller override via opts.Env.
	cmd.Env = os.Environ()
	for k, v := range desc.EnvVars {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	if desc.Name == "firefox" {
		applyFirefoxRuntimeEnv(cmd, exePath)
	}
	if len(opts.Env) > 0 {
		for k, v := range opts.Env {
			cmd.Env = append(cmd.Env, k+"="+v)
		}
	}

	// Configure detached mode
	if opts.Detached {
		setDetached(cmd)
	}

	// Start the process
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting browser: %w", err)
	}

	proc := &Process{
		Cmd:        cmd,
		Pid:        cmd.Process.Pid,
		Executable: exePath,
		Args:       args,
		ProfileDir: profileDir,
		IsSystem:   isSystem,
		NativeMode: nativeMode,
	}

	return proc, nil
}

// Wait waits for a launched process to exit.
func (p *Process) Wait() error {
	if p.Cmd == nil || p.Cmd.Process == nil {
		return errors.New("no process to wait for")
	}
	return p.Cmd.Wait()
}

// Kill terminates a launched process.
func (p *Process) Kill() error {
	if p.Cmd == nil || p.Cmd.Process == nil {
		return errors.New("no process to kill")
	}
	return p.Cmd.Process.Kill()
}

// getProfileDir returns the profile directory path for this launch.
func (m *Manager) getProfileDir(opts Options) string {
	if opts.ProfileName != "" {
		// Named profile shared across versions
		return m.paths.ProfileDir(opts.Browser, "profiles/"+opts.ProfileName)
	}
	// Default: version-specific profile
	return m.paths.ProfileDir(opts.Browser, opts.Version)
}

// buildArgs constructs the command-line arguments for the browser.
// In native mode, no isolation flags (profile, multi-instance, etc.) are added.
func (m *Manager) buildArgs(desc *browser.BrowserDescriptor, opts Options, profileDir string, nativeMode bool) ([]string, error) {
	var args []string

	if !nativeMode {
		// Standard args (multi-instance, no-update, first-run skip)
		args = append(args, desc.BuildStandardArgs()...)

		// Profile directory
		if desc.Features.SupportsProfile {
			args = append(args, desc.BuildProfileArgs(profileDir)...)
		}

		// Standard preferences (e.g. Firefox user.js for disabling updates
		// and default-browser check). Written before proxy/fingerprint prefs
		// so that those can append without conflict.
		if len(desc.StandardPrefs) > 0 && profileDir != "" {
			if err := writeStandardPrefs(desc, profileDir); err != nil {
				return nil, fmt.Errorf("writing standard prefs: %w", err)
			}
		}
	}

	// Mode flags
	if opts.Headless && desc.Features.SupportsHeadless {
		args = append(args, desc.HeadlessArgs()...)
	}
	if opts.Incognito && desc.Features.SupportsIncognito {
		args = append(args, desc.IncognitoArg())
	}
	if opts.NewWindow {
		args = append(args, "--new-window")
	}

	// Proxy
	if opts.Proxy != "" {
		proxyArgs, err := buildProxyArgs(desc, opts.Proxy, profileDir)
		if err != nil {
			return nil, fmt.Errorf("configuring proxy: %w", err)
		}
		args = append(args, proxyArgs...)
	}

	// Fingerprint isolation
	if opts.Fingerprint != nil && !opts.Fingerprint.IsEmpty() {
		fpArgs, err := buildFingerprintArgs(desc, opts.Fingerprint, profileDir)
		if err != nil {
			return nil, fmt.Errorf("configuring fingerprint isolation: %w", err)
		}
		args = append(args, fpArgs...)
	}

	// URLs to open — insert "--" separator before URLs so that any URL
	// starting with "-" is treated as a positional argument (URL) rather
	// than a command-line flag, preventing argument injection.
	if len(opts.URLs) > 0 {
		args = append(args, "--")
		for _, url := range opts.URLs {
			args = append(args, url)
		}
	}

	// Extra args (last, so they can override)
	args = append(args, opts.ExtraArgs...)

	return args, nil
}

// buildProxyArgs constructs proxy-related arguments for the browser.
// Chrome/Chromium uses --proxy-server command-line flag.
// Firefox requires a user.js file in the profile directory (handled via side effect).
func buildProxyArgs(desc *browser.BrowserDescriptor, proxyURL, profileDir string) ([]string, error) {
	switch desc.Name {
	case "chrome", "chromium":
		return []string{"--proxy-server=" + proxyURL}, nil
	case "firefox":
		// Firefox doesn't support command-line proxy.
		// Write user.js in profile dir if available.
		if profileDir != "" {
			if err := writeFirefoxProxyPrefs(profileDir, proxyURL); err != nil {
				return nil, err
			}
		}
		return nil, nil
	default:
		return nil, nil
	}
}

// writeFirefoxProxyPrefs writes proxy preferences to user.js in the profile directory.
// Uses append mode to avoid overwriting existing content (e.g. from fingerprint settings).
func writeFirefoxProxyPrefs(profileDir, proxyURL string) error {
	prefsPath := filepath.Join(profileDir, "user.js")

	parsed, err := url.Parse(proxyURL)
	if err != nil {
		return fmt.Errorf("parsing proxy URL: %w", err)
	}

	host := jsEscapeString(parsed.Hostname())
	port := parsed.Port()
	if port == "" {
		switch parsed.Scheme {
		case "http", "https":
			port = "80"
		case "socks5", "socks5h":
			port = "1080"
		}
	}
	// Validate port is numeric to prevent JS injection
	if !isNumeric(port) {
		return fmt.Errorf("invalid proxy port: %s", port)
	}

	var content string
	if parsed.Scheme == "socks5" || parsed.Scheme == "socks5h" {
		content = fmt.Sprintf("// Proxy settings written by bws\n"+
			"user_pref(\"network.proxy.type\", 1);\n"+
			"user_pref(\"network.proxy.socks\", \"%s\");\n"+
			"user_pref(\"network.proxy.socks_port\", %s);\n"+
			"user_pref(\"network.proxy.socks_version\", 5);\n"+
			"user_pref(\"network.proxy.socks_remote_dns\", true);\n",
			host, port)
	} else {
		// HTTP/HTTPS proxy
		content = fmt.Sprintf("// Proxy settings written by bws\n"+
			"user_pref(\"network.proxy.type\", 1);\n"+
			"user_pref(\"network.proxy.http\", \"%s\");\n"+
			"user_pref(\"network.proxy.http_port\", %s);\n"+
			"user_pref(\"network.proxy.ssl\", \"%s\");\n"+
			"user_pref(\"network.proxy.ssl_port\", %s);\n",
			host, port, host, port)
	}

	// Append to existing user.js instead of overwriting
	var existing string
	if data, err := os.ReadFile(prefsPath); err == nil {
		existing = string(data)
	}
	if !strings.Contains(existing, "Proxy settings written by bws") {
		content = existing + "\n" + content
	} else {
		content = existing // already written, keep as-is
	}
	return os.WriteFile(prefsPath, []byte(content), 0o644)
}

// writeStandardPrefs writes browser-standard preferences (e.g. disabling
// update checks and default-browser checks) to user.js in the profile
// directory. If the preferences are already present (identified by a marker
// comment), the file is left unchanged. This is used for browsers like Firefox
// that do not support these settings via command-line flags.
func writeStandardPrefs(desc *browser.BrowserDescriptor, profileDir string) error {
	if len(desc.StandardPrefs) == 0 || profileDir == "" {
		return nil
	}

	prefsPath := filepath.Join(profileDir, "user.js")

	var existing string
	if data, err := os.ReadFile(prefsPath); err == nil {
		existing = string(data)
	}

	// Skip if already written (idempotent across launches)
	if strings.Contains(existing, "Standard prefs written by bws") {
		return nil
	}

	var content strings.Builder
	content.WriteString("// Standard prefs written by bws\n")
	for _, pref := range desc.StandardPrefs {
		content.WriteString(pref)
		content.WriteString("\n")
	}

	// Append existing content (e.g. proxy or fingerprint prefs from a prior
	// launch) after the standard prefs so that later prefs can override.
	if existing != "" {
		content.WriteString("\n")
		content.WriteString(existing)
	}

	return os.WriteFile(prefsPath, []byte(content.String()), 0o644)
}

// buildFingerprintArgs constructs fingerprint-related arguments for the browser.
// Chrome: uses command-line flags (--user-agent, --lang, --window-size, etc.)
// Firefox: writes user.js preferences to the profile directory.
func buildFingerprintArgs(desc *browser.BrowserDescriptor, cfg *fingerprint.Config, profileDir string) ([]string, error) {
	switch desc.Name {
	case "chrome", "chromium":
		return cfg.ChromeArgs(), nil
	case "firefox":
		// Firefox: write user.js to profile directory
		if profileDir != "" {
			if err := cfg.WriteFirefoxUserJS(profileDir); err != nil {
				return nil, fmt.Errorf("writing fingerprint user.js: %w", err)
			}
		}
		return nil, nil
	default:
		return nil, nil
	}
}

// BuildCommandPreview builds and returns the command that would be executed,
// without actually running it. Useful for --dry-run or debugging.
func (m *Manager) BuildCommandPreview(opts Options) (string, []string, error) {
	if opts.Browser == "" || opts.Version == "" {
		return "", nil, errors.New("browser and version are required")
	}

	// Resolve version (supports partial versions like "76", "latest", "system")
	resolvedVersion, err := m.installer.ResolveInstalledVersion(opts.Browser, opts.Version)
	if err != nil {
		return "", nil, fmt.Errorf("resolving version %s@%s: %w", opts.Browser, opts.Version, err)
	}

	desc := m.browsers.Get(opts.Browser)
	if desc == nil {
		return "", nil, fmt.Errorf("unsupported browser: %s", opts.Browser)
	}

	isSystem := m.installer.IsSystemVersion(opts.Browser, resolvedVersion)
	exePath, found := m.installer.GetExecutableWithSystem(opts.Browser, resolvedVersion)
	if !found {
		return "", nil, fmt.Errorf("finding executable for %s@%s", opts.Browser, resolvedVersion)
	}

	nativeMode := opts.NativeMode || isSystem

	var profileDir string
	if !nativeMode {
		profileOpts := opts
		profileOpts.Version = resolvedVersion
		profileDir = m.getProfileDir(profileOpts)
	}
	args, err := m.buildArgs(desc, opts, profileDir, nativeMode)
	if err != nil {
		return "", nil, err
	}

	return exePath, args, nil
}

// jsEscapeString escapes a string for safe embedding in a JavaScript
// double-quoted string literal (e.g. in Firefox user.js prefs).
func jsEscapeString(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\r", `\r`)
	return s
}

// isNumeric returns true if s consists only of ASCII digits.
func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// cleanupFirefoxStagedUpdates removes any staged Firefox update files that
// could cause the "we need to restart the browser" page to appear on every
// launch. On Linux the Firefox tarball ships with the `updater` binary, and
// if a past launch downloaded a MAR update it remains staged in the install
// directory. Firefox applies staged updates at startup regardless of the
// app.update.enabled preference.
func cleanupFirefoxStagedUpdates(exePath, profileDir string) {
	// Firefox install directory is the parent of the firefox script/binary.
	ffDir := filepath.Dir(exePath)

	// Remove the updates directory (contains staged MAR files such as
	// updates/0/update.mar and updates/0/update.status).
	updatesDir := filepath.Join(ffDir, "updates")
	if _, err := os.Stat(updatesDir); err == nil {
		if err := os.RemoveAll(updatesDir); err != nil {
			log.Warn("清理 Firefox 更新目录失败: %v", err)
		} else {
			log.Debug("已清理 Firefox staged 更新目录: %s", updatesDir)
		}
	}

	// Also remove active-update.xml in the profile directory. This file
	// tracks an in-progress update and can cause Firefox to show the
	// restart page even when the staged update was already applied.
	if profileDir != "" {
		activeUpdate := filepath.Join(profileDir, "active-update.xml")
		if _, err := os.Stat(activeUpdate); err == nil {
			if err := os.Remove(activeUpdate); err != nil {
				log.Warn("清理 active-update.xml 失败: %v", err)
			} else {
				log.Debug("已清理 active-update.xml: %s", activeUpdate)
			}
		}
	}
}

// applyFirefoxRuntimeEnv inspects the host environment for Firefox launch.
// On Linux, the most common cause of "tab crashed" pages at startup is the
// absence of unprivileged user namespaces, which Firefox needs to sandbox
// each tab. Ubuntu 24.04+ ships with kernel.unprivileged_userns_clone=0 by
// default. When that's the case we transparently set MOZ_DISABLE_SANDBOX=1
// so Firefox can still launch (without a sandbox), instead of crashing the
// content processes. The user is warned so they understand the trade-off.
func applyFirefoxRuntimeEnv(cmd *exec.Cmd, exePath string) {
	if runtime.GOOS != "linux" {
		return
	}

	// Detect unprivileged user namespace availability.
	if !userNamespaceAvailable() {
		appendEnv(cmd, "MOZ_DISABLE_SANDBOX", "1")
		log.Warn("检测到当前 Linux 内核禁用了 unprivileged user namespaces，" +
			"已自动为 Firefox 设置 MOZ_DISABLE_SANDBOX=1（关闭沙箱以避免标签页崩溃）。" +
			"如需恢复沙箱，请执行：sudo sysctl kernel.unprivileged_userns_clone=1")
	}

	// Detect /dev/shm size. Firefox needs ~1GB; otherwise it falls back to
	// slower IPC and may crash content processes on busy systems.
	if shmBytes, ok := devShmSize(); ok && shmBytes < 1<<30 {
		log.Warn("/dev/shm 容量小于 1GB（%s），Firefox 可能会出现性能问题或标签页崩溃。"+
			"可通过 `sudo mount -o remount,size=2g /dev/shm` 调整。",
			humanBytes(shmBytes))
	}
}

// userNamespaceAvailable reports whether the Linux kernel permits
// unprivileged user namespaces, which Firefox uses to sandbox each tab.
// Returns true on non-Linux, when the sysctl file is missing, or when
// the value is 1.
func userNamespaceAvailable() bool {
	data, err := os.ReadFile("/proc/sys/kernel/unprivileged_userns_clone")
	if err != nil {
		// File missing (older kernels or non-Linux) - assume available
		// and let Firefox attempt the sandbox.
		return true
	}
	return strings.TrimSpace(string(data)) == "1"
}

// devShmSize returns the size of /dev/shm in bytes by parsing the mountinfo
// line. Returns false if /dev/shm cannot be located.
func devShmSize() (int64, bool) {
	data, err := os.ReadFile("/proc/self/mountinfo")
	if err != nil {
		return 0, false
	}
	for _, line := range strings.Split(string(data), "\n") {
		// Format: mount-id parent-id major:minor root mountpoint opts ...
		fields := strings.Fields(line)
		if len(fields) < 10 {
			continue
		}
		mountpoint := fields[4]
		if mountpoint != "/dev/shm" {
			continue
		}
		// Optional fields start at index 8 (after "-" separator at index 7).
		var optStart, optEnd int
		for i, f := range fields {
			if f == "-" {
				optStart = i + 1
				break
			}
		}
		if optStart == 0 || optStart >= len(fields) {
			continue
		}
		optEnd = len(fields)
		for _, f := range fields[optStart:] {
			if strings.HasPrefix(f, "size=") {
				sizeStr := strings.TrimPrefix(f, "size=")
				n, err := strconv.ParseInt(sizeStr, 10, 64)
				if err != nil {
					return 0, false
				}
				return n, true
			}
		}
		_ = optEnd
	}
	return 0, false
}

// appendEnv adds a key=value entry to cmd.Env, replacing any existing entry
// for the same key. cmd.Env is expected to be non-nil.
func appendEnv(cmd *exec.Cmd, key, value string) {
	prefix := key + "="
	for i, kv := range cmd.Env {
		if strings.HasPrefix(kv, prefix) {
			cmd.Env[i] = prefix + value
			return
		}
	}
	cmd.Env = append(cmd.Env, prefix+value)
}

// humanBytes formats a byte count as a human-readable string.
func humanBytes(n int64) string {
	const (
		KB = 1 << 10
		MB = 1 << 20
		GB = 1 << 30
	)
	switch {
	case n >= GB:
		return fmt.Sprintf("%.1f GB", float64(n)/float64(GB))
	case n >= MB:
		return fmt.Sprintf("%.1f MB", float64(n)/float64(MB))
	case n >= KB:
		return fmt.Sprintf("%.1f KB", float64(n)/float64(KB))
	default:
		return fmt.Sprintf("%d B", n)
	}
}
