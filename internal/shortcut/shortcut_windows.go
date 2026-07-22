//go:build windows

package shortcut

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// defaultDesktopDir returns the user's desktop directory on Windows.
func defaultDesktopDir() string {
	if home := os.Getenv("USERPROFILE"); home != "" {
		return filepath.Join(home, "Desktop")
	}
	return filepath.Join(os.Getenv("HOMEDRIVE")+os.Getenv("HOMEPATH"), "Desktop")
}

// createShortcut creates a .lnk file on Windows using PowerShell.
func createShortcut(desktopDir string, opts Options) error {
	name := sanitizeName(opts.Name)
	shortcutPath := filepath.Join(desktopDir, name+".lnk")

	// Build PowerShell script
	script := fmt.Sprintf(
		"$ws = New-Object -ComObject WScript.Shell; "+
			"$s = $ws.CreateShortcut(%q); "+
			"$s.TargetPath = %q; "+
			"$s.Arguments = %q; "+
			"$s.WorkingDirectory = %q; "+
			"$s.Save();",
		shortcutPath,
		opts.Target,
		strings.Join(opts.Args, " "),
		opts.WorkingDir,
	)

	if opts.IconPath != "" {
		script += fmt.Sprintf(" $s.IconLocation = %q; $s.Save();", opts.IconPath)
	}

	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("创建快捷方式失败: %w", err)
	}

	return nil
}

// removeShortcut removes a .lnk file on Windows.
func removeShortcut(desktopDir string, name string) error {
	name = sanitizeName(name)
	shortcutPath := filepath.Join(desktopDir, name+".lnk")
	if err := os.Remove(shortcutPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("快捷方式不存在: %s", name)
		}
		return fmt.Errorf("删除快捷方式失败: %w", err)
	}
	return nil
}

// listShortcuts returns all .lnk files in the desktop directory.
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
		if strings.HasSuffix(strings.ToLower(entry.Name()), ".lnk") {
			names = append(names, strings.TrimSuffix(entry.Name(), ".lnk"))
		}
	}
	return names, nil
}
