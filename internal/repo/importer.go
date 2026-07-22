package repo

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/bws/bws/internal/archive"
	"github.com/bws/bws/internal/install"
	"github.com/bws/bws/internal/paths"
	"github.com/bws/bws/internal/version"
)

// ImportResult represents the result of importing a single version.
type ImportResult struct {
	SourcePath string
	Browser    string
	Version    string
	Arch       string
	Success    bool
	Error      error
	Size       int64
	SkipReason string // "incompatible-arch", "already-installed", "unsupported-format", "unrecognized"
}

// ImportSummary contains the overall result of a batch import.
type ImportSummary struct {
	Total              int
	Success            int
	Failed             int
	Skipped            int
	SkippedIncompatible int
	SkippedAlreadyInstalled int
	Results []ImportResult
}

// Importer handles batch importing from the local repository.
type Importer struct {
	scanner   *Scanner
	installer *install.Manager
}

// NewImporter creates a new importer.
func NewImporter(scanner *Scanner, installer *install.Manager) *Importer {
	return &Importer{
		scanner:   scanner,
		installer: installer,
	}
}

// ImportOptions configures a batch import.
type ImportOptions struct {
	// Force reinstalls even if already installed
	Force bool

	// DryRun scans but doesn't actually import
	DryRun bool
}

// ImportProgress reports progress during batch import.
type ImportProgress struct {
	Current int
	Total   int
	Message string
	Result  *ImportResult
	Phase   string // "scanning", "importing", "done"
}

// ProgressCallback is called during import to report progress.
type ProgressCallback func(progress ImportProgress)

// ImportAll scans the repository and imports all recognized browser versions.
func (imp *Importer) ImportAll(opts ImportOptions, onProgress ProgressCallback) (*ImportSummary, error) {
	// Phase 1: Scan
	if onProgress != nil {
		onProgress(ImportProgress{Phase: "scanning", Message: "Scanning repository..."})
	}

	matches, err := imp.scanner.Scan()
	if err != nil {
		return nil, fmt.Errorf("scanning repository: %w", err)
	}

	summary := &ImportSummary{
		Total: len(matches),
	}

	// Filter importable matches
	var importable []MatchResult

	for _, m := range matches {
		// Only import OK or Partial matches with browser+version
		if (m.Status == MatchOK || m.Status == MatchPartial) &&
			m.Browser != "" && m.Version != "" {
			// Check arch compatibility
			if !paths.ArchCompatible(m.Arch) {
				summary.Skipped++
				summary.SkippedIncompatible++
				summary.Results = append(summary.Results, ImportResult{
					SourcePath: m.Path,
					Browser:    m.Browser,
					Version:    m.Version,
					Arch:       m.Arch,
					Success:    false,
					SkipReason: "incompatible-arch",
					Error:      fmt.Errorf("incompatible architecture: %s (current: %s)", m.Arch, paths.Arch()),
				})
				continue
			}
			importable = append(importable, m)
		} else {
			summary.Skipped++
			summary.Results = append(summary.Results, ImportResult{
				SourcePath: m.Path,
				Browser:    m.Browser,
				Version:    m.Version,
				Arch:       m.Arch,
				Success:    false,
				SkipReason: "unrecognized",
				Error:      fmt.Errorf("unrecognized: %s", m.Detail),
			})
		}
	}

	if onProgress != nil {
		onProgress(ImportProgress{
			Phase:   "importing",
			Total:   len(importable),
			Message: fmt.Sprintf("Found %d importable versions (%d skipped)", len(importable), summary.Skipped),
		})
	}

	// Phase 2: Import each
	for i, match := range importable {
		result := ImportResult{
			SourcePath: match.Path,
			Browser:    match.Browser,
			Version:    match.Version,
			Arch:       match.Arch,
		}

		if opts.DryRun {
			result.Success = true
			summary.Success++
			summary.Results = append(summary.Results, result)

			if onProgress != nil {
				onProgress(ImportProgress{
					Phase:   "importing",
					Current: i + 1,
					Total:   len(importable),
					Message: fmt.Sprintf("[dry-run] Would import %s@%s", match.Browser, match.Version),
					Result:  &result,
				})
			}
			continue
		}

		// Check if already installed
		if imp.installer.IsInstalled(match.Browser, match.Version) {
			if !opts.Force {
				result.Success = true
				result.SkipReason = "already-installed"
				summary.Success++
				summary.SkippedAlreadyInstalled++
				summary.Results = append(summary.Results, result)

				if onProgress != nil {
					onProgress(ImportProgress{
						Phase:   "importing",
						Current: i + 1,
						Total:   len(importable),
						Message: fmt.Sprintf("Already installed: %s@%s", match.Browser, match.Version),
						Result:  &result,
					})
				}
				continue
			}
			// Force: uninstall first
			if err := imp.installer.Uninstall(match.Browser, match.Version); err != nil {
				result.Error = fmt.Errorf("force uninstall failed: %w", err)
				summary.Failed++
				summary.Results = append(summary.Results, result)
				if onProgress != nil {
					onProgress(ImportProgress{
						Phase:   "importing",
						Current: i + 1,
						Total:   len(importable),
						Message: fmt.Sprintf("Failed (uninstall): %s@%s", match.Browser, match.Version),
						Result:  &result,
					})
				}
				continue
			}
		}

		// Install from directory or file
		var record *version.InstallRecord
		var installErr error

		if match.IsFile {
			record, installErr = imp.installFromFile(match)
		} else {
			record, installErr = imp.installer.InstallFromDir(install.InstallOptions{
				Browser:   match.Browser,
				Version:   match.Version,
				Source:    "local-repo",
				SourceDir: match.Path,
			}, nil)
		}

		if installErr != nil {
			result.Error = installErr
			summary.Failed++
		} else {
			result.Success = true
			result.Size = record.Size
			summary.Success++
		}

		summary.Results = append(summary.Results, result)

		if onProgress != nil {
			msg := fmt.Sprintf("%s@%s: ", match.Browser, match.Version)
			if result.Success {
				msg += "imported"
			} else {
				msg += fmt.Sprintf("failed: %v", installErr)
			}
			onProgress(ImportProgress{
				Phase:   "importing",
				Current: i + 1,
				Total:   len(importable),
				Message: msg,
				Result:  &result,
			})
		}
	}

	if onProgress != nil {
		onProgress(ImportProgress{
			Phase:   "done",
			Current: summary.Success,
			Total:   len(importable),
			Message: fmt.Sprintf("Import complete: %d succeeded, %d failed, %d skipped",
				summary.Success, summary.Failed, summary.Skipped),
		})
	}

	return summary, nil
}

// installFromFile installs a browser version from a package file.
// It detects the format from the file extension, extracts if necessary,
// and then installs from the extracted directory.
// Uses the archive package for multi-format extraction support.
func (imp *Importer) installFromFile(match MatchResult) (*version.InstallRecord, error) {
	// Check if it's a supported archive format
	if !archive.IsSupportedFormat(match.DirName) {
		return nil, fmt.Errorf("unsupported package format: %s", filepath.Ext(match.DirName))
	}

	// Extract archive to temp dir
	tmpDir, err := os.MkdirTemp("", "bws-import-*")
	if err != nil {
		return nil, fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	if _, err := archive.ExtractRecursive(match.Path, tmpDir); err != nil {
		return nil, fmt.Errorf("extracting archive: %w", err)
	}

	// Find the actual content directory
	sourceDir, err := findContentDir(tmpDir, match.Browser)
	if err != nil {
		return nil, fmt.Errorf("finding content in extracted archive: %w", err)
	}

	return imp.installer.InstallFromDir(install.InstallOptions{
		Browser:   match.Browser,
		Version:   match.Version,
		Source:    "local-repo",
		SourceDir: sourceDir,
	}, nil)
}

// findContentDir finds the actual content directory within an extracted archive.
// It first tries to locate the browser executable (by common names),
// then falls back to the single-subdirectory heuristic.
func findContentDir(root string, browserName string) (string, error) {
	// Get common executable names for this browser
	exeCandidates := browserExecutableCandidates(browserName)

	// Use the archive package's FindContentDir which tries executable search first
	return archive.FindContentDir(root, browserName, paths.Platform(), paths.Arch(), exeCandidates)
}

// browserExecutableCandidates returns common executable file names for a given browser.
// These are used to locate the browser executable inside extracted archives.
func browserExecutableCandidates(browserName string) []string {
	lower := strings.ToLower(browserName)

	// Platform-specific extensions
	exeExt := ""
	if paths.Platform() == "windows" {
		exeExt = ".exe"
	}

	switch lower {
	case "chrome", "google chrome", "google-chrome":
		return []string{
			"chrome" + exeExt,
			"chrome.exe", // always include .exe variant for archives from other platforms
			"Google Chrome" + exeExt,
		}
	case "firefox", "mozilla firefox", "mozilla-firefox":
		return []string{
			"firefox" + exeExt,
			"firefox.exe",
		}
	case "chromium":
		return []string{
			"chromium" + exeExt,
			"chromium.exe",
			"chrome" + exeExt,
		}
	case "edge", "microsoft edge", "microsoft-edge", "msedge":
		return []string{
			"msedge" + exeExt,
			"msedge.exe",
			"edge" + exeExt,
		}
	case "brave":
		return []string{
			"brave" + exeExt,
			"brave.exe",
			"brave-browser" + exeExt,
		}
	case "opera":
		return []string{
			"opera" + exeExt,
			"opera.exe",
		}
	case "safari":
		return []string{
			"Safari" + exeExt,
		}
	default:
		// Generic fallback: browser name as executable
		return []string{
			lower + exeExt,
			lower + ".exe",
		}
	}
}

// PrintSummary prints a human-readable summary to the writer.
func (s *ImportSummary) PrintSummary(w io.Writer) {
	fmt.Fprintf(w, "\nImport Summary:\n")
	fmt.Fprintf(w, "  Total scanned:    %d\n", s.Total)
	fmt.Fprintf(w, "  Succeeded:        %d\n", s.Success)
	fmt.Fprintf(w, "  Failed:           %d\n", s.Failed)
	fmt.Fprintf(w, "  Skipped:          %d\n", s.Skipped)
	if s.SkippedIncompatible > 0 {
		fmt.Fprintf(w, "    (incompatible arch):  %d\n", s.SkippedIncompatible)
	}
	if s.SkippedAlreadyInstalled > 0 {
		fmt.Fprintf(w, "    (already installed):  %d\n", s.SkippedAlreadyInstalled)
	}

	if len(s.Results) > 0 {
		fmt.Fprintf(w, "\nResults:\n")
		for _, r := range s.Results {
			status := "OK"
			if !r.Success {
				if r.SkipReason == "incompatible-arch" {
					status = "SKIP(arch)"
				} else if r.SkipReason == "unsupported-format" {
					status = "SKIP(format)"
				} else if r.SkipReason == "unrecognized" {
					status = "SKIP"
				} else {
					status = "FAIL"
				}
			} else if r.SkipReason == "already-installed" {
				status = "OK(installed)"
			}
			fmt.Fprintf(w, "  %-16s %s@%s", status, r.Browser, r.Version)
			if r.Arch != "" {
				fmt.Fprintf(w, " [%s]", r.Arch)
			}
			if r.Error != nil {
				fmt.Fprintf(w, " (%v)", r.Error)
			}
			fmt.Fprintln(w)
		}
	}
}
