package automation

import (
	"strings"
	"testing"
)

func contains(args []string, want string) bool {
	for _, a := range args {
		if a == want {
			return true
		}
	}
	return false
}

// CDP-01: debugging flags are injected when absent.
func TestCDPArgs_Injects(t *testing.T) {
	args := CDPArgs(nil, 0)

	if !contains(args, "--remote-debugging-port=0") {
		t.Errorf("缺少 --remote-debugging-port=0: %v", args)
	}
	if !contains(args, "--remote-debugging-address=127.0.0.1") {
		t.Errorf("缺少 loopback 地址: %v", args)
	}
}

// CDP-02: user-supplied port wins.
func TestCDPArgs_UserPortWins(t *testing.T) {
	user := []string{"--remote-debugging-port=9222"}
	args := CDPArgs(user, 0)

	for _, a := range args {
		if strings.HasPrefix(a, "--remote-debugging-port") {
			t.Errorf("不应覆盖用户端口: %v", args)
		}
	}
	if !contains(args, "--remote-debugging-address=127.0.0.1") {
		t.Errorf("地址标志仍应注入: %v", args)
	}
}

// CDP-03: automation-detection flag is injected.
func TestAutomationArgs(t *testing.T) {
	args := AutomationArgs(nil)
	if !contains(args, "--disable-blink-features=AutomationControlled") {
		t.Errorf("缺少 AutomationControlled 关闭标志: %v", args)
	}

	// Respect an explicit user value.
	args = AutomationArgs([]string{"--disable-blink-features=Foo"})
	if len(args) != 0 {
		t.Errorf("用户已提供该标志时不应追加: %v", args)
	}
}

// BrowserArgs combines CDP and automation flags.
func TestBrowserArgs(t *testing.T) {
	args := BrowserArgs(nil, 0)
	if !contains(args, "--remote-debugging-port=0") {
		t.Errorf("缺少 CDP 端口: %v", args)
	}
	if !contains(args, "--disable-blink-features=AutomationControlled") {
		t.Errorf("缺少自动化标志: %v", args)
	}
}

func TestHasFlag(t *testing.T) {
	cases := []struct {
		args []string
		name string
		want bool
	}{
		{[]string{"--foo"}, "--foo", true},
		{[]string{"--foo=bar"}, "--foo", true},
		{[]string{"--foobar"}, "--foo", false},
		{nil, "--foo", false},
	}
	for _, c := range cases {
		if got := HasFlag(c.args, c.name); got != c.want {
			t.Errorf("HasFlag(%v, %q) = %v, want %v", c.args, c.name, got, c.want)
		}
	}
}

func TestSupports(t *testing.T) {
	if !SupportsCDP("chrome") || !SupportsCDP("chromium") || !SupportsCDP("edge") {
		t.Error("Chromium 系浏览器应支持 CDP")
	}
	if SupportsCDP("firefox") {
		t.Error("Firefox 不应报告支持 CDP")
	}
	if !SupportsWebDriver("chrome") || !SupportsWebDriver("chromium") {
		t.Error("chrome/chromium 应支持托管 WebDriver")
	}
	if SupportsWebDriver("firefox") || SupportsWebDriver("edge") {
		t.Error("非 Chromium 系不应报告支持托管 WebDriver")
	}
}