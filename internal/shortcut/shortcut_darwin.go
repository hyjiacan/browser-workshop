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

// escapeAppleScriptString escapes a string for safe embedding in an AppleScript
// double-quoted string literal. AppleScript uses backslash as the escape
// character inside double quotes.
func escapeAppleScriptString(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	return s
}

// escapeBashSingleQuote escapes a string for safe use in a bash single-quoted
// context. Single quotes in bash prevent all interpretation except the closing
// quote itself, which we handle with the standard '\'' trick.
func escapeBashSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
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
// All user-controlled values are properly escaped using escapeAppleScriptString
// to prevent AppleScript injection.
func buildAppleScript(cmdParts []string, workingDir string) string {
	var b strings.Builder
	b.WriteString(`on run
`)

	// Build the shell command with proper quoting using AppleScript's
	// "quoted form of" operator, which safely quotes for shell use.
	// The string literal itself is escaped to prevent breaking out of
	// the AppleScript string context.
	var quotedParts []string
	for _, part := range cmdParts {
		quotedParts = append(quotedParts, fmt.Sprintf(`quoted form of "%s"`, escapeAppleScriptString(part)))
	}

	if workingDir != "" {
		b.WriteString(fmt.Sprintf(`	do shell script "cd " & quoted form of "%s" & " && " & `, escapeAppleScriptString(workingDir)))
	} else {
		b.WriteString(`	do shell script `)
	}

	b.WriteString(strings.Join(quotedParts, ` & " " & `))
	b.WriteString(`
end run`)

	return b.String()
}

// createCommandScript creates a .command file as a fallback.
// All values are escaped using bash single-quote escaping to prevent
// shell command injection.
func createCommandScript(desktopDir, name string, cmdParts []string, workingDir string) error {
	scriptPath := filepath.Join(desktopDir, name+".command")

	var b strings.Builder
	b.WriteString("#!/bin/bash\n")
	if workingDir != "" {
		b.WriteString("cd " + escapeBashSingleQuote(workingDir) + "\n")
	}
	for i, part := range cmdParts {
		if i > 0 {
			b.WriteString(" ")
		}
		b.WriteString(escapeBashSingleQuote(part))
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

	// Check existence first to distinguish "not found" from "removal failed".
	appExists := fileExists(appPath)
	cmdExists := fileExists(cmdPath)

	if !appExists && !cmdExists {
		return fmt.Errorf("快捷方式不存在: %s", name)
	}

	var errs []string

	// Try .app first
	if appExists {
		if err := os.RemoveAll(appPath); err != nil {
			errs = append(errs, fmt.Sprintf(".app: %v", err))
		}
	}
	// Try .command
	if cmdExists {
		if err := os.Remove(cmdPath); err != nil {
			errs = append(errs, fmt.Sprintf(".command: %v", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("删除快捷方式失败: %s", strings.Join(errs, "; "))
	}

	return nil
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
