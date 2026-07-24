package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/bws/bws/internal/i18n"
)

func TestNewApp(t *testing.T) {
	i18n.Init("zh", "")
	ctx := DefaultContext()
	app := NewApp("bws", "1.0.0", ctx)

	if app.Name != "bws" {
		t.Errorf("Name = %q", app.Name)
	}
	if app.Version != "1.0.0" {
		t.Errorf("Version = %q", app.Version)
	}
}

func TestExecute_NoArgs(t *testing.T) {
	i18n.Init("zh", "")
	var buf bytes.Buffer
	ctx := &Context{Stdout: &buf, Stderr: &buf}
	app := NewApp("bws", "1.0.0", ctx)

	err := app.Execute([]string{})
	if err != nil {
		t.Errorf("Execute with no args error = %v", err)
	}
	if buf.Len() == 0 {
		t.Error("no output for no args (expected help)")
	}
	if !strings.Contains(buf.String(), "浏览器版本管理工具") {
		t.Errorf("output doesn't contain description: %s", buf.String())
	}
}

func TestExecute_VersionFlag(t *testing.T) {
	tests := []string{"--version", "-v", "version"}

	for _, arg := range tests {
		t.Run(arg, func(t *testing.T) {
			var buf bytes.Buffer
			ctx := &Context{Stdout: &buf, Stderr: &buf}
			app := NewApp("bws", "1.2.3", ctx)

			err := app.Execute([]string{arg})
			if err != nil {
				t.Errorf("error = %v", err)
			}
			if !strings.Contains(buf.String(), "1.2.3") {
				t.Errorf("output doesn't contain version: %s", buf.String())
			}
		})
	}
}

func TestExecute_HelpFlag(t *testing.T) {
	tests := []string{"--help", "-h"}

	for _, arg := range tests {
		t.Run(arg, func(t *testing.T) {
			var buf bytes.Buffer
			ctx := &Context{Stdout: &buf, Stderr: &buf}
			app := NewApp("bws", "1.0.0", ctx)

			err := app.Execute([]string{arg})
			if err != nil {
				t.Errorf("error = %v", err)
			}
			if buf.Len() == 0 {
				t.Error("no output for help flag")
			}
		})
	}
}

func TestExecute_UnknownCommand(t *testing.T) {
	var buf bytes.Buffer
	ctx := &Context{Stdout: &buf, Stderr: &buf}
	app := NewApp("bws", "1.0.0", ctx)

	err := app.Execute([]string{"nonexistent"})
	if err == nil {
		t.Error("expected error for unknown command")
	}
}

func TestAddAndExecuteCommand(t *testing.T) {
	var buf bytes.Buffer
	ctx := &Context{Stdout: &buf, Stderr: &buf}
	app := NewApp("bws", "1.0.0", ctx)

	called := false
	cmd := &Command{
		Name:        "test",
		Description: "a test command",
		Run: func(ctx *Context, args []string) error {
			called = true
			ctx.Printf("hello %s", strings.Join(args, " "))
			return nil
		},
	}

	app.AddCommand(cmd)

	err := app.Execute([]string{"test", "world"})
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if !called {
		t.Error("command was not called")
	}
	if !strings.Contains(buf.String(), "hello world") {
		t.Errorf("output = %q, want 'hello world'", buf.String())
	}
}

func TestCommandAliases(t *testing.T) {
	var buf bytes.Buffer
	ctx := &Context{Stdout: &buf, Stderr: &buf}
	app := NewApp("bws", "1.0.0", ctx)

	called := false
	cmd := &Command{
		Name:        "list",
		Aliases:     []string{"ls"},
		Description: "list versions",
		Run: func(ctx *Context, args []string) error {
			called = true
			return nil
		},
	}

	app.AddCommand(cmd)

	// Should work with alias
	err := app.Execute([]string{"ls"})
	if err != nil {
		t.Fatalf("alias error = %v", err)
	}
	if !called {
		t.Error("command not called via alias")
	}
}

func TestSubcommands(t *testing.T) {
	var buf bytes.Buffer
	ctx := &Context{Stdout: &buf, Stderr: &buf}
	app := NewApp("bws", "1.0.0", ctx)

	subCalled := false
	parent := &Command{
		Name:        "repo",
		Description: "manage repositories",
	}
	sub := &Command{
		Name:        "list",
		Description: "list repos",
		Run: func(ctx *Context, args []string) error {
			subCalled = true
			ctx.Println("repo list")
			return nil
		},
	}
	parent.SubCommands = append(parent.SubCommands, sub)
	app.AddCommand(parent)

	err := app.Execute([]string{"repo", "list"})
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if !subCalled {
		t.Error("subcommand not called")
	}
	if !strings.Contains(buf.String(), "repo list") {
		t.Errorf("output = %q", buf.String())
	}
}

func TestParseFlags(t *testing.T) {
	flags := []*Flag{
		{Name: "verbose", Short: "v", Usage: "verbose output", HasValue: false, Default: "false"},
		{Name: "output", Short: "o", Usage: "output file", HasValue: true, Default: ""},
		{Name: "count", Usage: "count", HasValue: true, Default: "10"},
	}

	t.Run("long flag with value", func(t *testing.T) {
		vals, pos, _ := ParseFlags([]string{"--output", "file.txt", "arg1"}, flags)
		if vals["output"] != "file.txt" {
			t.Errorf("output = %q", vals["output"])
		}
		if len(pos) != 1 || pos[0] != "arg1" {
			t.Errorf("positional args = %v", pos)
		}
	})

	t.Run("long flag with =", func(t *testing.T) {
		vals, _, _ := ParseFlags([]string{"--output=file.txt"}, flags)
		if vals["output"] != "file.txt" {
			t.Errorf("output = %q", vals["output"])
		}
	})

	t.Run("boolean flag", func(t *testing.T) {
		vals, _, _ := ParseFlags([]string{"--verbose"}, flags)
		if vals["verbose"] != "true" {
			t.Errorf("verbose = %q", vals["verbose"])
		}
	})

	t.Run("short flag", func(t *testing.T) {
		vals, _, _ := ParseFlags([]string{"-v"}, flags)
		if vals["verbose"] != "true" {
			t.Errorf("verbose via short = %q", vals["verbose"])
		}
	})

	t.Run("short flag with value", func(t *testing.T) {
		vals, _, _ := ParseFlags([]string{"-o", "out.txt"}, flags)
		if vals["output"] != "out.txt" {
			t.Errorf("output via short = %q", vals["output"])
		}
	})

	t.Run("default values", func(t *testing.T) {
		vals, _, _ := ParseFlags([]string{}, flags)
		if vals["count"] != "10" {
			t.Errorf("count default = %q, want '10'", vals["count"])
		}
		if vals["verbose"] != "false" {
			t.Errorf("verbose default = %q, want 'false'", vals["verbose"])
		}
	})

	t.Run("mixed flags and args", func(t *testing.T) {
		vals, pos, _ := ParseFlags([]string{"arg1", "--verbose", "arg2", "--count=5", "arg3"}, flags)
		if vals["verbose"] != "true" {
			t.Errorf("verbose = %q", vals["verbose"])
		}
		if vals["count"] != "5" {
			t.Errorf("count = %q", vals["count"])
		}
		if len(pos) != 3 {
			t.Errorf("positional count = %d, want 3: %v", len(pos), pos)
		}
	})
}

func TestPrintTable(t *testing.T) {
	var buf bytes.Buffer
	headers := []string{"Name", "Version", "Size"}
	rows := [][]string{
		{"Chrome", "120.0.6099.109", "842 MB"},
		{"Firefox", "121.0", "67 MB"},
	}

	PrintTable(&buf, headers, rows)
	output := buf.String()

	if !strings.Contains(output, "Name") {
		t.Errorf("output missing header: %s", output)
	}
	if !strings.Contains(output, "Chrome") {
		t.Errorf("output missing row: %s", output)
	}
	if !strings.Contains(output, "---") {
		t.Errorf("output missing separator: %s", output)
	}
}

func TestFormatSize(t *testing.T) {
	tests := []struct {
		bytes    int64
		expected string
	}{
		{0, "0 B"},
		{500, "500 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := FormatSize(tt.bytes)
			if result != tt.expected {
				t.Errorf("FormatSize(%d) = %q, want %q", tt.bytes, result, tt.expected)
			}
		})
	}
}

func TestCommandHelp(t *testing.T) {
	i18n.Init("zh", "")
	var buf bytes.Buffer
	ctx := &Context{Stdout: &buf, Stderr: &buf}
	app := NewApp("bws", "1.0.0", ctx)

	cmd := &Command{
		Name:        "install",
		Description: "install a browser version",
		Usage:       "bws install <browser@version> [options]",
		Examples:    []string{"install chrome@120", "install firefox@latest"},
		Flags: []*Flag{
			{Name: "force", Short: "f", Usage: "force reinstall", HasValue: false, Default: "false"},
		},
		Run: func(ctx *Context, args []string) error { return nil },
	}

	app.AddCommand(cmd)

	err := app.Execute([]string{"install", "--help"})
	if err != nil {
		t.Fatalf("error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "install a browser version") {
		t.Errorf("help missing description: %s", output)
	}
	if !strings.Contains(output, "bws install <browser@version>") {
		t.Errorf("help missing usage: %s", output)
	}
	if !strings.Contains(output, "示例:") {
		t.Errorf("help missing examples: %s", output)
	}
	if !strings.Contains(output, "--force") {
		t.Errorf("help missing flag: %s", output)
	}
}

func TestContextPrintf(t *testing.T) {
	var buf bytes.Buffer
	ctx := &Context{Stdout: &buf}

	ctx.Printf("hello %s", "world")
	if buf.String() != "hello world" {
		t.Errorf("Printf output = %q", buf.String())
	}
}

func TestContextPrintln(t *testing.T) {
	var buf bytes.Buffer
	ctx := &Context{Stdout: &buf}

	ctx.Println("hello")
	if buf.String() != "hello\n" {
		t.Errorf("Println output = %q", buf.String())
	}
}

func TestContextErrorf(t *testing.T) {
	var buf bytes.Buffer
	ctx := &Context{Stderr: &buf}

	ctx.Errorf("error: %s", "bad")
	if buf.String() != "error: bad" {
		t.Errorf("Errorf output = %q", buf.String())
	}
}

func TestDefaultContext(t *testing.T) {
	ctx := DefaultContext()
	if ctx.Stdout == nil {
		t.Error("Stdout is nil")
	}
	if ctx.Stderr == nil {
		t.Error("Stderr is nil")
	}
	if ctx.Stdin == nil {
		t.Error("Stdin is nil")
	}
}
