package install

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/bws/bws/internal/browser"
	"github.com/bws/bws/internal/paths"
)

const (
	testChromeAppID = "{8A69D345-D564-463C-AFF1-A69D9E530F96}"
	testChromeGUID  = "{082c5e9d-abd2-4bf7-ae3a-94b8f9bbb1da}"
)

func chromeExeName() string {
	if runtime.GOOS == "windows" {
		return "chrome.exe"
	}
	return "chrome"
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func zipTestDir(t *testing.T, srcDir, zipPath string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(zipPath), 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	w := zip.NewWriter(f)
	defer w.Close()

	err = filepath.WalkDir(srcDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		fw, err := w.Create(filepath.ToSlash(rel))
		if err != nil {
			return err
		}
		src, err := os.Open(path)
		if err != nil {
			return err
		}
		defer src.Close()
		_, err = io.Copy(fw, src)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}

// buildChromeOfflineInstaller 构造新版 Chrome 离线安装包（外层包）：
//
//	bin/Offline/{GUID}/OfflineManifest.gup
//	bin/Offline/{GUID}/{appid}/148.0.7778.179_chrome_installer.exe  (zip 载荷)
//
// 返回外层压缩包路径。
func buildChromeOfflineInstaller(t *testing.T, workDir string) string {
	t.Helper()

	pkgRoot := filepath.Join(workDir, "pkg")
	appDir := filepath.Join(pkgRoot, "bin", "Offline", testChromeGUID)

	// 真实载荷（zip 内容）：模拟 chrome_installer.exe 解压得到的 Chrome-bin
	payloadSrc := filepath.Join(workDir, "payload-src", "Chrome-bin")
	if err := os.MkdirAll(payloadSrc, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(payloadSrc, chromeExeName()), []byte("fake-chrome"), 0o755); err != nil {
		t.Fatal(err)
	}
	zipTestDir(t, filepath.Dir(payloadSrc), filepath.Join(appDir, testChromeAppID, "148.0.7778.179_chrome_installer.exe"))

	manifest := `<?xml version="1.0" encoding="UTF-8"?>
<response protocol="3.0">
  <app appid="` + testChromeAppID + `" status="ok">
    <updatecheck status="ok">
      <manifest version="148.0.7778.179">
        <packages>
          <package name="148.0.7778.179_chrome_installer.exe" size="139546024" required="true"/>
        </packages>
      </manifest>
    </updatecheck>
  </app>
</response>`
	writeTestFile(t, filepath.Join(appDir, "OfflineManifest.gup"), manifest)

	// 干扰文件：不应被误当作载荷
	writeTestFile(t, filepath.Join(pkgRoot, "bin", "updater.exe"), "not-an-archive")
	writeTestFile(t, filepath.Join(pkgRoot, "bin", "uninstall.cmd"), "del /f /q")

	outerZip := filepath.Join(workDir, "offline-installer.zip")
	zipTestDir(t, pkgRoot, outerZip)
	return outerZip
}

// TestInstallFromFile_ChromeOfflineLayout 覆盖新版 Chrome 离线包从解压到安装的完整链路。
func TestInstallFromFile_ChromeOfflineLayout(t *testing.T) {
	root := t.TempDir()
	p := paths.New(root)
	if err := p.EnsureAll(); err != nil {
		t.Fatal(err)
	}

	reg := browser.NewRegistry()
	reg.Register(browser.Chrome)
	m := NewManager(p, reg)

	outerZip := buildChromeOfflineInstaller(t, t.TempDir())

	record, err := m.InstallFromFile("chrome", "149.0.7827.22", outerZip)
	if err != nil {
		t.Fatalf("InstallFromFile 失败: %v", err)
	}
	if record.ExecutablePath != chromeExeName() {
		t.Errorf("ExecutablePath = %q, 期望 %q", record.ExecutablePath, chromeExeName())
	}

	exePath, err := m.GetExecutablePath("chrome", "149.0.7827.22")
	if err != nil {
		t.Fatalf("GetExecutablePath 失败: %v", err)
	}
	if _, err := os.Stat(exePath); err != nil {
		t.Errorf("安装后的可执行文件不存在: %s", exePath)
	}
	if !m.IsInstalled("chrome", "149.0.7827.22") {
		t.Error("安装记录未写入")
	}
}
