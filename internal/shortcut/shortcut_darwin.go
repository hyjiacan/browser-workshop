//go:build darwin

package shortcut

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// defaultDesktopDir returns the user's desktop directory on macOS.
func defaultDesktopDir() string {
	if home := os.Getenv("HOME"); home != "" {
		return filepath.Join(home, "Desktop")
	}
	return "/tmp"
}

// createShortcut creates an AppleScript .app bundle on macOS.
func createShortcut(desktopDir string, opts Options) error {
	name := sanitizeName(opts.Name)
	appPath := filepath.Join(desktopDir, name+".app")

	// Build the shell command that the AppleScript will execute
	cmdParts := []string{opts.Target}
	for _, arg := range opts.Args {
		cmdParts = append(cmdParts, arg)
	}

	// Build AppleScript source
	script := buildAppleScript(cmdParts, opts.WorkingDir)

	// Use osacompile to create the .app bundle
	cmd := exec.Command("osacompile", "-o", appPath, "-")
	cmd.Stdin = strings.NewReader(script)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		// Fallback: create a .command script if osacompile fails
		return createCommandScript(desktopDir, name, cmdParts, opts.WorkingDir)
	}

	return nil
}

// buildAppleScript builds an AppleScript that launches the browser.
func buildAppleScript(cmdParts []string, workingDir string) string {
	var b strings.Builder
	b.WriteString(`on run
`)

	// Build the shell command with proper quoting
	var quotedParts []string
	for _, part := range cmdParts {
		quotedParts = append(quotedParts, fmt.Sprintf("quoted form of %q", part))
	}

	if workingDir != "" {
		b.WriteString(fmt.Sprintf(`	do shell script "cd " & quoted form of %q & " && " & `, workingDir))
	} else {
		b.WriteString(`	do shell script `)
	}

	b.WriteString(strings.Join(quotedParts, ` & " " & `))
	b.WriteString(`
end run`)

	return b.String()
}

// createCommandScript creates a .command file as a fallback.
func createCommandScript(desktopDir, name string, cmdParts []string, workingDir string) error {
	scriptPath := filepath.Join(desktopDir, name+".command")

	var b strings.Builder
	b.WriteString("#!/bin/bash\n")
	if workingDir != "" {
		b.WriteString(fmt.Sprintf("cd %q\n", workingDir))
	}
	for i, part := range cmdParts {
		if i > 0 {
			b.WriteString(" ")
		}
		b.WriteString(fmt.Sprintf("%q", part))
	}
	b.WriteString("\n")

	if err := os.WriteFile(scriptPath, []byte(b.String()), 0o755); err != nil {
		return fmt.Errorf("创建快捷方式失败: %w", err)
	}
	return nil
}

// removeShortcut removes a .app bundle or .command file on macOS.
func removeShortcut(desktopDir string, name string) error {
	name = sanitizeName(name)
	appPath := filepath.Join(desktopDir, name+".app")
	cmdPath := filepath.Join(desktopDir, name+".command")

	// Try .app first
	if err := os.RemoveAll(appPath); err == nil {
		return nil
	}
	// Try .command
	if err := os.Remove(cmdPath); err == nil {
		return nil
	}

	// Check existence
	if _, err := os.Stat(appPath); os.IsNotExist(err) {
		if _, err2 := os.Stat(cmdPath); os.IsNotExist(err2) {
			return fmt.Errorf("快捷方式不存在: %s", name)
		}
	}
	return fmt.Errorf("删除快捷方式失败")
}

// listShortcuts returns all .app bundles and .command files in the desktop directory.
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
			// .app bundles are directories
			if strings.HasSuffix(strings.ToLower(entry.Name()), ".app") {
				names = append(names, strings.TrimSuffix(entry.Name(), ".app"))
			}
			continue
		}
		if strings.HasSuffix(strings.ToLower(entry.Name()), ".command") {
			names = append(names, strings.TrimSuffix(entry.Name(), ".command"))
		}
	}
	return names, nil
}
