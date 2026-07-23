// Package launch handles launching browser versions with proper isolation.
package launch

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"

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

	// Print matching versions to stdout (user-visible output)
	if len(matches) == 1 {
		fmt.Fprintf(os.Stdout, "使用 %s@%s\n", opts.Browser, matches[0].Version)
	} else {
		fmt.Fprintf(os.Stdout, "%s@%s 的匹配版本:\n", opts.Browser, opts.Version)
		for i, v := range matches {
			prefix := "  "
			if i == 0 {
				prefix = "> "
			}
			fmt.Fprintf(os.Stdout, "%s%s\n", prefix, v.Version)
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
	}

	// Build arguments
	args := m.buildArgs(desc, opts, profileDir, nativeMode)

	// Build command
	cmd := exec.Command(exePath, args...)

	if opts.WorkingDir != "" {
		cmd.Dir = opts.WorkingDir
	}

	// Set environment
	if len(opts.Env) > 0 {
		cmd.Env = os.Environ()
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
func (m *Manager) buildArgs(desc *browser.BrowserDescriptor, opts Options, profileDir string, nativeMode bool) []string {
	var args []string

	if !nativeMode {
		// Standard args (multi-instance, no-update, first-run skip)
		args = append(args, desc.BuildStandardArgs()...)

		// Profile directory
		if desc.Features.SupportsProfile {
			args = append(args, desc.BuildProfileArgs(profileDir)...)
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
		proxyArgs := buildProxyArgs(desc, opts.Proxy, profileDir)
		args = append(args, proxyArgs...)
	}

	// Fingerprint isolation
	if opts.Fingerprint != nil && !opts.Fingerprint.IsEmpty() {
		fpArgs := buildFingerprintArgs(desc, opts.Fingerprint, profileDir)
		args = append(args, fpArgs...)
	}

	// URLs to open
	for _, url := range opts.URLs {
		args = append(args, url)
	}

	// Extra args (last, so they can override)
	args = append(args, opts.ExtraArgs...)

	return args
}

// buildProxyArgs constructs proxy-related arguments for the browser.
// Chrome/Chromium uses --proxy-server command-line flag.
// Firefox requires a user.js file in the profile directory (handled separately).
func buildProxyArgs(desc *browser.BrowserDescriptor, proxyURL, profileDir string) []string {
	switch desc.Name {
	case "chrome", "chromium":
		return []string{"--proxy-server=" + proxyURL}
	case "firefox":
		// Firefox doesn't support command-line proxy.
		// Write user.js in profile dir if available.
		if profileDir != "" {
			writeFirefoxProxyPrefs(profileDir, proxyURL)
		}
		return nil
	default:
		return nil
	}
}

// writeFirefoxProxyPrefs writes proxy preferences to user.js in the profile directory.
func writeFirefoxProxyPrefs(profileDir, proxyURL string) {
	prefsPath := filepath.Join(profileDir, "user.js")

	parsed, err := url.Parse(proxyURL)
	if err != nil {
		return
	}

	host := parsed.Hostname()
	port := parsed.Port()
	if port == "" {
		switch parsed.Scheme {
		case "http", "https":
			port = "80"
		case "socks5", "socks5h":
			port = "1080"
		}
	}

	var content string
	if parsed.Scheme == "socks5" || parsed.Scheme == "socks5h" {
		content = fmt.Sprintf(`// Proxy settings written by bws
user_pref("network.proxy.type", 1);
user_pref("network.proxy.socks", "%s");
user_pref("network.proxy.socks_port", %s);
user_pref("network.proxy.socks_version", 5);
user_pref("network.proxy.socks_remote_dns", true);
`, host, port)
	} else {
		// HTTP/HTTPS proxy
		content = fmt.Sprintf(`// Proxy settings written by bws
user_pref("network.proxy.type", 1);
user_pref("network.proxy.http", "%s");
user_pref("network.proxy.http_port", %s);
user_pref("network.proxy.ssl", "%s");
user_pref("network.proxy.ssl_port", %s);
`, host, port, host, port)
	}

	_ = os.WriteFile(prefsPath, []byte(content), 0o644)
}

// buildFingerprintArgs constructs fingerprint-related arguments for the browser.
// Chrome: uses command-line flags (--user-agent, --lang, --window-size, etc.)
// Firefox: writes user.js preferences to the profile directory.
func buildFingerprintArgs(desc *browser.BrowserDescriptor, cfg *fingerprint.Config, profileDir string) []string {
	switch desc.Name {
	case "chrome", "chromium":
		return cfg.ChromeArgs()
	case "firefox":
		// Firefox: write user.js to profile directory
		if profileDir != "" {
			_ = cfg.WriteFirefoxUserJS(profileDir)
		}
		return nil
	default:
		return nil
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
	args := m.buildArgs(desc, opts, profileDir, nativeMode)

	return exePath, args, nil
}
