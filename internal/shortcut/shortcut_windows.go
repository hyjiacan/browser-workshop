//go:build windows

package shortcut

import (
	"encoding/base64"
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

// psEscape escapes a string for safe embedding in a PowerShell double-quoted
// string. PowerShell uses backtick (`) as the escape character inside double
// quotes, not backslash. We escape characters that could break out of the
// string context.
func psEscape(s string) string {
	s = strings.ReplaceAll(s, "`", "``")
	s = strings.ReplaceAll(s, "\"", "`\"")
	s = strings.ReplaceAll(s, "$", "`$")
	return s
}

// createShortcut creates a .lnk file on Windows using PowerShell.
// The PowerShell script is passed via -EncodedCommand (Base64 UTF-16LE)
// to prevent any command injection through user-controlled paths.
func createShortcut(desktopDir string, opts Options) error {
	name := sanitizeName(opts.Name)
	shortcatPath := filepath.Join(desktopDir, name+".lnk")

	// Build PowerShell script with properly escaped values
	script := fmt.Sprintf(
		"$ws = New-Object -ComObject WScript.Shell; "+
			"$s = $ws.CreateShortcut(\"%s\"); "+
			"$s.TargetPath = \"%s\"; "+
			"$s.Arguments = \"%s\"; "+
			"$s.WorkingDirectory = \"%s\"; "+
			"$s.Save();",
		psEscape(shortcatPath),
		psEscape(opts.Target),
		psEscape(strings.Join(opts.Args, " ")),
		psEscape(opts.WorkingDir),
	)

	if opts.IconPath != "" {
		script += fmt.Sprintf(" $s.IconLocation = \"%s\"; $s.Save();", psEscape(opts.IconPath))
	}

	// Encode as UTF-16LE Base64 for -EncodedCommand to prevent injection
	encoded := base64.StdEncoding.EncodeToString([]byte(script))

	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-EncodedCommand", encoded)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("创建快捷方式失败: %w", err)
	}

	return nil
}

// removeShortcut removes a .lnk file on Windows.
func removeShortcut(desktopDir string, name string) error {
	name = sanitizeName(name)
	shortcatPath := filepath.Join(desktopDir, name+".lnk")
	if err := os.Remove(shortcatPath); err != nil {
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
