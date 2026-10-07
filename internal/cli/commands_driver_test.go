package cli

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/bws/bws/internal/driver"
)

// mockDriver is a DriverProvider that records calls for assertions.
type mockDriver struct {
	info         *driver.Info
	resolveErr   error
	records      []driver.Record
	listErr      error
	ensureRec    *driver.Record
	ensureErr    error
	ensureCalls  []string
	uninstalled  []string
	uninstallErr error
	startOpts    []driver.StartOptions
	startProc    *driver.Process
	startErr     error
}

func (m *mockDriver) Resolve(chromeVersion string) (*driver.Info, error) {
	if m.resolveErr != nil {
		return nil, m.resolveErr
	}
	if m.info != nil {
		return m.info, nil
	}
	return &driver.Info{
		Name:         driver.NameChromedriver,
		Version:      chromeVersion,
		MajorVersion: "120",
		Platform:     "windows",
		Arch:         "amd64",
		DownloadURL:  "https://example.test/chromedriver-win64.zip",
		Filename:     "chromedriver-win64.zip",
		Source:       "test",
	}, nil
}

func (m *mockDriver) Ensure(chromeVersion string, force bool, _ func(int64, int64, float64)) (*driver.Record, error) {
	m.ensureCalls = append(m.ensureCalls, chromeVersion)
	if m.ensureErr != nil {
		return nil, m.ensureErr
	}
	if m.ensureRec != nil {
		return m.ensureRec, nil
	}
	return &driver.Record{
		Name:         driver.NameChromedriver,
		Version:      chromeVersion,
		MajorVersion: "120",
		Dir:          "/tmp/drivers/chromedriver/120",
	}, nil
}

func (m *mockDriver) List() ([]driver.Record, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.records, nil
}

func (m *mockDriver) Uninstall(major string) error {
	m.uninstalled = append(m.uninstalled, major)
	return m.uninstallErr
}

func (m *mockDriver) Start(opts driver.StartOptions) (*driver.Process, error) {
	m.startOpts = append(m.startOpts, opts)
	if m.startErr != nil {
		return nil, m.startErr
	}
	if m.startProc != nil {
		return m.startProc, nil
	}
	return &driver.Process{Pid: 4321, Port: 9515, URL: "http://127.0.0.1:9515"}, nil
}

// setupDriverApp builds a test app whose driver provider is drv and whose
// stdin is fed the given text (used for confirmation prompts).
func setupDriverApp(t *testing.T, drv DriverProvider, stdin string) (*App, *bytes.Buffer) {
	t.Helper()
	var buf bytes.Buffer

	inst := newMockInstall()
	inst.add("chrome", "120.0.6099.109", 842000000)

	cfg := &mockConfig{defaultBrowser: "chrome", aliases: map[string]string{}}

	ctx := &Context{
		Stdout: &buf,
		Stderr: &buf,
		Stdin:  strings.NewReader(stdin),
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
		Browsers: &mockBrowsers{
			list: []BrowserDescriptor{{Name: "chrome", DisplayName: "Google Chrome"}},
		},
		Install: inst,
		Launch:  &mockLaunch{},
		Driver:  drv,
	}

	app := NewApp("bws", "0.1.0", ctx)
	RegisterCommands(app)
	return app, &buf
}

func TestDriverLs_Empty(t *testing.T) {
	app, buf := setupDriverApp(t, &mockDriver{}, "")

	if err := app.Execute([]string{"driver", "ls"}); err != nil {
		t.Fatalf("error = %v", err)
	}
	if !strings.Contains(buf.String(), "未安装任何驱动") {
		t.Errorf("输出缺少空提示: %s", buf.String())
	}
}

func TestDriverLs_WithRecords(t *testing.T) {
	drv := &mockDriver{records: []driver.Record{{
		Name:         driver.NameChromedriver,
		MajorVersion: "120",
		Version:      "120.0.6099.109",
		Platform:     "windows",
		Arch:         "amd64",
		Size:         1024,
		InstalledAt:  time.Date(2026, 1, 2, 3, 4, 0, 0, time.UTC),
	}}}
	app, buf := setupDriverApp(t, drv, "")

	if err := app.Execute([]string{"driver", "ls"}); err != nil {
		t.Fatalf("error = %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "120.0.6099.109") {
		t.Errorf("输出缺少版本: %s", out)
	}
	if !strings.Contains(out, "windows/amd64") {
		t.Errorf("输出缺少平台: %s", out)
	}
}

func TestDriverInstall_DryRun(t *testing.T) {
	app, buf := setupDriverApp(t, &mockDriver{}, "")

	if err := app.Execute([]string{"driver", "install", "120", "--dry-run"}); err != nil {
		t.Fatalf("error = %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "将安装") {
		t.Errorf("输出缺少解析信息: %s", out)
	}
	if !strings.Contains(out, "https://example.test/chromedriver-win64.zip") {
		t.Errorf("输出缺少下载地址: %s", out)
	}
}

func TestDriverInstall_NoArg(t *testing.T) {
	app, _ := setupDriverApp(t, &mockDriver{}, "")

	err := app.Execute([]string{"driver", "install"})
	if err == nil {
		t.Fatal("缺少版本参数时应报错")
	}
	if !strings.Contains(err.Error(), "请指定 Chrome 版本") {
		t.Errorf("error = %v", err)
	}
}

func TestDriverInstall_UnsupportedBrowser(t *testing.T) {
	app, _ := setupDriverApp(t, &mockDriver{}, "")

	err := app.Execute([]string{"driver", "install", "firefox@120"})
	if err == nil {
		t.Fatal("非 Chrome 浏览器应报错")
	}
	if !strings.Contains(err.Error(), "仅支持 Chrome/Chromium") {
		t.Errorf("error = %v", err)
	}
}

func TestDriverUninstall_Force(t *testing.T) {
	drv := &mockDriver{}
	app, buf := setupDriverApp(t, drv, "")

	if err := app.Execute([]string{"driver", "uninstall", "120", "--force"}); err != nil {
		t.Fatalf("error = %v", err)
	}
	if len(drv.uninstalled) != 1 || drv.uninstalled[0] != "120" {
		t.Fatalf("uninstalled = %v, 期望 [120]", drv.uninstalled)
	}
	if !strings.Contains(buf.String(), "已卸载 chromedriver 120") {
		t.Errorf("输出缺少卸载确认: %s", buf.String())
	}
}

func TestDriverUninstall_Cancelled(t *testing.T) {
	drv := &mockDriver{}
	app, buf := setupDriverApp(t, drv, "n\n")

	if err := app.Execute([]string{"driver", "uninstall", "120"}); err != nil {
		t.Fatalf("error = %v", err)
	}
	if len(drv.uninstalled) != 0 {
		t.Errorf("取消后不应卸载, got %v", drv.uninstalled)
	}
	if !strings.Contains(buf.String(), "已取消") {
		t.Errorf("输出缺少取消提示: %s", buf.String())
	}
}

func TestDriverStart_Detach(t *testing.T) {
	drv := &mockDriver{}
	app, buf := setupDriverApp(t, drv, "")

	if err := app.Execute([]string{"driver", "start", "120", "--detach", "--port", "9515"}); err != nil {
		t.Fatalf("error = %v", err)
	}
	if len(drv.startOpts) != 1 {
		t.Fatalf("startOpts = %v", drv.startOpts)
	}
	if drv.startOpts[0].Major != "120" {
		t.Errorf("Major = %q, 期望 120", drv.startOpts[0].Major)
	}
	if drv.startOpts[0].Port != 9515 {
		t.Errorf("Port = %d, 期望 9515", drv.startOpts[0].Port)
	}
	if !drv.startOpts[0].Detach {
		t.Error("Detach = false, 期望 true")
	}
	if !strings.Contains(buf.String(), "http://127.0.0.1:9515") {
		t.Errorf("输出缺少 WebDriver 端点: %s", buf.String())
	}
}

func TestDriverDisabled(t *testing.T) {
	app, _ := setupDriverApp(t, nil, "")

	err := app.Execute([]string{"driver", "ls"})
	if err == nil {
		t.Fatal("未注入驱动提供者时应报错")
	}
	if !strings.Contains(err.Error(), "当前构建不支持驱动管理功能") {
		t.Errorf("error = %v", err)
	}
}
