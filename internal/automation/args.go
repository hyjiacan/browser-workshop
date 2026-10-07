// Package automation exposes the browser endpoints that external test
// frameworks connect to: CDP for Playwright/Puppeteer and WebDriver for
// Selenium/WebdriverIO. bws itself implements no test logic — it only injects
// the right flags and discovers the resulting endpoints.
package automation

import (
	"fmt"
	"strconv"
	"strings"
)

// CDP flag names understood by Chromium-family browsers.
const (
	FlagRemoteDebuggingPort    = "--remote-debugging-port"
	FlagRemoteDebuggingAddress = "--remote-debugging-address"
	FlagDisableBlinkFeatures   = "--disable-blink-features"
)

// cdpLoopback binds the debugging endpoint to loopback only. The endpoint
// grants full control over the browser, so it must never be exposed publicly
// (design principle 4: "端点即凭证").
const cdpLoopback = "127.0.0.1"

// HasFlag reports whether args already contains the given flag, either as a
// bare flag or in "--flag=value" form.
func HasFlag(args []string, name string) bool {
	prefix := name + "="
	for _, a := range args {
		if a == name || strings.HasPrefix(a, prefix) {
			return true
		}
	}
	return false
}

// CDPArgs returns the CDP arguments that are not already present in userArgs.
// A port of 0 asks the OS to assign a free port. User-supplied values always
// win: bws never overrides an explicit --remote-debugging-port.
func CDPArgs(userArgs []string, port int) []string {
	var out []string
	if !HasFlag(userArgs, FlagRemoteDebuggingPort) {
		out = append(out, fmt.Sprintf("%s=%d", FlagRemoteDebuggingPort, port))
	}
	if !HasFlag(userArgs, FlagRemoteDebuggingAddress) {
		out = append(out, FlagRemoteDebuggingAddress+"="+cdpLoopback)
	}
	return out
}

// CDPPortFromArgs extracts the port from a user-supplied
// "--remote-debugging-port=N" argument. It returns 0 when the flag is absent,
// bare, or carries a non-positive/invalid value.
//
// Endpoint discovery needs this: when a fixed port is known, the HTTP
// /json/version probe can be used even if the profile directory is unavailable
// (native mode never writes DevToolsActivePort).
func CDPPortFromArgs(userArgs []string) int {
	prefix := FlagRemoteDebuggingPort + "="
	for _, a := range userArgs {
		if !strings.HasPrefix(a, prefix) {
			continue
		}
		port, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(a, prefix)))
		if err != nil || port <= 0 || port > 65535 {
			return 0
		}
		return port
	}
	return 0
}

// AutomationArgs returns the automation-friendly browser arguments that are not
// already present in userArgs. Disabling the AutomationControlled blink feature
// removes the most common "this browser is automated" signal.
func AutomationArgs(userArgs []string) []string {
	var out []string
	if !HasFlag(userArgs, FlagDisableBlinkFeatures) {
		out = append(out, FlagDisableBlinkFeatures+"=AutomationControlled")
	}
	return out
}

// BrowserArgs returns the full set of automation arguments to append to a
// launch, honouring any flags the user supplied explicitly.
func BrowserArgs(userArgs []string, port int) []string {
	args := CDPArgs(userArgs, port)
	args = append(args, AutomationArgs(userArgs)...)
	return args
}

// SupportsCDP reports whether a browser exposes a usable CDP endpoint.
// Firefox's CDP support is incomplete, so it is treated as WebDriver-only.
func SupportsCDP(browser string) bool {
	switch browser {
	case "chrome", "chromium", "edge":
		return true
	default:
		return false
	}
}

// SupportsWebDriver reports whether bws can manage a driver for the browser.
// Only Chromium-family browsers have a managed driver today.
func SupportsWebDriver(browser string) bool {
	switch browser {
	case "chrome", "chromium":
		return true
	default:
		return false
	}
}
