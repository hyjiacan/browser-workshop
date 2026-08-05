// Package cli - Version resolution pipeline.
//
// Uses the Chain of Responsibility pattern to resolve a browser version
// through successively broader searches:
//
//	Step 1 — LocalResolver:    check already-installed versions
//	Step 2 — RemoteResolver:   query remote sources, prompt selection, auto-install
//	Step 3 — Fallback:         give user guidance
//
// Each resolver either handles the request (returns a *Resolution) or
// passes it to the next resolver in the chain (returns nil, nil).
package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/bws/bws/internal/source"
)

// ---------------------------------------------------------------
// Domain types
// ---------------------------------------------------------------

// versionChoice represents a selectable browser version from either local
// installations or remote source listings.
type versionChoice struct {
	Version   string
	Channel   string
	Installed bool
	Remote    *source.VersionInfo // non-nil only when !Installed
}

// Resolution is the result of a successful version resolution step.
type Resolution struct {
	Version   string
	Installed bool // true if the version was already present locally
}

// ResolutionError wraps a resolution failure with a user-friendly message.
type ResolutionError struct {
	Cause   error
	Message string // shown to the user
}

func (e *ResolutionError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	if e.Cause != nil {
		return e.Cause.Error()
	}
	return "版本解析失败"
}

func (e *ResolutionError) Unwrap() error { return e.Cause }

// ---------------------------------------------------------------
// Chain of Responsibility: Resolver interface
// ---------------------------------------------------------------

// Resolver attempts to resolve a browser@version spec. Returns:
//
//	(*Resolution, nil) — successfully resolved
//	(nil, nil)         — cannot handle, pass to next resolver
//	(nil, error)       — fatal error, stop the chain
type Resolver interface {
	Resolve(ctx *Context, browser, versionSpec string, refresh bool) (*Resolution, error)
}

// ---------------------------------------------------------------
// Step 1 — LocalResolver: checks installed versions
// ---------------------------------------------------------------

type localResolver struct{}

func (r *localResolver) Resolve(ctx *Context, browser, versionSpec string, refresh bool) (*Resolution, error) {
	resolved, err := ctx.Install.ResolveInstalledVersion(browser, versionSpec)
	if err != nil {
		if ctx.Logger != nil {
			ctx.Logger.Debug("[run] 本地未找到 %s@%s", browser, versionSpec)
		}
		return nil, nil // pass to next
	}
	if ctx.Logger != nil {
		ctx.Logger.Debug("[run] 本地已安装: %s@%s", browser, resolved)
	}
	return &Resolution{Version: resolved, Installed: true}, nil
}

// ---------------------------------------------------------------
// Step 2 — RemoteResolver: queries source, selects, installs
// ---------------------------------------------------------------

type remoteResolver struct {
	selector SelectionStrategy
}

func newRemoteResolver(selector SelectionStrategy) *remoteResolver {
	return &remoteResolver{selector: selector}
}

func (r *remoteResolver) Resolve(ctx *Context, browser, versionSpec string, refresh bool) (*Resolution, error) {
	if ctx.Source == nil || ctx.Download == nil {
		return nil, nil // remote not available, pass to next
	}

	if ctx.Logger != nil {
		ctx.Logger.Debug("[run] 本地未找到 %s@%s，尝试远程源自动安装", browser, versionSpec)
	}

	// Refresh remote source cache if requested
	if refresh {
		if ref, ok := ctx.Source.(interface{ ForceRefresh() }); ok {
			ref.ForceRefresh()
			ctx.Printf("正在刷新远程源缓存...\n")
		}
	}

	// Collect matching versions: local + remote
	choices := collectChoices(ctx, browser, versionSpec)

	if len(choices) == 0 {
		return nil, nil // no matches anywhere, pass to next
	}

	// Single choice: auto-pick
	if len(choices) == 1 {
		chosen := choices[0]
		if chosen.Installed {
			ctx.Printf("使用已安装版本: %s@%s\n", browser, chosen.Version)
			return &Resolution{Version: chosen.Version, Installed: true}, nil
		}
		return r.installAndResolve(ctx, browser, chosen)
	}

	// Multiple choices: delegate to selection strategy
	chosen, cancelled := r.selector.Select(ctx, choices)
	if cancelled {
		return nil, &ResolutionError{Message: "用户取消"}
	}

	if chosen.Installed {
		ctx.Printf("使用已安装版本: %s@%s\n", browser, chosen.Version)
		return &Resolution{Version: chosen.Version, Installed: true}, nil
	}

	return r.installAndResolve(ctx, browser, chosen)
}

func (r *remoteResolver) installAndResolve(ctx *Context, browser string, c versionChoice) (*Resolution, error) {
	ctx.Printf("正在自动安装 %s@%s...\n", browser, c.Version)
	if err := fetchAndInstall(ctx, browser, c.Version, c.Channel, false, false, true); err != nil {
		return nil, &ResolutionError{
			Cause:   err,
			Message: fmt.Sprintf("自动安装失败: %v", err),
		}
	}
	return &Resolution{Version: c.Version, Installed: false}, nil
}

// ---------------------------------------------------------------
// Step 3 — Fallback: no match found
// ---------------------------------------------------------------

type fallbackResolver struct{}

func (r *fallbackResolver) Resolve(ctx *Context, browser, versionSpec string, refresh bool) (*Resolution, error) {
	return nil, &ResolutionError{
		Message: fmt.Sprintf(
			"%s@%s 未安装，且远程源中未找到匹配版本。\n  使用 'bws ls --remote' 查看可用版本，或 'bws i %s@%s' 指定安装。",
			browser, versionSpec, browser, versionSpec,
		),
	}
}

// ---------------------------------------------------------------
// Resolution Pipeline
// ---------------------------------------------------------------

// RunResolutionChain executes the resolver chain and returns the resolved version.
// The selector is injected via the Strategy pattern.
func RunResolutionChain(ctx *Context, browser, versionSpec string, refresh bool, selector SelectionStrategy) (string, error) {
	chain := []Resolver{
		&localResolver{},
		newRemoteResolver(selector),
		&fallbackResolver{},
	}

	for _, r := range chain {
		result, err := r.Resolve(ctx, browser, versionSpec, refresh)
		if err != nil {
			return "", err
		}
		if result != nil {
			return result.Version, nil
		}
	}

	// Should never reach here (fallbackResolver always returns non-nil)
	return "", fmt.Errorf("版本解析失败: %s@%s 未找到", browser, versionSpec)
}

// ---------------------------------------------------------------
// Choice collection (shared between resolver and selector)
// ---------------------------------------------------------------

// collectChoices gathers matching versions from both local and remote sources.
// Results are sorted: installed first, then by version descending.
func collectChoices(ctx *Context, browser, versionPrefix string) []versionChoice {
	seen := make(map[string]bool)
	var choices []versionChoice

	// 1. Locally installed versions matching the prefix
	if installed, err := ctx.Install.ListInstalledByBrowser(browser); err == nil {
		for _, v := range installed {
			if matchesVersionPrefix(v.Version, versionPrefix) {
				seen[v.Version] = true
				choices = append(choices, versionChoice{
					Version:   v.Version,
					Channel:   v.Channel,
					Installed: true,
				})
			}
		}
	}

	// 2. Remote versions matching the prefix (search all channels)
	channels := []string{"stable", "beta", "esr", "dev", "canary"}
	for _, ch := range channels {
		remote, err := ctx.Source.ListVersions(browser, ch, versionPrefix)
		if err != nil {
			continue
		}
		for _, rv := range remote {
			if seen[rv.Version] {
				continue // already present as installed
			}
			seen[rv.Version] = true
			choices = append(choices, versionChoice{
				Version:   rv.Version,
				Channel:   string(rv.Channel),
				Installed: false,
				Remote:    &rv,
			})
		}
	}

	// Sort: installed first, then newer version first
	sort.Slice(choices, func(i, j int) bool {
		if choices[i].Installed != choices[j].Installed {
			return choices[i].Installed
		}
		return compareVersions(choices[i].Version, choices[j].Version) > 0
	})

	return choices
}



// compareVersions compares two dotted version strings numerically.
// Returns 1 if a > b, -1 if a < b, 0 if equal.
func compareVersions(a, b string) int {
	pa := splitVersion(a)
	pb := splitVersion(b)
	n := len(pa)
	if len(pb) > n {
		n = len(pb)
	}
	for i := 0; i < n; i++ {
		var na, nb int
		if i < len(pa) {
			na = atoi(pa[i])
		}
		if i < len(pb) {
			nb = atoi(pb[i])
		}
		if na > nb {
			return 1
		}
		if na < nb {
			return -1
		}
	}
	return 0
}

func splitVersion(v string) []string {
	return strings.Split(v, ".")
}

func atoi(s string) int {
	var n int
	for _, c := range s {
		if c < '0' || c > '9' {
			break
		}
		n = n*10 + int(c-'0')
	}
	return n
}
