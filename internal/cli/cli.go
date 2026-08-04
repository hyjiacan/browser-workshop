// Package cli provides the command-line interface for bm.
package cli

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"
	"unicode"

	"github.com/bws/bws/internal/i18n"
	"github.com/bws/bws/internal/install"
	"github.com/bws/bws/internal/plugin"
	"github.com/bws/bws/internal/repo"
	"github.com/bws/bws/internal/shortcut"
	"github.com/bws/bws/internal/source"
	"github.com/bws/bws/internal/version"
)

// Command represents a CLI command.
type Command struct {
	Name        string
	Aliases     []string
	Description string
	Usage       string
	Examples    []string
	Run         func(ctx *Context, args []string) error
	SubCommands []*Command
	Flags       []*Flag
}

// Flag represents a command-line flag.
type Flag struct {
	Name     string
	Short    string // single character
	Usage    string
	HasValue bool
	Default  string
}

// Context holds shared state for CLI commands.
type Context struct {
	Stdout   io.Writer
	Stderr   io.Writer
	Stdin    io.Reader
	Paths    PathsProvider
	Cfg      *Settings // role-based config interfaces
	Browsers BrowserProvider
	Install  InstallProvider
	Profile  ProfileProvider
	Launch   LaunchProvider
	Repo     RepoProvider
	Download DownloadProvider
	Source   SourceProvider
	Shortcut ShortcutProvider
	Logger   Logger
	Serve    ServeProvider
	Plugin   PluginProvider
}

// Confirm asks the user for confirmation and returns true if they agree.
// It writes the prompt to Stderr and reads from Stdin.
func (ctx *Context) Confirm(prompt string) bool {
	fmt.Fprint(ctx.Stderr, i18n.Tfmt("confirm.prompt", prompt))
	var response string
	fmt.Fscanln(ctx.Stdin, &response)
	response = strings.ToLower(strings.TrimSpace(response))
	return response == "y" || response == "yes" || response == "是"
}

// ServeProvider provides the HTTP serve functionality.
type ServeProvider interface {
	// StartFromConfig starts the HTTP server using configuration from bws-serve.ini
	// in the executable directory.
	// baseDir overrides the base directory (containing packages/ and bin/) if non-empty.
	StartFromConfig(baseDir string) error

	// ConfigPath returns the path to the serve config file.
	ConfigPath() string

	// EnsureDefaultConfig creates a default bws-serve.ini if it doesn't exist.
	// baseDir can be empty to use the executable directory.
	// Returns the config path and whether it was newly created.
	EnsureDefaultConfig(baseDir string) (string, bool, error)
}

// Logger is a minimal logging interface for CLI commands.
type Logger interface {
	Info(msg string, args ...interface{})
	Debug(msg string, args ...interface{})
	Warn(msg string, args ...interface{})
	Error(msg string, args ...interface{})
}

// RepoProvider provides repository scanning and importing.
type RepoProvider interface {
	Scan() ([]repo.MatchResult, error)
	Import(force bool, onProgress func(int, int, string)) (*repo.ImportSummary, error)
}

// PathsProvider provides path management.
type PathsProvider interface {
	VersionDir(browser string, version string) string
	DownloadCacheDir() string
	EnsureAll() error
}

// PluginProvider manages plugins.
type PluginProvider interface {
	List() []plugin.ManifestEntry
	GetManifestEntry(name string) (*plugin.ManifestEntry, error)
	Install(entry plugin.ManifestEntry) error
	Uninstall(name string) error
	PluginsDir() string
}

// ---- Role-based config interfaces (ISP-compliant) ----

// AliasManager manages browser version aliases.
type AliasManager interface {
	GetAlias(name string) (string, bool)
	AddAlias(name, target string) error
	RemoveAlias(name string) error
	ListAliases() map[string]string
}

// RepoSettings manages the local binary repository path.
type RepoSettings interface {
	GetRepoPath() string
	SetRepoPath(path string) error
}

// DefaultSettings manages default browser and channel.
type DefaultSettings interface {
	DefaultBrowser() string
	SetDefaultBrowser(browser string) error
	DefaultVersion() string
	SetDefaultVersion(version string) error
	DefaultChannel() string
	SetDefaultChannel(channel string) error
}

// AppDataConfig manages data directory and config path.
type AppDataConfig interface {
	GetDataDir() string
	SetDataDir(path string) error
	ConfigPath() string
}

// ProxySettings manages proxy configuration.
type ProxySettings interface {
	GetProxy() string
	SetProxy(proxy string) error
}

// SourceSettings manages remote source URL and source-type toggles.
type SourceSettings interface {
	GetRemoteSource() string
	SetRemoteSource(url string) error
	ClearRemoteSource() error
	IsServeSourceEnabled() bool
	SetServeSourceEnabled(v bool) error
	IsFirefoxFTPEnabled() bool
	SetFirefoxFTPEnabled(v bool) error
}

// DiskSpaceSettings manages the free-disk-space threshold (GB).
type DiskSpaceSettings interface {
	GetDiskSpaceThresholdGB() int
	SetDiskSpaceThresholdGB(v int) error
}

// LogSettings manages log-level configuration.
type LogSettings interface {
	GetLogLevel() string
	SetLogLevel(level string) error
}

// LangSettings manages the UI language.
type LangSettings interface {
	GetLanguage() string
	SetLanguage(lang string) error
}

// Settings groups the fine-grained config interfaces together.
// A single ConfigProvider implementation can satisfy every field,
// or individual mocks/stubs can be injected independently in tests.
type Settings struct {
	Aliases  AliasManager
	Repo     RepoSettings
	Defaults DefaultSettings
	Data     AppDataConfig
	Proxy    ProxySettings
	Source   SourceSettings
	Disk     DiskSpaceSettings
	Log      LogSettings
	Language LangSettings
}

// ConfigProvider is the full configuration interface, composing all
// role-based interfaces. Prefer the smaller interfaces when a command
// only needs a subset of configuration — use ctx.Cfg.* instead.
type ConfigProvider interface {
	AliasManager
	RepoSettings
	DefaultSettings
	AppDataConfig
	ProxySettings
	SourceSettings
	DiskSpaceSettings
	LogSettings
	LangSettings
}

// BrowserProvider provides browser descriptors.
type BrowserProvider interface {
	Get(name string) BrowserDescriptor
	List() []BrowserDescriptor
	Has(name string) bool
	ResolveName(name string) (string, bool)
}

// BrowserDescriptor is a simplified view of browser.BrowserDescriptor for CLI.
type BrowserDescriptor struct {
	Name        string
	DisplayName string
}

// InstallProvider provides installation management.
type InstallProvider interface {
	IsInstalled(browser, version string) bool
	ListInstalled() (version.List, error)
	ListInstalledByBrowser(browser string) (version.List, error)
	GetRecord(browser, version string) (*version.InstallRecord, error)
	Uninstall(browser, version string) error
	// Install from a local directory
	InstallFromDir(browser, version, sourceDir string) (*version.InstallRecord, error)
	// Install from a local archive file
	InstallFromFile(browser, version, filePath string) (*version.InstallRecord, error)
	// System browser support
	HasSystem() bool
	ListWithSystem() (version.List, error)
	ListWithSystemByBrowser(browser string) (version.List, error)
	IsSystemVersion(browser, version string) bool
	// ResolveInstalledVersion resolves a version spec (exact/partial/alias) to a full installed version.
	// Supports "126" → "126.0.6478.115", "latest", "system", exact match.
	ResolveInstalledVersion(browser, version string) (string, error)
	// ImportFromDir scans a directory and imports all recognized browser versions.
	// The onProgress callback is called for each item being processed (item index, total, message).
	ImportFromDir(dir string, force bool, onProgress func(current int, total int, message string)) (*ImportSummary, error)
}

// ImportSummary summarizes the result of a batch import operation.
type ImportSummary struct {
	Total                  int
	Success                int
	Failed                 int
	Skipped                int
	SkippedIncompatible    int
	SkippedAlreadyInstalled int
	FailedUnrecognized     int
	Errors                 []ImportError
}

// ImportError represents an error during import.
type ImportError struct {
	Path    string
	Browser string
	Version string
	Error   string
}

// ProfileProvider provides profile directory management.
type ProfileProvider interface {
	// ProfileDir returns the profile directory path.
	ProfileDir(browser string, version string, profileName string) string
	// ResetProfile deletes and recreates the profile directory.
	ResetProfile(browser string, version string, profileName string) error
	// ListProfiles lists all profiles for a browser.
	ListProfiles(browser string) ([]install.ProfileInfo, error)
	// CleanOrphanedProfiles finds orphaned profiles for uninstalled versions.
	// Returns paths; actual deletion is handled by the caller.
	CleanOrphanedProfiles(browser string) ([]string, error)
}

// LaunchProvider provides browser launching.
type LaunchProvider interface {
	Run(opts LaunchOptions) error
	PreviewCommand(opts LaunchOptions) (string, []string, error)
}

// LaunchOptions maps to launch.Options for CLI use.
type LaunchOptions struct {
	Browser     string
	Version     string
	URLs        []string
	Headless    bool
	Incognito   bool
	NewWindow   bool
	ProfileName string
	ExtraArgs   []string
	NativeMode  bool
	Detached    bool
	DryRun      bool
	Proxy       string // proxy URL for the browser (empty = no proxy)

	// Fingerprint is the fingerprint isolation config string.
	// Supports: "standard", "random", "none", JSON, or "@filepath".
	Fingerprint string
	Plugins     []string // names of plugins to activate for this launch
}

// DownloadProvider provides file downloading with progress.
type DownloadProvider interface {
	Download(url string, destPath string, onProgress func(downloaded, total int64, percent float64)) (string, error)
}

// ShortcutProvider provides desktop shortcut creation.
type ShortcutProvider interface {
	Create(opts shortcut.Options) error
	Remove(name string, desktopDir string) error
	List(desktopDir string) ([]string, error)
}

// SourceProvider provides browser version data sources.
type SourceProvider interface {
	ResolveVersion(browser string, version string) (source.VersionInfo, error)
	ListVersions(browser string, channel string, versionPrefix string) ([]source.VersionInfo, error)
	// Describe returns a human-readable description of the data source(s).
	Describe() string
}

// App is the main CLI application.
type App struct {
	Name        string
	Version     string
	Description string
	RootCmd     *Command
	Context     *Context
}

// NewApp creates a new CLI application.
func NewApp(name, version string, ctx *Context) *App {
	return &App{
		Name:    name,
		Version: version,
		Context: ctx,
		RootCmd: &Command{
			Name:        name,
			Description: "浏览器版本管理工具",
			Flags: []*Flag{
				{Name: "help", Short: "h", Usage: "显示帮助"},
				{Name: "version", Short: "v", Usage: "显示版本"},
				{Name: "verbose", Short: "V", Usage: "输出详细调试信息"},
			},
		},
	}
}

// AddCommand adds a top-level command.
func (a *App) AddCommand(cmd *Command) {
	a.RootCmd.SubCommands = append(a.RootCmd.SubCommands, cmd)
}

// Execute runs the CLI with the given arguments (excluding program name).
func (a *App) Execute(args []string) error {
	if len(args) == 0 {
		a.printRootHelp()
		return nil
	}

	// Check for help flags (--help, -h only - not "help" as a command)
	if args[0] == "--help" || args[0] == "-h" {
		a.printRootHelp()
		return nil
	}

	// Check for version flag
	if args[0] == "--version" || args[0] == "-v" || args[0] == "version" {
		fmt.Fprintf(a.Context.Stdout, "%s %s\n", a.Name, a.Version)
		return nil
	}

	// Find the command
	cmd, remainingArgs, candidates, matched := findCommandWithTypo(a.RootCmd, args)

	// Command not found at some level
	if !matched {
		if cmd == a.RootCmd {
			// Case 1: Unknown top-level command
			a.printRootHelp()
			printTypoSuggestion(a.Context.Stderr, remainingArgs[0], candidates, a.RootCmd)
			return fmt.Errorf("%s", i18n.Tfmt("error.unknown_command", remainingArgs[0]))
		}
		// Parent command matched but next arg didn't match any subcommand
		if cmd.Run != nil {
			// Check if remaining args contain --help/-h before passing to Run
			if hasHelpFlag(remainingArgs) {
				a.printCommandHelp(cmd)
				return nil
			}
			// cmd has Run, remaining args are its arguments — pass them through
			return cmd.Run(a.Context, remainingArgs)
		}
		// Case 2: Subcommand-only command (no Run), subcommand not found
		a.printCommandHelp(cmd)
		printTypoSuggestion(a.Context.Stderr, remainingArgs[0], candidates, cmd)
		return fmt.Errorf("%s", i18n.Tfmt("error.unknown_subcommand", cmd.Name, remainingArgs[0]))
	}

	// Check for help flag on subcommand
	if hasHelpFlag(remainingArgs) {
		a.printCommandHelp(cmd)
		return nil
	}

	// Run the command
	if cmd.Run != nil {
		return cmd.Run(a.Context, remainingArgs)
	}

	// Command has SubCommands but no Run, and there are remaining args
	// → treat as unknown subcommand
	if len(remainingArgs) > 0 && len(cmd.SubCommands) > 0 {
		a.printCommandHelp(cmd)
		subCandidates := collectCommandCandidates(cmd)
		printTypoSuggestion(a.Context.Stderr, remainingArgs[0], subCandidates, cmd)
		return fmt.Errorf("%s", i18n.Tfmt("error.unknown_subcommand", cmd.Name, remainingArgs[0]))
	}

	// No Run function, no remaining args → print help for this command
	a.printCommandHelp(cmd)
	return nil
}

// hasHelpFlag checks if any arg is a help flag.
func hasHelpFlag(args []string) bool {
	for _, arg := range args {
		if arg == "--help" || arg == "-h" {
			return true
		}
	}
	return false
}

func (a *App) printRootHelp() {
	w := a.Context.Stdout
	fmt.Fprintf(w, "%s - %s\n\n", a.Name, a.RootCmd.Description)
	fmt.Fprint(w, i18n.Tfmt("root.usage", a.Name)+"\n\n")
	fmt.Fprintf(w, "%s\n", i18n.T("root.commands"))

	// Find max name length for alignment
	maxLen := 0
	for _, cmd := range a.RootCmd.SubCommands {
		if len(cmd.Name) > maxLen {
			maxLen = len(cmd.Name)
		}
	}

	for _, cmd := range a.RootCmd.SubCommands {
		fmt.Fprintf(w, "  %-*s  %s\n", maxLen, cmd.Name, cmd.Description)
	}

	fmt.Fprintf(w, "\n%s\n", i18n.T("root.flags"))
	fmt.Fprintf(w, "  -h, --help     %s\n", i18n.T("root.flag_help"))
	fmt.Fprintf(w, "  -V, --verbose  %s\n", i18n.T("root.flag_verbose"))
	fmt.Fprintf(w, "  -v, --version  %s\n", i18n.T("root.flag_version"))
	fmt.Fprintf(w, "\n%s\n", i18n.Tfmt("root.help_line1", a.Name))
	fmt.Fprintf(w, "%s\n", i18n.Tfmt("root.help_line2", a.Name))
}

func (a *App) printCommandHelp(cmd *Command) {
	w := a.Context.Stdout

	fmt.Fprintf(w, "%s\n\n", cmd.Description)

	if cmd.Usage != "" {
		fmt.Fprintf(w, "%s\n  %s\n\n", i18n.T("cmd.usage"), cmd.Usage)
	} else {
		fmt.Fprintf(w, "%s\n  %s %s [options]\n\n", i18n.T("cmd.usage"), a.Name, cmd.Name)
	}

	if len(cmd.Examples) > 0 {
		fmt.Fprintf(w, "%s\n", i18n.T("cmd.examples"))
		for _, ex := range cmd.Examples {
			fmt.Fprintf(w, "  $ %s %s\n", a.Name, ex)
		}
		fmt.Fprintln(w)
	}

	if len(cmd.SubCommands) > 0 {
		fmt.Fprintf(w, "%s\n", i18n.T("cmd.subcommands"))
		maxLen := 0
		for _, sub := range cmd.SubCommands {
			if len(sub.Name) > maxLen {
				maxLen = len(sub.Name)
			}
		}
		for _, sub := range cmd.SubCommands {
			fmt.Fprintf(w, "  %-*s  %s\n", maxLen, sub.Name, sub.Description)
		}
		fmt.Fprintln(w)
	}

	if len(cmd.Flags) > 0 {
		fmt.Fprintf(w, "%s\n", i18n.T("cmd.flags"))
		for _, f := range cmd.Flags {
			short := "  "
			if f.Short != "" {
				short = "-" + f.Short + ","
			}
			fmt.Fprintf(w, "  %s --%-12s %s", short, f.Name, f.Usage)
			if f.Default != "" {
				fmt.Fprintf(w, " (%s: %s)", i18n.T("cmd.default"), f.Default)
			}
			fmt.Fprintln(w)
		}
		fmt.Fprintln(w)
	}

	fmt.Fprintf(w, "%s\n", i18n.Tfmt("cmd.help_line", a.Name, cmd.Name))
}

// ParseFlags parses flags from args, returning flag values and remaining positional args.
func ParseFlags(args []string, flags []*Flag) (map[string]string, []string, error) {
	result := make(map[string]string)
	var positional []string

	// Set defaults
	for _, f := range flags {
		if f.Default != "" {
			result[f.Name] = f.Default
		}
	}

	i := 0
	for i < len(args) {
		arg := args[i]

		if !strings.HasPrefix(arg, "-") {
			positional = append(positional, arg)
			i++
			continue
		}

		found := false
		for _, f := range flags {
			// Long form: exact match or --name=value
			if arg == "--"+f.Name || strings.HasPrefix(arg, "--"+f.Name+"=") {
				if f.HasValue {
					if eq := strings.Index(arg, "="); eq > 0 {
						result[f.Name] = arg[eq+1:]
					} else if i+1 < len(args) {
						i++
						result[f.Name] = args[i]
					}
				} else {
					result[f.Name] = "true"
				}
				found = true
				break
			}
			// Short form
			if f.Short != "" && arg == "-"+f.Short {
				if f.HasValue {
					if i+1 < len(args) {
						i++
						result[f.Name] = args[i]
					}
				} else {
					result[f.Name] = "true"
				}
				found = true
				break
			}
		}

		if !found {
			return nil, nil, fmt.Errorf("%s", i18n.Tfmt("error.unknown_option", arg))
		}

		i++
	}

	return result, positional, nil
}

// ErrorExit prints an error and returns an error with exit code info.
func ErrorExit(msg string) error {
	return fmt.Errorf("%s", msg)
}

// PrintTable prints a simple formatted table.
// displayWidth returns the display width of a string, accounting for
// wide (CJK) and combining characters. Most CJK characters have width 2,
// ASCII characters have width 1, and combining marks have width 0.
func displayWidth(s string) int {
	w := 0
	for _, r := range s {
		w += runeWidth(r)
	}
	return w
}

// runeWidth returns the display width of a single rune.
func runeWidth(r rune) int {
	if r == 0 || r < 32 {
		return 0
	}
	if unicode.IsControl(r) {
		return 0
	}
	if isWideRune(r) {
		return 2
	}
	return 1
}

// isWideRune reports whether a rune is a wide (full-width) character
// that typically occupies 2 columns in a terminal. Covers CJK, Hangul,
// full-width ASCII, and CJK punctuation.
func isWideRune(r rune) bool {
	switch {
	case r >= 0x1100 && r <= 0x115F: // Hangul Jamo
		return true
	case r >= 0x2E80 && r <= 0x303E: // CJK Radicals, Kangxi
		return true
	case r >= 0x3041 && r <= 0x33FF: // Hiragana, Katakana, CJK Symbols
		return true
	case r >= 0x3400 && r <= 0x4DBF: // CJK Unified Ideographs Extension A
		return true
	case r >= 0x4E00 && r <= 0x9FFF: // CJK Unified Ideographs
		return true
	case r >= 0xA000 && r <= 0xA4CF: // Yi Syllables, Yi Radicals
		return true
	case r >= 0xAC00 && r <= 0xD7A3: // Hangul Syllables
		return true
	case r >= 0xF900 && r <= 0xFAFF: // CJK Compatibility Ideographs
		return true
	case r >= 0xFE30 && r <= 0xFE4F: // CJK Compatibility Forms
		return true
	case r >= 0xFF00 && r <= 0xFF60: // Fullwidth Forms
		return true
	case r >= 0xFFE0 && r <= 0xFFE6: // Fullwidth Signs
		return true
	case r >= 0x20000 && r <= 0x2FFFD: // CJK Unified Ideographs Extension B-F
		return true
	case r >= 0x30000 && r <= 0x3FFFD: // CJK Unified Ideographs Extension G
		return true
	}
	return false
}

// padRight pads s with spaces on the right to reach the given display width.
func padRight(s string, width int) string {
	dw := displayWidth(s)
	if dw >= width {
		return s
	}
	return s + strings.Repeat(" ", width-dw)
}

// PrintTable prints a formatted table with aligned columns.
// Column widths are calculated using display width (not byte length),
// so CJK characters are correctly aligned in terminals.
func PrintTable(w io.Writer, headers []string, rows [][]string) {
	// Calculate column widths using display width
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = displayWidth(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) {
				dw := displayWidth(cell)
				if dw > widths[i] {
					widths[i] = dw
				}
			}
		}
	}

	// Print header
	for i, h := range headers {
		if i > 0 {
			fmt.Fprint(w, "  ")
		}
		fmt.Fprint(w, padRight(h, widths[i]))
	}
	fmt.Fprintln(w)

	// Print separator
	for i, width := range widths {
		if i > 0 {
			fmt.Fprint(w, "  ")
		}
		fmt.Fprint(w, strings.Repeat("-", width))
	}
	fmt.Fprintln(w)

	// Print rows
	for _, row := range rows {
		for i, cell := range row {
			if i > 0 {
				fmt.Fprint(w, "  ")
			}
			if i < len(widths) {
				fmt.Fprint(w, padRight(cell, widths[i]))
			} else {
				fmt.Fprint(w, cell)
			}
		}
		fmt.Fprintln(w)
	}
}

// FormatSize formats a byte size into a human-readable string.
func FormatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// FormatSpeed formats a speed in bytes per second to a human-readable string.
func FormatSpeed(bps float64) string {
	if bps < 1024 {
		return fmt.Sprintf("%.0f B/s", bps)
	}
	if bps < 1024*1024 {
		return fmt.Sprintf("%.1f KB/s", bps/1024)
	}
	if bps < 1024*1024*1024 {
		return fmt.Sprintf("%.1f MB/s", bps/(1024*1024))
	}
	return fmt.Sprintf("%.1f GB/s", bps/(1024*1024*1024))
}

// formatDuration formats a duration in a human-readable way.
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
}

// Stdout returns stdout writer (convenience).
func (c *Context) Printf(format string, a ...interface{}) {
	fmt.Fprintf(c.Stdout, format, a...)
}

func (c *Context) Println(a ...interface{}) {
	fmt.Fprintln(c.Stdout, a...)
}

func (c *Context) Errorf(format string, a ...interface{}) {
	fmt.Fprintf(c.Stderr, format, a...)
}

// DefaultContext creates a context with standard I/O.
func DefaultContext() *Context {
	return &Context{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Stdin:  os.Stdin,
	}
}
