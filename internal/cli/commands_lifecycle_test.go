package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bws/bws/internal/install"
	"github.com/bws/bws/internal/instance"
)

// mockProfile is a ProfileProvider returning deterministic paths.
type mockProfile struct{}

func (m *mockProfile) ProfileDir(browser, version, name string) string {
	base := "/fake/profiles/" + browser + "/" + version
	if name != "" {
		base += "/" + name
	}
	return base
}
func (m *mockProfile) ResetProfile(browser, version, name string) error { return nil }
func (m *mockProfile) ListProfiles(browser string) ([]install.ProfileInfo, error) {
	return nil, nil
}
func (m *mockProfile) CleanOrphanedProfiles(browser string) ([]string, error) { return nil, nil }

// setupLifecycleApp builds an app with separate stdout/stderr buffers and the
// instance/profile/driver providers wired up for the automation lifecycle.
func setupLifecycleApp(t *testing.T) (*App, *bytes.Buffer, *bytes.Buffer, *mockInstance, *mockLaunch, *mockDriver) {
	t.Helper()
	var stdout, stderr bytes.Buffer

	inst := newMockInstall()
	inst.add("chrome", "120.0.6099.109", 842000000)
	inst.add("firefox", "121.0", 67000000)

	launcher := &mockLaunch{}
	mgr := &mockInstance{}
	drv := &mockDriver{}
	cfg := &mockConfig{defaultBrowser: "chrome", aliases: map[string]string{}}

	ctx := &Context{
		Stdout: &stdout,
		Stderr: &stderr,
		Cfg: &Settings{
			Aliases:  cfg,
			Repo:     cfg,
			Defaults: cfg,
			Data:     cfg,
			Proxy:    cfg,
			Source:   cfg,
			Disk:     cfg,
			Log:      cfg,
			Language: cfg,
		},
		Browsers: &mockBrowsers{list: []BrowserDescriptor{
			{Name: "chrome", DisplayName: "Google Chrome"},
			{Name: "firefox", DisplayName: "Mozilla Firefox"},
		}},
		Install:  inst,
		Launch:   launcher,
		Profile:  &mockProfile{},
		Instance: mgr,
		Driver:   drv,
	}

	app := NewApp("bws", "0.1.0", ctx)
	RegisterCommands(app)
	return app, &stdout, &stderr, mgr, launcher, drv
}

// --- ps ---

// CMD-01: empty registry prints the canonical empty message.
func TestPsCommand_Empty(t *testing.T) {
	app, stdout, _, _, _, _ := setupLifecycleApp(t)

	if err := app.Execute([]string{"ps"}); err != nil {
		t.Fatalf("error = %v", err)
	}
	if !strings.Contains(stdout.String(), "No running instances.") {
		t.Errorf("输出 = %q", stdout.String())
	}
}

// CMD-02: JSON output exposes the stable instances[] contract.
func TestPsCommand_JSON(t *testing.T) {
	app, stdout, _, mgr, _, _ := setupLifecycleApp(t)

	cdp := "ws://127.0.0.1:9222/devtools/browser/x"
	mgr.instances = []instance.Instance{{
		Name:    "bws-chrome-120",
		Browser: "chrome",
		Version: "120.0.6099.109",
		PID:     1234,
		Binary:  "/fake/versions/chrome/120.0.6099.109/chrome",
		CDP:     &cdp,
	}}

	if err := app.Execute([]string{"ps", "--json"}); err != nil {
		t.Fatalf("error = %v", err)
	}

	var env struct {
		OK   bool `json:"ok"`
		Data struct {
			Instances []struct {
				Name    string  `json:"name"`
				Browser string  `json:"browser"`
				Version string  `json:"version"`
				PID     int     `json:"pid"`
				Status  string  `json:"status"`
				CDP     *string `json:"cdp"`
			} `json:"instances"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("输出不是合法 JSON: %v\n%s", err, stdout.String())
	}
	if !env.OK || len(env.Data.Instances) != 1 {
		t.Fatalf("信封/实例数不符: %s", stdout.String())
	}
	got := env.Data.Instances[0]
	if got.Name != "bws-chrome-120" || got.Browser != "chrome" || got.PID != 1234 {
		t.Errorf("实例字段不符: %+v", got)
	}
	if got.Status != "running" {
		t.Errorf("status = %q, want running", got.Status)
	}
	if got.CDP == nil || *got.CDP != cdp {
		t.Errorf("cdp = %v", got.CDP)
	}
}

// CMD-02b: human output renders an aligned table.
func TestPsCommand_Table(t *testing.T) {
	app, stdout, _, mgr, _, _ := setupLifecycleApp(t)

	mgr.instances = []instance.Instance{{
		Name:    "bws-chrome-120",
		Browser: "chrome",
		Version: "120.0.6099.109",
		PID:     1234,
	}}

	if err := app.Execute([]string{"ps"}); err != nil {
		t.Fatalf("error = %v", err)
	}
	out := stdout.String()
	for _, want := range []string{"NAME", "BROWSER", "VERSION", "PROFILE", "PID", "CDP", "bws-chrome-120"} {
		if !strings.Contains(out, want) {
			t.Errorf("表格缺少 %q: %s", want, out)
		}
	}
}

// --- stop ---

// CMD-04: stopping a named instance reports success and removes it.
func TestStopCommand_Named(t *testing.T) {
	app, stdout, _, mgr, _, _ := setupLifecycleApp(t)

	mgr.instances = []instance.Instance{{Name: "bws-chrome-120", PID: 1234}}

	if err := app.Execute([]string{"stop", "bws-chrome-120"}); err != nil {
		t.Fatalf("error = %v", err)
	}
	if !strings.Contains(stdout.String(), "Stopped: bws-chrome-120") {
		t.Errorf("输出 = %q", stdout.String())
	}
	if len(mgr.stopped) != 1 || mgr.stopped[0] != "bws-chrome-120" {
		t.Errorf("stopped = %v", mgr.stopped)
	}
	if len(mgr.instances) != 0 {
		t.Errorf("实例应被移除: %v", mgr.instances)
	}
}

// CMD-05: --all stops every registered instance.
func TestStopCommand_All(t *testing.T) {
	app, _, _, mgr, _, _ := setupLifecycleApp(t)

	mgr.instances = []instance.Instance{
		{Name: "bws-chrome-120", PID: 1},
		{Name: "bws-chrome-121", PID: 2},
	}

	if err := app.Execute([]string{"stop", "--all"}); err != nil {
		t.Fatalf("error = %v", err)
	}
	if len(mgr.stopped) != 2 {
		t.Errorf("stopped = %v, 期望 2 个", mgr.stopped)
	}
}

// CMD-06: stopping an unknown instance fails with a non-nil error.
func TestStopCommand_NotFound(t *testing.T) {
	app, _, _, _, _, _ := setupLifecycleApp(t)

	if err := app.Execute([]string{"stop", "nope"}); err == nil {
		t.Fatal("期望错误")
	}
}

// CMD-06b: no target and no --all is an invalid invocation.
func TestStopCommand_NoTarget(t *testing.T) {
	app, _, _, _, _, _ := setupLifecycleApp(t)

	if err := app.Execute([]string{"stop"}); err == nil {
		t.Fatal("期望错误")
	}
}

// CMD-07: an already-exited process is reported distinctly.
func TestStopCommand_AlreadyExited(t *testing.T) {
	app, stdout, _, mgr, _, _ := setupLifecycleApp(t)

	mgr.instances = []instance.Instance{{Name: "bws-chrome-120", PID: 1234}}
	mgr.alreadyExited = true

	if err := app.Execute([]string{"stop", "bws-chrome-120"}); err != nil {
		t.Fatalf("error = %v", err)
	}
	if !strings.Contains(stdout.String(), "Stopped: bws-chrome-120 (already exited)") {
		t.Errorf("输出 = %q", stdout.String())
	}
}

// CMD-04b: JSON stop output exposes stopped/failed arrays.
func TestStopCommand_JSON(t *testing.T) {
	app, stdout, _, mgr, _, _ := setupLifecycleApp(t)

	mgr.instances = []instance.Instance{{Name: "bws-chrome-120", PID: 1234}}

	if err := app.Execute([]string{"stop", "bws-chrome-120", "--json"}); err != nil {
		t.Fatalf("error = %v", err)
	}
	var env struct {
		OK   bool `json:"ok"`
		Data struct {
			Stopped []string `json:"stopped"`
			Failed  []string `json:"failed"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("输出不是合法 JSON: %v", err)
	}
	if !env.OK || len(env.Data.Stopped) != 1 || env.Data.Stopped[0] != "bws-chrome-120" {
		t.Errorf("stopped = %v", env.Data.Stopped)
	}
	if len(env.Data.Failed) != 0 {
		t.Errorf("failed = %v", env.Data.Failed)
	}
}

// --- where ---

// CMD-08: default output is only the binary path (usable as $(bws where ...)).
func TestWhereCommand_Default(t *testing.T) {
	app, stdout, _, _, _, _ := setupLifecycleApp(t)

	if err := app.Execute([]string{"where", "chrome@120.0.6099.109"}); err != nil {
		t.Fatalf("error = %v", err)
	}
	want := "/fake/versions/chrome/120.0.6099.109/chrome\n"
	if stdout.String() != want {
		t.Errorf("输出 = %q, want %q", stdout.String(), want)
	}
}

// CMD-09: --dir returns the containing directory.
func TestWhereCommand_Dir(t *testing.T) {
	app, stdout, _, _, _, _ := setupLifecycleApp(t)

	if err := app.Execute([]string{"where", "chrome@120.0.6099.109", "--dir"}); err != nil {
		t.Fatalf("error = %v", err)
	}
	out := strings.TrimSpace(stdout.String())
	if !strings.Contains(out, "120.0.6099.109") || strings.HasSuffix(out, "chrome") {
		t.Errorf("目录输出不符: %q", out)
	}
}

// CMD-09b: --profile returns the profile directory.
func TestWhereCommand_Profile(t *testing.T) {
	app, stdout, _, _, _, _ := setupLifecycleApp(t)

	if err := app.Execute([]string{"where", "chrome@120.0.6099.109", "--profile"}); err != nil {
		t.Fatalf("error = %v", err)
	}
	want := "/fake/profiles/chrome/120.0.6099.109\n"
	if stdout.String() != want {
		t.Errorf("输出 = %q, want %q", stdout.String(), want)
	}
}

// CMD-10: --json emits the full where contract.
func TestWhereCommand_JSON(t *testing.T) {
	app, stdout, _, _, _, _ := setupLifecycleApp(t)

	if err := app.Execute([]string{"where", "chrome@120.0.6099.109", "--json"}); err != nil {
		t.Fatalf("error = %v", err)
	}
	var env struct {
		OK   bool `json:"ok"`
		Data struct {
			Browser string  `json:"browser"`
			Version string  `json:"version"`
			Binary  string  `json:"binary"`
			Dir     string  `json:"dir"`
			Profile *string `json:"profile"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("输出不是合法 JSON: %v", err)
	}
	if env.Data.Browser != "chrome" || env.Data.Version != "120.0.6099.109" {
		t.Errorf("browser/version = %+v", env.Data)
	}
	if env.Data.Binary == "" || env.Data.Dir == "" {
		t.Errorf("binary/dir 不应为空: %+v", env.Data)
	}
	if env.Data.Profile == nil {
		t.Errorf("profile 应存在（指针字段恒存在）")
	}
}

// CMD-11: an uninstalled version fails.
func TestWhereCommand_NotInstalled(t *testing.T) {
	app, _, _, _, _, _ := setupLifecycleApp(t)

	if err := app.Execute([]string{"where", "chrome@999"}); err == nil {
		t.Fatal("期望错误")
	}
}

// CMD-11b: missing argument fails.
func TestWhereCommand_NoArg(t *testing.T) {
	app, _, _, _, _, _ := setupLifecycleApp(t)

	if err := app.Execute([]string{"where"}); err == nil {
		t.Fatal("期望错误")
	}
}

// --- endpoint ---

// CMD-12: both endpoints are printed.
func TestEndpointCommand_Both(t *testing.T) {
	app, stdout, _, mgr, _, _ := setupLifecycleApp(t)

	cdp := "ws://127.0.0.1:9222/devtools/browser/x"
	wd := "http://127.0.0.1:9515"
	mgr.instances = []instance.Instance{{Name: "bws-chrome-120", CDP: &cdp, WebDriver: &wd}}

	if err := app.Execute([]string{"endpoint", "bws-chrome-120"}); err != nil {
		t.Fatalf("error = %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, cdp) || !strings.Contains(out, wd) {
		t.Errorf("端点输出不符: %q", out)
	}
}

// CMD-12b: JSON endpoint contract keeps null for missing endpoints.
func TestEndpointCommand_JSON(t *testing.T) {
	app, stdout, _, mgr, _, _ := setupLifecycleApp(t)

	mgr.instances = []instance.Instance{{Name: "bws-chrome-120"}}

	if err := app.Execute([]string{"endpoint", "bws-chrome-120", "--json"}); err != nil {
		t.Fatalf("error = %v", err)
	}
	var env struct {
		OK   bool `json:"ok"`
		Data struct {
			Instance  string  `json:"instance"`
			CDP       *string `json:"cdp"`
			WebDriver *string `json:"webdriver"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("输出不是合法 JSON: %v", err)
	}
	if env.Data.Instance != "bws-chrome-120" {
		t.Errorf("instance = %q", env.Data.Instance)
	}
	if env.Data.CDP != nil || env.Data.WebDriver != nil {
		t.Errorf("缺失端点应为 null: %+v", env.Data)
	}
}

// CMD-13: unknown instance fails.
func TestEndpointCommand_NotFound(t *testing.T) {
	app, _, _, _, _, _ := setupLifecycleApp(t)

	if err := app.Execute([]string{"endpoint", "nope"}); err == nil {
		t.Fatal("期望错误")
	}
}

// CMD-13b: missing argument fails.
func TestEndpointCommand_NoArg(t *testing.T) {
	app, _, _, _, _, _ := setupLifecycleApp(t)

	if err := app.Execute([]string{"endpoint"}); err == nil {
		t.Fatal("期望错误")
	}
}

// CMD-13c: a missing instance provider surfaces as an explicit error.
func TestEndpointCommand_NoProvider(t *testing.T) {
	app, _, _, _, _, _ := setupLifecycleApp(t)
	app.Context.Instance = nil

	if err := app.Execute([]string{"endpoint", "bws-chrome-120"}); err == nil {
		t.Fatal("期望错误")
	}
}

// CMD-13d: an instance without endpoints reports it on stderr instead of
// printing nothing (metadata never pollutes stdout).
func TestEndpointCommand_NoEndpoints(t *testing.T) {
	app, stdout, stderr, mgr, _, _ := setupLifecycleApp(t)

	mgr.instances = []instance.Instance{{Name: "bws-chrome-120"}}

	if err := app.Execute([]string{"endpoint", "bws-chrome-120"}); err != nil {
		t.Fatalf("error = %v", err)
	}
	if stdout.String() != "" {
		t.Errorf("stdout 应为空: %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "没有可用的端点") {
		t.Errorf("stderr = %q", stderr.String())
	}
}

// truncateMiddle keeps both ends and collapses the middle.
func TestTruncateMiddle(t *testing.T) {
	cases := []struct {
		in   string
		max  int
		want string
	}{
		{"short", 20, "short"},                       // fits
		{"ws://127.0.0.1:9222/devtools/browser/x", 20, "ws://127.…browser/x"},
		{"abcdefghij", 1, "abcdefghij"},              // max<=1 returns as-is
		{"abcdef", 8, "abcdef"},                      // len <= keep*2
	}
	for _, c := range cases {
		if got := truncateMiddle(c.in, c.max); got != c.want {
			t.Errorf("truncateMiddle(%q,%d) = %q, want %q", c.in, c.max, got, c.want)
		}
	}
}

// --- run (automation) ---

// CMD-14: --automation --json emits the full launch contract with live endpoints.
func TestRunAutomation_JSON(t *testing.T) {
	app, stdout, _, _, launcher, _ := setupLifecycleApp(t)

	// Point the fake browser at a profile dir carrying a CDP port file so
	// discovery succeeds without waiting for the timeout.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "DevToolsActivePort"), []byte("9222\n/devtools/browser/auto\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	launcher.profileDir = dir

	if err := app.Execute([]string{"run", "chrome@120.0.6099.109", "--automation", "--json"}); err != nil {
		t.Fatalf("error = %v", err)
	}

	var env struct {
		OK   bool `json:"ok"`
		Data struct {
			Instance  string  `json:"instance"`
			Browser   string  `json:"browser"`
			Version   string  `json:"version"`
			PID       int     `json:"pid"`
			Binary    string  `json:"binary"`
			Profile   *string `json:"profile"`
			CDP       *string `json:"cdp"`
			WebDriver *string `json:"webdriver"`
			Daemon    bool    `json:"daemon"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("输出不是合法 JSON: %v\n%s", err, stdout.String())
	}
	if !env.OK {
		t.Fatalf("ok 应为 true: %s", stdout.String())
	}
	d := env.Data
	if d.Instance != "bws-chrome-120" || d.Browser != "chrome" || d.PID != 4242 {
		t.Errorf("契约字段不符: %+v", d)
	}
	if d.CDP == nil || *d.CDP != "ws://127.0.0.1:9222/devtools/browser/auto" {
		t.Errorf("cdp = %v", d.CDP)
	}
	if d.WebDriver == nil || *d.WebDriver != "http://127.0.0.1:9515" {
		t.Errorf("webdriver = %v", d.WebDriver)
	}
	if d.Daemon {
		t.Error("daemon 应为 false")
	}
}

// CMD-15: --automation --daemon registers the instance and returns immediately.
func TestRunAutomation_Daemon(t *testing.T) {
	app, _, _, mgr, launcher, _ := setupLifecycleApp(t)

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "DevToolsActivePort"), []byte("9223\n/devtools/browser/auto\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	launcher.profileDir = dir

	if err := app.Execute([]string{"run", "chrome@120.0.6099.109", "--automation", "--daemon"}); err != nil {
		t.Fatalf("error = %v", err)
	}
	if len(mgr.instances) != 1 {
		t.Fatalf("应注册 1 个实例, got %d", len(mgr.instances))
	}
	got := mgr.instances[0]
	if got.Name != "bws-chrome-120" || got.PID != 4242 {
		t.Errorf("实例字段不符: %+v", got)
	}
	if !got.Daemon {
		t.Error("Daemon 应为 true")
	}
	if got.DriverPID != 4321 {
		t.Errorf("DriverPID = %d, 期望 4321", got.DriverPID)
	}
	if got.CDP == nil || *got.CDP != "ws://127.0.0.1:9223/devtools/browser/auto" {
		t.Errorf("cdp = %v", got.CDP)
	}
}

// CMD-16: a plain run keeps the original blocking behaviour (no registry write).
func TestRunCommand_NonAutomationUnchanged(t *testing.T) {
	app, _, _, mgr, launcher, _ := setupLifecycleApp(t)

	if err := app.Execute([]string{"run", "chrome@120.0.6099.109"}); err != nil {
		t.Fatalf("error = %v", err)
	}
	if launcher.lastOpts.Browser != "chrome" || launcher.lastOpts.Version != "120.0.6099.109" {
		t.Errorf("launch opts = %+v", launcher.lastOpts)
	}
	if launcher.lastOpts.Detached {
		t.Error("非自动化 run 不应 detached")
	}
	if len(mgr.instances) != 0 {
		t.Errorf("非自动化 run 不应注册实例: %v", mgr.instances)
	}
}

// CMD-17: combining --automation with a sub-switch is a no-op, not an error.
func TestRunAutomation_SubSwitchEquivalent(t *testing.T) {
	app, stdout, _, _, launcher, _ := setupLifecycleApp(t)

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "DevToolsActivePort"), []byte("9224\n/devtools/browser/auto\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	launcher.profileDir = dir

	if err := app.Execute([]string{"run", "chrome@120.0.6099.109", "--automation", "--cdp", "--json"}); err != nil {
		t.Fatalf("error = %v", err)
	}
	var env struct {
		OK bool `json:"ok"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("输出不是合法 JSON: %v", err)
	}
	if !env.OK {
		t.Error("ok 应为 true")
	}
}

// --json without an automation switch is rejected.
func TestRunCommand_JSONRequiresManaged(t *testing.T) {
	app, _, _, _, _, _ := setupLifecycleApp(t)

	if err := app.Execute([]string{"run", "chrome@120.0.6099.109", "--json"}); err == nil {
		t.Fatal("期望错误：--json 需要与自动化/守护开关同用")
	}
}

// A missing instance provider surfaces as an explicit unsupported error.
func TestPsCommand_NoProvider(t *testing.T) {
	app, _, _, _, _, _ := setupLifecycleApp(t)
	app.Context.Instance = nil

	if err := app.Execute([]string{"ps"}); err == nil {
		t.Fatal("期望错误")
	}
}

// --- ls --json (WP6 migration) ---

// CMD-18: `ls --json` emits the stable installed[] contract.
func TestLsCommand_JSON(t *testing.T) {
	app, stdout, _, _, _, _ := setupLifecycleApp(t)

	if err := app.Execute([]string{"ls", "--json"}); err != nil {
		t.Fatalf("error = %v", err)
	}

	var env struct {
		OK   bool `json:"ok"`
		Data struct {
			Installed []struct {
				Browser string `json:"browser"`
				Version string `json:"version"`
				Type    string `json:"type"`
				Binary  string `json:"binary"`
			} `json:"installed"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("输出不是合法 JSON: %v\n%s", err, stdout.String())
	}
	if !env.OK {
		t.Fatalf("ok 应为 true: %s", stdout.String())
	}
	if len(env.Data.Installed) != 2 {
		t.Fatalf("installed 数量 = %d, 期望 2: %s", len(env.Data.Installed), stdout.String())
	}
	// Sorted by browser asc: chrome before firefox.
	if env.Data.Installed[0].Browser != "chrome" || env.Data.Installed[1].Browser != "firefox" {
		t.Errorf("排序不符: %+v", env.Data.Installed)
	}
	if env.Data.Installed[0].Binary == "" {
		t.Error("binary 不应为空")
	}
	if env.Data.Installed[0].Type != "bws" {
		t.Errorf("type = %q, 期望 bws", env.Data.Installed[0].Type)
	}
}

// CMD-18b: an empty registry still yields an (empty) array, never null.
func TestLsCommand_JSONEmpty(t *testing.T) {
	app, stdout, _, _, _, _ := setupLifecycleApp(t)

	if err := app.Execute([]string{"ls", "chrome@999", "--json"}); err != nil {
		t.Fatalf("error = %v", err)
	}
	if !strings.Contains(stdout.String(), `"installed": []`) {
		t.Errorf("空结果应为数组: %s", stdout.String())
	}
}