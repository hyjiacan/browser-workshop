//go:build !windows && !darwin

package shortcut

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// defaultDesktopDir returns the user's desktop directory on Linux.
func defaultDesktopDir() string {
	if home := os.Getenv("HOME"); home != "" {
		return filepath.Join(home, "Desktop")
	}
	return "/tmp"
}

// createShortcut creates a .desktop file on Linux.
func createShortcut(desktopDir string, opts Options) error {
	name := sanitizeName(opts.Name)
	// Write to both desktop and applications dir for better integration
	appDir := filepath.Join(os.Getenv("HOME"), ".local", "share", "applications")
	_ = os.MkdirAll(appDir, 0o755)

	content := buildDesktopEntry(name, opts)

	// Write to desktop
	desktopPath := filepath.Join(desktopDir, name+".desktop")
	if err := os.WriteFile(desktopPath, []byte(content), 0o755); err != nil {
		return fmt.Errorf("写入桌面快捷方式失败: %w", err)
	}

	// Also write to applications dir
	appPath := filepath.Join(appDir, "bws-"+name+".desktop")
	if err := os.WriteFile(appPath, []byte(content), 0o755); err != nil {
		return fmt.Errorf("写入应用菜单项失败: %w", err)
	}

	return nil
}

func buildDesktopEntry(name string, opts Options) string {
	execLine := escapeArg(opts.Target)
	for _, arg := range opts.Args {
		execLine += " " + escapeArg(arg)
	}

	var b strings.Builder
	b.WriteString("[Desktop Entry]\n")
	b.WriteString("Name=" + name + "\n")
	b.WriteString("Comment=Browser launched by BrowserWorkshop\n")
	b.WriteString("Exec=" + execLine + "\n")
	b.WriteString("Type=Application\n")
	b.WriteString("Terminal=false\n")
	if opts.IconPath != "" {
		b.WriteString("Icon=" + opts.IconPath + "\n")
	}
	if opts.WorkingDir != "" {
		b.WriteString("Path=" + opts.WorkingDir + "\n")
	}
	return b.String()
}

// removeShortcut removes a .desktop file on Linux.
func removeShortcut(desktopDir string, name string) error {
	name = sanitizeName(name)
	var errs []string

	// Remove from desktop
	desktopPath := filepath.Join(desktopDir, name+".desktop")
	if err := os.Remove(desktopPath); err != nil && !os.IsNotExist(err) {
		errs = append(errs, fmt.Sprintf("桌面: %v", err))
	}

	// Remove from applications dir
	appDir := filepath.Join(os.Getenv("HOME"), ".local", "share", "applications")
	appPath := filepath.Join(appDir, "bws-"+name+".desktop")
	if err := os.Remove(appPath); err != nil && !os.IsNotExist(err) {
		errs = append(errs, fmt.Sprintf("应用菜单: %v", err))
	}

	if len(errs) > 0 {
		return fmt.Errorf("删除快捷方式失败: %s", strings.Join(errs, "; "))
	}

	// Check if at least one file existed
	if _, err := os.Stat(desktopPath); err == nil {
		return nil // desktop file still exists but we didn't get an error (shouldn't happen)
	}
	if _, err := os.Stat(appPath); err == nil {
		return nil
	}
	return fmt.Errorf("快捷方式不存在: %s", name)
}

// listShortcuts returns all .desktop files in the desktop directory.
func listShortcuts(desktopDir string) ([]string, error) {
	entries, err := os.ReadDir(desktopDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.HasSuffix(strings.ToLower(entry.Name()), ".desktop") {
			names = append(names, strings.TrimSuffix(entry.Name(), ".desktop"))
		}
	}
	return names, nil
}
