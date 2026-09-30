package archive

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

const (
	testOfflineAppID = "{8A69D345-D564-463C-AFF1-A69D9E530F96}"
	testOfflineGUID  = "{082c5e9d-abd2-4bf7-ae3a-94b8f9bbb1da}"
)

// offlineManifestXML 生成与 Google 更新器离线安装包一致的 OfflineManifest.gup 内容。
func offlineManifestXML(appID, pkgName string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<response protocol="3.0">
  <systemrequirements platform="win" arch="x64" min_os_version="6.1"/>
  <app appid="` + appID + `" status="ok">
    <updatecheck status="ok">
      <urls>
        <url codebase="http://dl.google.com/edgedl/chrome/install/148.0.7778.179/"/>
      </urls>
      <manifest version="148.0.7778.179">
        <packages>
          <package name="` + pkgName + `" hash_sha256="deadbeef" size="139546024" required="true"/>
        </packages>
      </manifest>
    </updatecheck>
  </app>
</response>`
}

// writeFile 写入文件并自动创建父目录。
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("创建目录失败: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("写入文件失败: %v", err)
	}
}

// zipDir 将目录内容打包为 zip，条目使用相对路径。
func zipDir(t *testing.T, srcDir, zipPath string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(zipPath), 0o755); err != nil {
		t.Fatalf("创建 zip 目录失败: %v", err)
	}
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("创建 zip 失败: %v", err)
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
		t.Fatalf("打包 zip 失败: %v", err)
	}
}

// buildChromeOfflineTree 构造新版 Chrome 离线包解压后的目录结构：
//
//	<root>/bin/Offline/{GUID}/OfflineManifest.gup
//	<root>/bin/Offline/{GUID}/{appid}/<installerName>
//
// installerName 为一个 zip 格式的自解压包，内含 Chrome-bin/chrome.exe。
func buildChromeOfflineTree(t *testing.T, root, installerName string) {
	t.Helper()

	appDir := filepath.Join(root, "bin", "Offline", testOfflineGUID)

	// 真实载荷：zip 内容，模拟 chrome_installer.exe 解压后得到的 Chrome-bin
	payloadSrc := filepath.Join(t.TempDir(), "payload-src", "Chrome-bin")
	if err := os.MkdirAll(payloadSrc, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(payloadSrc, exeName("chrome")), []byte("fake-chrome-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	zipDir(t, filepath.Dir(payloadSrc), filepath.Join(appDir, testOfflineAppID, installerName))

	// 清单
	writeFile(t, filepath.Join(appDir, offlineManifestName), offlineManifestXML(testOfflineAppID, installerName))

	// 干扰文件：不应被当作载荷解压
	writeFile(t, filepath.Join(root, "bin", "updater.exe"), "not-an-archive")
	writeFile(t, filepath.Join(root, "bin", "uninstall.cmd"), "del /f /q")
}

func TestParseOfflineManifest(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, offlineManifestName)
	writeFile(t, path, offlineManifestXML(testOfflineAppID, "chrome_installer.exe"))

	apps, err := parseOfflineManifest(path)
	if err != nil {
		t.Fatalf("parseOfflineManifest 失败: %v", err)
	}
	if len(apps) != 1 {
		t.Fatalf("期望 1 个 app，实际 %d", len(apps))
	}
	if apps[0].AppID != testOfflineAppID {
		t.Errorf("appid = %q, 期望 %q", apps[0].AppID, testOfflineAppID)
	}
	if len(apps[0].Packages) != 1 || apps[0].Packages[0].Name != "chrome_installer.exe" {
		t.Errorf("packages = %+v, 期望 [chrome_installer.exe]", apps[0].Packages)
	}
}

func TestParseOfflineManifest_InvalidXML(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, offlineManifestName)
	writeFile(t, path, "this is not xml")

	if _, err := parseOfflineManifest(path); err == nil {
		t.Error("非法 XML 应当返回错误")
	}
}

func TestFindOfflinePayloads(t *testing.T) {
	root := t.TempDir()
	buildChromeOfflineTree(t, root, "148.0.7778.179_chrome_installer.exe")

	payloads, err := findOfflinePayloads(root)
	if err != nil {
		t.Fatalf("findOfflinePayloads 失败: %v", err)
	}
	if len(payloads) != 1 {
		t.Fatalf("期望 1 个载荷，实际 %d: %v", len(payloads), payloads)
	}
	want := filepath.Join(root, "bin", "Offline", testOfflineGUID, testOfflineAppID, "148.0.7778.179_chrome_installer.exe")
	if payloads[0] != want {
		t.Errorf("载荷 = %q, 期望 %q", payloads[0], want)
	}
}

func TestFindOfflinePayloads_FallbackToArchiveInAppDir(t *testing.T) {
	root := t.TempDir()
	appDir := filepath.Join(root, "bin", "Offline", testOfflineGUID, testOfflineAppID)
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// 清单声明的包名与实际存在的文件不一致 → 回退到目录下的压缩包
	writeFile(t, filepath.Join(filepath.Dir(appDir), offlineManifestName),
		offlineManifestXML(testOfflineAppID, "chrome_installer.exe"))
	writeFile(t, filepath.Join(appDir, "chrome.7z"), "placeholder")

	payloads, err := findOfflinePayloads(root)
	if err != nil {
		t.Fatalf("findOfflinePayloads 失败: %v", err)
	}
	if len(payloads) != 1 || filepath.Base(payloads[0]) != "chrome.7z" {
		t.Fatalf("期望回退到 chrome.7z，实际: %v", payloads)
	}
}

func TestFindOfflinePayloads_NoManifest(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "bin", "updater.exe"), "not-an-archive")

	payloads, err := findOfflinePayloads(root)
	if err != nil {
		t.Fatalf("findOfflinePayloads 失败: %v", err)
	}
	if len(payloads) != 0 {
		t.Errorf("无清单时不应发现载荷，实际: %v", payloads)
	}
}

// TestExtractRecursive_ChromeOfflineLayout 覆盖新版 Chrome 离线安装包的嵌套结构：
// 外层包 → bin/Offline/{GUID}/OfflineManifest.gup → {appid}/<installer>.exe → Chrome-bin/chrome.exe
func TestExtractRecursive_ChromeOfflineLayout(t *testing.T) {
	tmpDir := t.TempDir()

	pkgRoot := filepath.Join(tmpDir, "pkg")
	buildChromeOfflineTree(t, pkgRoot, "148.0.7778.179_chrome_installer.exe")

	outerZip := filepath.Join(tmpDir, "offline-installer.zip")
	zipDir(t, pkgRoot, outerZip)

	destDir := filepath.Join(tmpDir, "dest")
	if _, err := ExtractRecursive(outerZip, destDir); err != nil {
		t.Fatalf("ExtractRecursive 失败: %v", err)
	}

	// 载荷安装器应已被解压并移除
	installerPath := filepath.Join(destDir, "bin", "Offline", testOfflineGUID, testOfflineAppID,
		"148.0.7778.179_chrome_installer.exe")
	if _, err := os.Stat(installerPath); !os.IsNotExist(err) {
		t.Errorf("载荷安装器应已被解压并移除: %s", installerPath)
	}

	// chrome.exe 应能被找到（层级很深，超过旧的 3 层限制）
	contentDir, err := FindContentDir(destDir, "chrome", runtime.GOOS, runtime.GOARCH, []string{exeName("chrome")})
	if err != nil {
		t.Fatalf("FindContentDir 失败: %v", err)
	}
	chromeExe := filepath.Join(contentDir, exeName("chrome"))
	if _, err := os.Stat(chromeExe); os.IsNotExist(err) {
		t.Errorf("chrome.exe 未找到: %s", chromeExe)
	}
}

// TestExtractRecursive_ChromeOfflineLayout_7z 使用真实的 updater.7z 层级验证。
func TestExtractRecursive_ChromeOfflineLayout_7z(t *testing.T) {
	if !Has7z() {
		t.Skip("7z 不可用，跳过")
	}

	tmpDir := t.TempDir()

	// updater 内容目录（含 bin/ 子目录）
	updaterContent := filepath.Join(tmpDir, "updater-content")
	buildChromeOfflineTree(t, updaterContent, "148.0.7778.179_chrome_installer.exe")

	// 打包为 updater.7z
	updater7z := filepath.Join(tmpDir, "updater.7z")
	if err := create7z(updater7z, updaterContent); err != nil {
		t.Fatalf("创建 updater.7z 失败: %v", err)
	}

	// 外层包：包含 updater.7z
	outerSrc := filepath.Join(tmpDir, "outer-src")
	if err := os.MkdirAll(outerSrc, 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(updater7z)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outerSrc, "updater.7z"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	outerZip := filepath.Join(tmpDir, "offline.zip")
	zipDir(t, outerSrc, outerZip)

	destDir := filepath.Join(tmpDir, "dest")
	if _, err := ExtractRecursive(outerZip, destDir); err != nil {
		t.Fatalf("ExtractRecursive 失败: %v", err)
	}

	contentDir, err := FindContentDir(destDir, "chrome", runtime.GOOS, runtime.GOARCH, []string{exeName("chrome")})
	if err != nil {
		t.Fatalf("FindContentDir 失败: %v", err)
	}
	if _, err := os.Stat(filepath.Join(contentDir, exeName("chrome"))); os.IsNotExist(err) {
		t.Errorf("chrome.exe 未找到于 %s", contentDir)
	}
}

// TestExtractWithoutExecuting_RejectsNonArchiveExe 确保离线载荷中的 exe
// 不会被当作安装器执行——既不是 zip 也不是 7z 时必须报错。
func TestExtractWithoutExecuting_RejectsNonArchiveExe(t *testing.T) {
	tmpDir := t.TempDir()
	exePath := filepath.Join(tmpDir, "chrome_installer.exe")
	if err := os.WriteFile(exePath, []byte("MZ this is not an archive"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := extractWithoutExecuting(exePath, filepath.Join(tmpDir, "out")); err == nil {
		t.Error("非压缩格式的 exe 应当提取失败，而不是执行安装器")
	}
}

// TestFindBrowserExe_DeepNesting 验证可执行文件位于深层目录时仍能定位。
func TestFindBrowserExe_DeepNesting(t *testing.T) {
	root := t.TempDir()

	deep := filepath.Join(root, "a", "b", "c", "d", "e", "f", "g", "Chrome-bin")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(deep, exeName("chrome")), []byte("fake"), 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := FindBrowserExe(root, "chrome", runtime.GOOS, runtime.GOARCH, []string{exeName("chrome")})
	if err != nil {
		t.Fatalf("FindBrowserExe 失败: %v", err)
	}
	if got != deep {
		t.Errorf("FindBrowserExe = %q, 期望 %q", got, deep)
	}
}

// TestIsExtractablePayload 校验载荷候选判定。
func TestIsExtractablePayload(t *testing.T) {
	cases := map[string]bool{
		"chrome.7z":            true,
		"chrome_installer.exe": true,
		"payload.zip":          true,
		"uninstall.cmd":        false,
		"OfflineManifest.gup":  false,
		"CHROME.7Z":            true,
		"chrome_installer.EXE": true,
	}
	for name, want := range cases {
		if got := isExtractablePayload(name); got != want {
			t.Errorf("isExtractablePayload(%q) = %v, 期望 %v", name, got, want)
		}
	}
}
