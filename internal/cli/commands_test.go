package cli

import (
	"bytes"
	"testing"
)

// Mock providers for testing

type mockConfig struct {
	defaultBrowser string
	defaultChannel string
	logLevel       string
	dataDir        string
	configPath     string
	aliases        map[string]string
	repoPath       string
	setRepoPathErr error
}

func (m *mockConfig) DefaultBrowser() string { return m.defaultBrowser }
func (m *mockConfig) SetDefaultBrowser(browser string) error {
	m.defaultBrowser = browser
	return nil
}
func (m *mockConfig) DefaultChannel() string {
	if m.defaultChannel == "" {
		return "stable"
	}
	return m.defaultChannel
}
func (m *mockConfig) SetDefaultChannel(channel string) error {
	m.defaultChannel = channel
	return nil
}
func (m *mockConfig) GetLogLevel() string {
	if m.logLevel == "" {
		return "info"
	}
	return m.logLevel
}
func (m *mockConfig) SetLogLevel(level string) error {
	m.logLevel = level
	return nil
}
func (m *mockConfig) GetDataDir() string {
	if m.dataDir == "" {
		return "/tmp/bm-data"
	}
	return m.dataDir
}
func (m *mockConfig) SetDataDir(path string) error {
	m.dataDir = path
	return nil
}
func (m *mockConfig) ConfigPath() string {
	if m.configPath == "" {
		return "/tmp/bm-data/config.json"
	}
	return m.configPath
}
func (m *mockConfig) GetRemoteSource() string {
	return ""
}
func (m *mockConfig) SetRemoteSource(url string) error {
	return nil
}
func (m *mockConfig) ClearRemoteSource() error {
	return nil
}
func (m *mockConfig) IsServeSourceEnabled() bool     { return true }
func (m *mockConfig) SetServeSourceEnabled(v bool) error { return nil }
func (m *mockConfig) IsOmahaSourceEnabled() bool      { return true }
func (m *mockConfig) SetOmahaSourceEnabled(v bool) error { return nil }
func (m *mockConfig) IsFirefoxFTPEnabled() bool       { return true }
func (m *mockConfig) SetFirefoxFTPEnabled(v bool) error { return nil }
func (m *mockConfig) GetDiskSpaceThresholdGB() int     { return 5 }
func (m *mockConfig) SetDiskSpaceThresholdGB(v int) error { return nil }
func (m *mockConfig) GetProxy() string                    { return "" }
func (m *mockConfig) SetProxy(proxy string) error         { return nil }
func (m *mockConfig) GetAlias(name string) (string, bool) {
	v, ok := m.aliases[name]
	return v, ok
}
func (m *mockConfig) AddAlias(name, target string) error {
	if m.aliases == nil {
		m.aliases = make(map[string]string)
	}
	m.aliases[name] = target
	return nil
}
func (m *mockConfig) RemoveAlias(name string) error {
	delete(m.aliases, name)
	return nil
}
func (m *mockConfig) ListAliases() map[string]string {
	return m.aliases
}
func (m *mockConfig) GetRepoPath() string { return m.repoPath }
func (m *mockConfig) SetRepoPath(path string) error {
	if m.setRepoPathErr != nil {
		return m.setRepoPathErr
	}
	m.repoPath = path
	return nil
}
func (m *mockConfig) GetLanguage() string         { return "zh" }
func (m *mockConfig) SetLanguage(lang string) error { return nil }

type mockBrowsers struct {
	list []BrowserDescriptor
}

func (m *mockBrowsers) Get(name string) BrowserDescriptor {
	for _, b := range m.list {
		if b.Name == name {
			return b
		}
	}
	return BrowserDescriptor{}
}
func (m *mockBrowsers) List() []BrowserDescriptor { return m.list }
func (m *mockBrowsers) Has(name string) bool {
	for _, b := range m.list {
		if b.Name == name {
			return true
		}
	}
	return false
}
func (m *mockBrowsers) ResolveName(name string) (string, bool) {
	for _, b := range m.list {
		if b.Name == name {
			return b.Name, true
		}
	}
	return name, false
}

type mockInstall struct {
	installed map[string]map[string]InstalledVersion // browser -> version -> record
	system    map[string]map[string]InstalledVersion // system browsers
}

func newMockInstall() *mockInstall {
	return &mockInstall{
		installed: make(map[string]map[string]InstalledVersion),
		system:    make(map[string]map[string]InstalledVersion),
	}
}

func (m *mockInstall) add(browser, version string, size int64) {
	if m.installed[browser] == nil {
		m.installed[browser] = make(map[string]InstalledVersion)
	}
	m.installed[browser][version] = InstalledVersion{
		Browser: browser,
		Version: version,
		Size:    size,
	}
}

func (m *mockInstall) addSystem(browser, version, channel string) {
	if m.system[browser] == nil {
		m.system[browser] = make(map[string]InstalledVersion)
	}
	m.system[browser][version] = InstalledVersion{
		Browser:  browser,
		Version:  version,
		Channel:  channel,
		IsSystem: true,
		Source:   "system",
	}
}

func (m *mockInstall) HasSystem() bool {
	return len(m.system) > 0
}

func (m *mockInstall) ListWithSystem() ([]InstalledVersion, error) {
	var result []InstalledVersion
	for _, b := range m.installed {
		for _, v := range b {
			result = append(result, v)
		}
	}
	for _, b := range m.system {
		for _, v := range b {
			result = append(result, v)
		}
	}
	return result, nil
}

func (m *mockInstall) ListWithSystemByBrowser(browser string) ([]InstalledVersion, error) {
	var result []InstalledVersion
	if b, ok := m.installed[browser]; ok {
		for _, v := range b {
			result = append(result, v)
		}
	}
	if b, ok := m.system[browser]; ok {
		for _, v := range b {
			result = append(result, v)
		}
	}
	return result, nil
}

func (m *mockInstall) IsSystemVersion(browser, version string) bool {
	b, ok := m.system[browser]
	if !ok {
		return false
	}
	_, ok = b[version]
	return ok
}

func (m *mockInstall) IsInstalled(browser, version string) bool {
	b, ok := m.installed[browser]
	if !ok {
		return false
	}
	_, ok = b[version]
	return ok
}

func (m *mockInstall) ListInstalled() ([]InstalledVersion, error) {
	var result []InstalledVersion
	for _, b := range m.installed {
		for _, v := range b {
			result = append(result, v)
		}
	}
	return result, nil
}

func (m *mockInstall) ListInstalledByBrowser(browser string) ([]InstalledVersion, error) {
	b, ok := m.installed[browser]
	if !ok {
		return nil, nil
	}
	var result []InstalledVersion
	for _, v := range b {
		result = append(result, v)
	}
	return result, nil
}

func (m *mockInstall) GetRecord(browser, version string) (*InstallRecord, error) {
	if m.IsInstalled(browser, version) {
		return &InstallRecord{
			Browser: browser,
			Version: version,
			Size:    m.installed[browser][version].Size,
		}, nil
	}
	return nil, nil
}

func (m *mockInstall) Uninstall(browser, version string) error {
	if b, ok := m.installed[browser]; ok {
		delete(b, version)
	}
	return nil
}

func (m *mockInstall) InstallFromDir(browser, version, sourceDir string) (*InstallRecord, error) {
	rec := &InstallRecord{
		Browser: browser,
		Version: version,
		Size:    1024 * 1024,
		Source:  "test",
	}
	if m.installed == nil {
		m.installed = make(map[string]map[string]InstalledVersion)
	}
	if m.installed[browser] == nil {
		m.installed[browser] = make(map[string]InstalledVersion)
	}
	m.installed[browser][version] = InstalledVersion{
		Browser: browser,
		Version: version,
		Size:    rec.Size,
	}
	return rec, nil
}

func (m *mockInstall) InstallFromFile(browser, version, filePath string) (*InstallRecord, error) {
	return m.InstallFromDir(browser, version, "")
}

func (m *mockInstall) ImportFromDir(dir string, force bool, onProgress func(current int, total int, message string)) (*ImportSummary, error) {
	// Simple mock: just return empty summary
	return &ImportSummary{
		Total:   0,
		Success: 0,
	}, nil
}

type mockLaunch struct {
	lastOpts LaunchOptions
}

func (m *mockLaunch) Run(opts LaunchOptions) error {
	m.lastOpts = opts
	return nil
}

func (m *mockLaunch) PreviewCommand(opts LaunchOptions) (string, []string, error) {
	return "/fake/path/browser", []string{"--arg", "value"}, nil
}

func setupTestApp(t *testing.T) (*App, *bytes.Buffer, *mockInstall, *mockLaunch) {
	t.Helper()
	var buf bytes.Buffer

	inst := newMockInstall()
	inst.add("chrome", "120.0.6099.109", 842000000)
	inst.add("chrome", "121.0.6167.85", 861000000)
	inst.add("firefox", "121.0", 67000000)

	launcher := &mockLaunch{}

	ctx := &Context{
		Stdout: &buf,
		Stderr: &buf,
		Config: &mockConfig{
			defaultBrowser: "chrome",
			aliases:        map[string]string{"stable": "chrome@120.0.6099.109"},
		},
		Browsers: &mockBrowsers{
			list: []BrowserDescriptor{
				{Name: "chrome", DisplayName: "Google Chrome"},
				{Name: "firefox", DisplayName: "Mozilla Firefox"},
			},
		},
		Install: inst,
		Launch:  launcher,
	}

	app := NewApp("bws", "0.1.0", ctx)
	RegisterCommands(app)

	return app, &buf, inst, launcher
}

func TestLsCommand(t *testing.T) {
	app, buf, _, _ := setupTestApp(t)

	t.Run("list all", func(t *testing.T) {
		buf.Reset()
		err := app.Execute([]string{"ls"})
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		output := buf.String()
		if len(output) == 0 {
			t.Error("no output")
		}
		if !bytes.Contains(buf.Bytes(), []byte("Google Chrome")) {
			t.Errorf("output missing Chrome: %s", output)
		}
		if !bytes.Contains(buf.Bytes(), []byte("Mozilla Firefox")) {
			t.Errorf("output missing Firefox: %s", output)
		}
		if !bytes.Contains(buf.Bytes(), []byte("120.0.6099.109")) {
			t.Errorf("output missing version 120: %s", output)
		}
	})

	t.Run("list by browser", func(t *testing.T) {
		buf.Reset()
		err := app.Execute([]string{"ls", "chrome"})
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		output := buf.String()
		if !bytes.Contains(buf.Bytes(), []byte("120.0.6099.109")) {
			t.Errorf("output missing chrome version: %s", output)
		}
	})

	t.Run("list with alias", func(t *testing.T) {
		buf.Reset()
		err := app.Execute([]string{"list"})
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		if buf.Len() == 0 {
			t.Error("no output for 'list' alias")
		}
	})
}

func TestRunCommand(t *testing.T) {
	app, buf, _, launcher := setupTestApp(t)

	t.Run("run basic", func(t *testing.T) {
		buf.Reset()
		err := app.Execute([]string{"run", "chrome@120.0.6099.109"})
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		if launcher.lastOpts.Browser != "chrome" {
			t.Errorf("browser = %q", launcher.lastOpts.Browser)
		}
		if launcher.lastOpts.Version != "120.0.6099.109" {
			t.Errorf("version = %q", launcher.lastOpts.Version)
		}
	})

	t.Run("run with flags", func(t *testing.T) {
		buf.Reset()
		err := app.Execute([]string{"run", "chrome@120.0.6099.109", "--headless", "--incognito"})
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		if !launcher.lastOpts.Headless {
			t.Error("headless should be true")
		}
		if !launcher.lastOpts.Incognito {
			t.Error("incognito should be true")
		}
	})

	t.Run("run with URLs", func(t *testing.T) {
		buf.Reset()
		err := app.Execute([]string{"run", "chrome@120.0.6099.109", "https://example.com"})
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		if len(launcher.lastOpts.URLs) != 1 || launcher.lastOpts.URLs[0] != "https://example.com" {
			t.Errorf("URLs = %v", launcher.lastOpts.URLs)
		}
	})

	t.Run("run dry-run", func(t *testing.T) {
		buf.Reset()
		err := app.Execute([]string{"run", "chrome@120.0.6099.109", "--dry-run"})
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		if !bytes.Contains(buf.Bytes(), []byte("将要执行:")) {
			t.Errorf("dry-run output: %s", buf.String())
		}
	})

	t.Run("run with extra args after --", func(t *testing.T) {
		buf.Reset()
		err := app.Execute([]string{"run", "chrome@120.0.6099.109", "--", "--custom-flag"})
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		if len(launcher.lastOpts.ExtraArgs) != 1 || launcher.lastOpts.ExtraArgs[0] != "--custom-flag" {
			t.Errorf("ExtraArgs = %v", launcher.lastOpts.ExtraArgs)
		}
	})

	t.Run("run no version specified", func(t *testing.T) {
		buf.Reset()
		err := app.Execute([]string{"run"})
		if err == nil {
			t.Error("should error with no version specified")
		}
	})
}

func TestUninstallCommand(t *testing.T) {
	app, buf, inst, _ := setupTestApp(t)

	t.Run("uninstall existing", func(t *testing.T) {
		buf.Reset()
		err := app.Execute([]string{"uninstall", "chrome@120.0.6099.109"})
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		if inst.IsInstalled("chrome", "120.0.6099.109") {
			t.Error("version should be uninstalled")
		}
		if !bytes.Contains(buf.Bytes(), []byte("已卸载")) {
			t.Errorf("output: %s", buf.String())
		}
	})

	t.Run("uninstall not installed", func(t *testing.T) {
		buf.Reset()
		err := app.Execute([]string{"uninstall", "chrome@999.0.0.0"})
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		if !bytes.Contains(buf.Bytes(), []byte("未安装")) {
			t.Errorf("output: %s", buf.String())
		}
	})
}

func TestAliasCommand(t *testing.T) {
	app, buf, _, _ := setupTestApp(t)

	t.Run("list aliases", func(t *testing.T) {
		buf.Reset()
		err := app.Execute([]string{"alias", "list"})
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		if !bytes.Contains(buf.Bytes(), []byte("stable")) {
			t.Errorf("output missing alias: %s", buf.String())
		}
	})

	t.Run("add alias", func(t *testing.T) {
		buf.Reset()
		err := app.Execute([]string{"alias", "add", "my-chrome", "chrome@120"})
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		if !bytes.Contains(buf.Bytes(), []byte("别名已添加:")) {
			t.Errorf("output: %s", buf.String())
		}
	})

	t.Run("remove alias", func(t *testing.T) {
		buf.Reset()
		err := app.Execute([]string{"alias", "remove", "stable"})
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		if !bytes.Contains(buf.Bytes(), []byte("别名已删除:")) {
			t.Errorf("output: %s", buf.String())
		}
	})
}

func TestUseCommand(t *testing.T) {
	app, buf, _, _ := setupTestApp(t)

	buf.Reset()
	err := app.Execute([]string{"use", "chrome@120.0.6099.109"})
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if !bytes.Contains(buf.Bytes(), []byte("当前使用")) {
		t.Errorf("output: %s", buf.String())
	}
}

func TestParseBrowserVersion(t *testing.T) {
	tests := []struct {
		input          string
		defaultBrowser string
		expectedBrowser string
		expectedVersion string
		expectedAlias   bool
	}{
		{"chrome@120.0.6099.109", "chrome", "chrome", "120.0.6099.109", false},
		{"firefox@beta", "chrome", "firefox", "beta", true},
		{"120", "chrome", "chrome", "120", false},
		{"v121.0", "chrome", "chrome", "v121.0", false},
		{"firefox", "chrome", "firefox", "latest", true},
		{"", "chrome", "chrome", "latest", true},
		{"latest", "chrome", "chrome", "latest", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseBrowserVersion(tt.input, tt.defaultBrowser)
			if result.Browser != tt.expectedBrowser {
				t.Errorf("Browser = %q, want %q", result.Browser, tt.expectedBrowser)
			}
			if result.Version != tt.expectedVersion {
				t.Errorf("Version = %q, want %q", result.Version, tt.expectedVersion)
			}
			if result.IsAlias != tt.expectedAlias {
				t.Errorf("IsAlias = %v, want %v", result.IsAlias, tt.expectedAlias)
			}
		})
	}
}

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"120.0.6099.109", "120.0.6099.109", 0},
		{"121.0.6167.85", "120.0.6099.109", 1},
		{"120.0.6099.109", "121.0.6167.85", -1},
		{"120.0", "120.0.0.0", 0},
	}

	for _, tt := range tests {
		t.Run(tt.a+"_vs_"+tt.b, func(t *testing.T) {
			result := compareVersions(tt.a, tt.b)
			if result != tt.want {
				t.Errorf("compareVersions(%q, %q) = %d, want %d", tt.a, tt.b, result, tt.want)
			}
		})
	}
}

func TestRepoCommand(t *testing.T) {
	app, buf, _, _ := setupTestApp(t)

	t.Run("repo path - no path set", func(t *testing.T) {
		buf.Reset()
		err := app.Execute([]string{"repo", "path"})
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		if !bytes.Contains(buf.Bytes(), []byte("未配置仓库路径")) {
			t.Errorf("expected '未配置仓库路径', got: %s", buf.String())
		}
	})

	t.Run("repo (default shows path)", func(t *testing.T) {
		buf.Reset()
		err := app.Execute([]string{"repo"})
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		if !bytes.Contains(buf.Bytes(), []byte("未配置仓库路径")) {
			t.Errorf("expected '未配置仓库路径', got: %s", buf.String())
		}
	})

	t.Run("repo set", func(t *testing.T) {
		buf.Reset()
		err := app.Execute([]string{"repo", "set", "/tmp/browsers"})
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		if !bytes.Contains(buf.Bytes(), []byte("仓库路径已设置为")) {
			t.Errorf("expected '仓库路径已设置为', got: %s", buf.String())
		}
		// Verify path was set
		cfg := app.Context.Config.(*mockConfig)
		if cfg.repoPath != "/tmp/browsers" {
			t.Errorf("repoPath = %q, want '/tmp/browsers'", cfg.repoPath)
		}
	})

	t.Run("repo set - no path", func(t *testing.T) {
		buf.Reset()
		err := app.Execute([]string{"repo", "set"})
		if err == nil {
			t.Error("expected error when no path provided")
		}
	})

	t.Run("repo path - with path set", func(t *testing.T) {
		// Set a path first
		cfg := app.Context.Config.(*mockConfig)
		cfg.repoPath = "/tmp/test-repo"

		buf.Reset()
		err := app.Execute([]string{"repo", "path"})
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		if !bytes.Contains(buf.Bytes(), []byte("/tmp/test-repo")) {
			t.Errorf("expected path in output, got: %s", buf.String())
		}
	})

	t.Run("repo scan - no repo provider", func(t *testing.T) {
		cfg := app.Context.Config.(*mockConfig)
		cfg.repoPath = "/tmp/test-repo"

		buf.Reset()
		err := app.Execute([]string{"repo", "scan"})
		if err == nil {
			t.Error("expected error when repo provider not available")
		}
	})

	t.Run("repo import - no repo provider", func(t *testing.T) {
		cfg := app.Context.Config.(*mockConfig)
		cfg.repoPath = "/tmp/test-repo"

		buf.Reset()
		err := app.Execute([]string{"repo", "import"})
		if err == nil {
			t.Error("expected error when repo provider not available")
		}
	})
}

// --- System browser CLI tests ---

func setupTestAppWithSystem(t *testing.T) (*App, *bytes.Buffer, *mockInstall, *mockLaunch) {
	t.Helper()
	app, buf, inst, launcher := setupTestApp(t)
	inst.addSystem("chrome", "125.0.6422.112", "stable")
	inst.addSystem("edge", "125.0.2535.67", "stable")
	return app, buf, inst, launcher
}

func TestLsCommand_WithSystem(t *testing.T) {
	app, buf, _, _ := setupTestAppWithSystem(t)

	t.Run("list includes system browsers", func(t *testing.T) {
		buf.Reset()
		err := app.Execute([]string{"ls"})
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		output := buf.String()
		if !bytes.Contains(buf.Bytes(), []byte("[系统]")) {
			t.Errorf("output should contain '[系统]' tag: %s", output)
		}
		if !bytes.Contains(buf.Bytes(), []byte("125.0.6422.112")) {
			t.Errorf("output missing system chrome version: %s", output)
		}
	})

	t.Run("list by browser with system", func(t *testing.T) {
		buf.Reset()
		err := app.Execute([]string{"ls", "chrome"})
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		output := buf.String()
		// Should have both local and system versions
		if !bytes.Contains(buf.Bytes(), []byte("120.0.6099.109")) {
			t.Errorf("missing local version: %s", output)
		}
		if !bytes.Contains(buf.Bytes(), []byte("125.0.6422.112")) {
			t.Errorf("missing system version: %s", output)
		}
	})

	t.Run("list with --no-system", func(t *testing.T) {
		buf.Reset()
		err := app.Execute([]string{"ls", "--no-system"})
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		output := buf.String()
		if bytes.Contains(buf.Bytes(), []byte("125.0.6422.112")) {
			t.Errorf("--no-system should hide system versions: %s", output)
		}
	})

	t.Run("system count in header", func(t *testing.T) {
		buf.Reset()
		err := app.Execute([]string{"ls", "chrome"})
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		output := buf.String()
		if !bytes.Contains(buf.Bytes(), []byte("系统")) {
			t.Errorf("header should include system count: %s", output)
		}
	})
}

func TestRunCommand_WithSystem(t *testing.T) {
	app, buf, _, launcher := setupTestAppWithSystem(t)

	t.Run("run system version", func(t *testing.T) {
		buf.Reset()
		err := app.Execute([]string{"run", "chrome@125.0.6422.112"})
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		if launcher.lastOpts.Browser != "chrome" {
			t.Errorf("browser = %q", launcher.lastOpts.Browser)
		}
		if launcher.lastOpts.Version != "125.0.6422.112" {
			t.Errorf("version = %q", launcher.lastOpts.Version)
		}
	})

	t.Run("run with --native flag", func(t *testing.T) {
		buf.Reset()
		err := app.Execute([]string{"run", "chrome@120.0.6099.109", "--native"})
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		if !launcher.lastOpts.NativeMode {
			t.Error("NativeMode should be true with --native flag")
		}
	})

	t.Run("run chrome@system alias", func(t *testing.T) {
		buf.Reset()
		err := app.Execute([]string{"run", "chrome@system"})
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		if launcher.lastOpts.Version != "125.0.6422.112" {
			t.Errorf("system alias should resolve to stable version, got %q", launcher.lastOpts.Version)
		}
	})
}

func TestUseCommand_WithSystem(t *testing.T) {
	app, buf, _, _ := setupTestAppWithSystem(t)

	t.Run("use system version", func(t *testing.T) {
		buf.Reset()
		err := app.Execute([]string{"use", "chrome@system"})
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		output := buf.String()
		if !bytes.Contains(buf.Bytes(), []byte("当前使用")) {
			t.Errorf("output: %s", output)
		}
		if !bytes.Contains(buf.Bytes(), []byte("125.0.6422.112")) {
			t.Errorf("should resolve system alias to actual version: %s", output)
		}
	})

	t.Run("use specific system version", func(t *testing.T) {
		buf.Reset()
		err := app.Execute([]string{"use", "chrome@125.0.6422.112"})
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		if !bytes.Contains(buf.Bytes(), []byte("chrome@125.0.6422.112")) {
			t.Errorf("output: %s", buf.String())
		}
	})
}

func TestIsVersionAlias_System(t *testing.T) {
	if !isVersionAlias("system") {
		t.Error("'system' should be recognized as a version alias")
	}
	if !isVersionAlias("SYSTEM") {
		t.Error("'SYSTEM' (uppercase) should be recognized as a version alias")
	}
}

func TestInstalledVersion_IsSystem(t *testing.T) {
	v := InstalledVersion{
		Browser:  "chrome",
		Version:  "125.0.0.0",
		IsSystem: true,
		Source:   "system",
	}
	if !v.IsSystem {
		t.Error("IsSystem should be true")
	}
	if v.Source != "system" {
		t.Errorf("Source = %q", v.Source)
	}
}

func TestLaunchOptions_NativeMode(t *testing.T) {
	opts := LaunchOptions{
		Browser:    "chrome",
		Version:    "120.0.0.0",
		NativeMode: true,
	}
	if !opts.NativeMode {
		t.Error("NativeMode should be true")
	}
}

func TestMockInstall_HasSystem(t *testing.T) {
	inst := newMockInstall()
	if inst.HasSystem() {
		t.Error("HasSystem should be false initially")
	}
	inst.addSystem("chrome", "125.0.0.0", "stable")
	if !inst.HasSystem() {
		t.Error("HasSystem should be true after adding system browser")
	}
}

func TestMockInstall_IsSystemVersion(t *testing.T) {
	inst := newMockInstall()
	inst.addSystem("chrome", "125.0.0.0", "stable")

	if !inst.IsSystemVersion("chrome", "125.0.0.0") {
		t.Error("IsSystemVersion should be true for system browser")
	}
	if inst.IsSystemVersion("chrome", "120.0.0.0") {
		t.Error("IsSystemVersion should be false for non-system version")
	}
	if inst.IsSystemVersion("firefox", "125.0.0.0") {
		t.Error("IsSystemVersion should be false for wrong browser")
	}
}

// --- Install command tests ---
