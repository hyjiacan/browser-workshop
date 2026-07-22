// Package shortcut provides cross-platform desktop shortcut creation.
// Supports Windows (.lnk), Linux (.desktop), and macOS (alias/.app).
package shortcut

import (
	"fmt"
	"os"
	"strings"
)

// Options configures a shortcut to be created.
type Options struct {
	// Name is the display name of the shortcut (e.g. "Google Chrome 120").
	Name string

	// Target is the absolute path to the browser executable.
	Target string

	// Args are the command-line arguments to pass to the browser.
	Args []string

	// WorkingDir is the working directory for the shortcut.
	WorkingDir string

	// IconPath is an optional path to an icon file.
	IconPath string

	// DesktopDir is an optional override for the desktop directory.
	// If empty, the platform default is used.
	DesktopDir string
}

// Manager handles shortcut creation and removal.
type Manager struct{}

// NewManager creates a new shortcut manager.
func NewManager() *Manager {
	return &Manager{}
}

// Create creates a desktop shortcut for the given options.
func (m *Manager) Create(opts Options) error {
	if opts.Target == "" {
		return fmt.Errorf("快捷方式目标不能为空")
	}
	if opts.Name == "" {
		return fmt.Errorf("快捷方式名称不能为空")
	}

	desktopDir := opts.DesktopDir
	if desktopDir == "" {
		desktopDir = defaultDesktopDir()
	}

	if err := os.MkdirAll(desktopDir, 0o755); err != nil {
		return fmt.Errorf("创建桌面目录失败: %w", err)
	}

	return createShortcut(desktopDir, opts)
}

// Remove removes a desktop shortcut by name.
func (m *Manager) Remove(name string, desktopDir string) error {
	if desktopDir == "" {
		desktopDir = defaultDesktopDir()
	}
	return removeShortcut(desktopDir, name)
}

// List returns the names of all shortcuts created by bws in the desktop directory.
func (m *Manager) List(desktopDir string) ([]string, error) {
	if desktopDir == "" {
		desktopDir = defaultDesktopDir()
	}
	return listShortcuts(desktopDir)
}

// sanitizeName removes characters that are unsafe for filenames.
func sanitizeName(name string) string {
	// Remove path separators and other unsafe chars
	replacer := strings.NewReplacer(
		"/", "-",
		"\\", "-",
		":", "-",
		"*", "-",
		"?", "-",
		"\"", "-",
		"<", "-",
		">", "-",
		"|", "-",
	)
	return strings.TrimSpace(replacer.Replace(name))
}

// escapeArg escapes a single argument for use in shell commands.
func escapeArg(arg string) string {
	if strings.ContainsAny(arg, " \\t\n\r\"") {
		return fmt.Sprintf("%q", arg)
	}
	return arg
}
