// Package cli provides the command-line interface for bm.
package cli

import (
	"fmt"
	"io"
	"os"
	"strings"
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
	Config   ConfigProvider
	Browsers BrowserProvider
	Install  InstallProvider
	Profile  ProfileProvider
	Launch   LaunchProvider
	Repo     RepoProvider
	Download DownloadProvider
	Source   SourceProvider
	Logger   Logger
	Serve    ServeProvider
}

// Confirm asks the user for confirmation and returns true if they agree.
// It writes the prompt to Stderr and reads from Stdin.
func (ctx *Context) Confirm(prompt string) bool {
	fmt.Fprintf(ctx.Stderr, "%s [y/N]: ", prompt)
	var response string
	fmt.Fscanln(ctx.Stdin, &response)
	response = strings.ToLower(strings.TrimSpace(response))
	return response == "y" || response == "yes"
}

// ServeProvider provides the HTTP serve functionality.
type ServeProvider interface {
	// StartFromConfig starts the HTTP server using configuration from bws-serve.ini.
	// baseDir is the base directory containing packages/ and bin/ subdirectories.
	// If empty, the executable directory is used.
	StartFromConfig(baseDir string) error

	// SetConfig sets a single serve configuration key.
	SetConfig(key string, value string) error

	// GetConfig gets the value of a single serve configuration key.
	GetConfig(key string) (string, error)

	// GetFullConfig returns the full serve configuration.
	GetFullConfig() ServeConfigInfo

	// ConfigPath returns the path to the serve config file.
	ConfigPath() string
}

// ServeConfigInfo is a simplified view of serve configuration for CLI display.
type ServeConfigInfo struct {
	Host         string
	Port         string
	BaseDir      string
	SyncEnabled  bool
	SyncInterval string
	SyncBrowsers string
	SyncChannels string
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
	Scan() ([]RepoScanResult, error)
	Import(force bool) (*RepoImportSummary, error)
}

// RepoScanResult is a simplified view of a scanned repository entry.
type RepoScanResult struct {
	Path    string
	Browser string
	Version string
	Arch    string
	Status  string
	Detail  string
}

// RepoImportSummary is a simplified view of an import summary.
type RepoImportSummary struct {
	Total                  int
	Success                int
	Failed                 int
	Skipped                int
	SkippedIncompatible    int
	SkippedAlreadyInstalled int
}

// PathsProvider provides path management.
type PathsProvider interface {
	VersionDir(browser string, version string) string
	EnsureAll() error
}

// ConfigProvider provides configuration.
type ConfigProvider interface {
	// Basic settings
	DefaultBrowser() string
	SetDefaultBrowser(browser string) error
	DefaultChannel() string
	SetDefaultChannel(channel string) error

	// Repository
	GetRepoPath() string
	SetRepoPath(path string) error

	// Aliases
	GetAlias(name string) (string, bool)
	AddAlias(name, target string) error
	RemoveAlias(name string) error
	ListAliases() map[string]string

	// Logging
	GetLogLevel() string
	SetLogLevel(level string) error

	// Data directory
	GetDataDir() string
	SetDataDir(path string) error

	// Remote source (custom HTTP source for bws serve)
	GetRemoteSource() string
	SetRemoteSource(url string) error
	ClearRemoteSource() error

	// Source switches
	IsServeSourceEnabled() bool
	SetServeSourceEnabled(v bool) error
	IsOmahaSourceEnabled() bool
	SetOmahaSourceEnabled(v bool) error
	IsFirefoxFTPEnabled() bool
	SetFirefoxFTPEnabled(v bool) error

	// Disk space
	GetDiskSpaceThresholdGB() int
	SetDiskSpaceThresholdGB(v int) error

	// File path
	ConfigPath() string
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
	ListInstalled() ([]InstalledVersion, error)
	ListInstalledByBrowser(browser string) ([]InstalledVersion, error)
	GetRecord(browser, version string) (*InstallRecord, error)
	Uninstall(browser, version string) error
	// Install from a local directory
	InstallFromDir(browser, version, sourceDir string) (*InstallRecord, error)
	// Install from a local archive file
	InstallFromFile(browser, version, filePath string) (*InstallRecord, error)
	// System browser support
	HasSystem() bool
	ListWithSystem() ([]InstalledVersion, error)
	ListWithSystemByBrowser(browser string) ([]InstalledVersion, error)
	IsSystemVersion(browser, version string) bool
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

// InstalledVersion is a simplified view of an installed version.
type InstalledVersion struct {
	Browser  string
	Version  string
	Channel  string
	Size     int64
	IsSystem bool
	Source   string
}

// InstallRecord is a simplified view of install.InstallRecord.
type InstallRecord struct {
	Browser        string
	Version        string
	InstalledAt    string
	Platform       string
	Arch           string
	InstallDir     string
	ExecutablePath string
	Size           int64
	Source         string
}

// ProfileProvider provides profile directory management.
type ProfileProvider interface {
	// ProfileDir returns the profile directory path.
	ProfileDir(browser string, version string, profileName string) string
	// ResetProfile deletes and recreates the profile directory.
	ResetProfile(browser string, version string, profileName string) error
	// ListProfiles lists all profiles for a browser.
	ListProfiles(browser string) ([]ProfileInfo, error)
	// CleanOrphanedProfiles finds orphaned profiles for uninstalled versions.
	CleanOrphanedProfiles(browser string) ([]string, error)
}

// ProfileInfo describes a browser profile.
type ProfileInfo struct {
	Name    string
	Path    string
	Type    string // "named" or "version"
	Version string // for version-type profiles
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
}

// DownloadProvider provides file downloading with progress.
type DownloadProvider interface {
	Download(url string, destPath string, onProgress func(downloaded, total int64, percent float64)) (string, error)
}

// SourceProvider provides browser version data sources.
type SourceProvider interface {
	ResolveVersion(browser string, version string) (SourceVersionInfo, error)
	ListVersions(browser string, channel string) ([]SourceVersionInfo, error)
	// Describe returns a human-readable description of the data source(s).
	Describe() string
}

// SourceVersionInfo is a simplified view of source.VersionInfo for CLI.
type SourceVersionInfo struct {
	Browser     string
	Version     string
	Channel     string
	Platform    string
	Arch        string
	DownloadURL string
	Size        int64
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
			Description: "Browser Manager - manage multiple browser versions",
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
	cmd, remainingArgs := a.findCommand(args)
	// If we ended up back at root with args remaining, the command was not found
	if cmd == a.RootCmd && len(remainingArgs) > 0 && remainingArgs[0] == args[0] {
		fmt.Fprintf(a.Context.Stderr, "unknown command: %s\n", args[0])
		a.printRootHelp()
		return fmt.Errorf("unknown command: %s", args[0])
	}

	// Check for help flag on subcommand
	for _, arg := range remainingArgs {
		if arg == "--help" || arg == "-h" {
			a.printCommandHelp(cmd)
			return nil
		}
	}

	// Run the command
	if cmd.Run != nil {
		return cmd.Run(a.Context, remainingArgs)
	}

	// No Run function, print help for this command
	a.printCommandHelp(cmd)
	return nil
}

// findCommand finds the command matching the argument path.
func (a *App) findCommand(args []string) (*Command, []string) {
	return findCommandRecursive(a.RootCmd, args)
}

func findCommandRecursive(cmd *Command, args []string) (*Command, []string) {
	if len(args) == 0 {
		return cmd, args
	}

	name := args[0]
	for _, sub := range cmd.SubCommands {
		if sub.Name == name {
			return findCommandRecursive(sub, args[1:])
		}
		for _, alias := range sub.Aliases {
			if alias == name {
				return findCommandRecursive(sub, args[1:])
			}
		}
	}

	return cmd, args
}

func (a *App) printRootHelp() {
	w := a.Context.Stdout
	fmt.Fprintf(w, "%s - %s\n\n", a.Name, a.RootCmd.Description)
	fmt.Fprintf(w, "Usage:\n  %s <command> [options]\n\n", a.Name)
	fmt.Fprintf(w, "Available commands:\n")

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

	fmt.Fprintf(w, "\nFlags:\n")
	fmt.Fprintf(w, "  -h, --help     Show help\n")
	fmt.Fprintf(w, "  -v, --version  Show version\n")
	fmt.Fprintf(w, "\nUse '%s <command> --help' for more information about a command.\n", a.Name)
	fmt.Fprintf(w, "Use '%s help <topic>' for detailed help.\n", a.Name)
}

func (a *App) printCommandHelp(cmd *Command) {
	w := a.Context.Stdout

	fmt.Fprintf(w, "%s\n\n", cmd.Description)

	if cmd.Usage != "" {
		fmt.Fprintf(w, "Usage:\n  %s\n\n", cmd.Usage)
	} else {
		fmt.Fprintf(w, "Usage:\n  %s %s [options]\n\n", a.Name, cmd.Name)
	}

	if len(cmd.Examples) > 0 {
		fmt.Fprintf(w, "Examples:\n")
		for _, ex := range cmd.Examples {
			fmt.Fprintf(w, "  $ %s %s\n", a.Name, ex)
		}
		fmt.Fprintln(w)
	}

	if len(cmd.SubCommands) > 0 {
		fmt.Fprintf(w, "Subcommands:\n")
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
		fmt.Fprintf(w, "Flags:\n")
		for _, f := range cmd.Flags {
			short := "  "
			if f.Short != "" {
				short = "-" + f.Short + ","
			}
			fmt.Fprintf(w, "  %s --%-12s %s", short, f.Name, f.Usage)
			if f.Default != "" {
				fmt.Fprintf(w, " (default: %s)", f.Default)
			}
			fmt.Fprintln(w)
		}
		fmt.Fprintln(w)
	}

	fmt.Fprintf(w, "Use '%s help %s' for detailed help.\n", a.Name, cmd.Name)
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
			// Long form
			if strings.HasPrefix(arg, "--"+f.Name) {
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
			return nil, nil, fmt.Errorf("未知选项: %s", arg)
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
func PrintTable(w io.Writer, headers []string, rows [][]string) {
	// Calculate column widths
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}

	// Print header
	for i, h := range headers {
		if i > 0 {
			fmt.Fprint(w, "  ")
		}
		fmt.Fprintf(w, "%-*s", widths[i], h)
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
				fmt.Fprintf(w, "%-*s", widths[i], cell)
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
