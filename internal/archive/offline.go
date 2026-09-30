package archive

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// offlineManifestName 是 Google 更新器（updater）离线安装包中的清单文件名。
//
// 新版 Chrome 离线安装包（GoogleChrome_<version>_Windows_x64_Offline.exe）的
// 结构与旧版不同，解压后得到的是 updater.7z 而不是 chrome.7z：
//
//	<安装包>.exe
//	└── updater.7z
//	    └── bin/
//	        ├── updater.exe
//	        ├── uninstall.cmd
//	        └── Offline/{安装器 GUID}/
//	            ├── OfflineManifest.gup        ← 清单，记录 appid
//	            └── {appid}/
//	                └── <xxx>_chrome_installer.exe  ← 再解压得到 chrome.7z
//
// 因此仅靠扩展名扫描无法发现真实载荷，需要先解析 OfflineManifest.gup 中的
// appid，再进入同名目录解压其中的安装器。
const offlineManifestName = "OfflineManifest.gup"

// offlineManifest 描述 OfflineManifest.gup 中我们关心的部分。
type offlineManifest struct {
	Apps []offlineManifestApp `xml:"app"`
}

// offlineManifestApp 是清单中的一个应用条目，AppID 即存放载荷的目录名。
type offlineManifestApp struct {
	AppID    string `xml:"appid,attr"`
	Packages []struct {
		Name string `xml:"name,attr"`
	} `xml:"updatecheck>manifest>packages>package"`
}

// parseOfflineManifest 解析 OfflineManifest.gup，返回其中带 appid 的应用条目。
func parseOfflineManifest(path string) ([]offlineManifestApp, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var m offlineManifest
	if err := xml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("解析 %s 失败: %w", filepath.Base(path), err)
	}

	var apps []offlineManifestApp
	for _, app := range m.Apps {
		if strings.TrimSpace(app.AppID) != "" {
			apps = append(apps, app)
		}
	}
	return apps, nil
}

// findOfflinePayloads 扫描 rootDir 下的 OfflineManifest.gup，定位其 appid 目录，
// 返回该目录中仍需解压的载荷文件。
func findOfflinePayloads(rootDir string) ([]string, error) {
	var manifests []string
	err := filepath.WalkDir(rootDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.EqualFold(d.Name(), offlineManifestName) {
			manifests = append(manifests, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool)
	var payloads []string
	for _, manifestPath := range manifests {
		apps, err := parseOfflineManifest(manifestPath)
		if err != nil {
			continue
		}
		for _, app := range apps {
			dir := locateAppIDDir(filepath.Dir(manifestPath), rootDir, app.AppID)
			if dir == "" {
				continue
			}
			for _, payload := range offlinePayloadFiles(dir, app) {
				if !seen[payload] {
					seen[payload] = true
					payloads = append(payloads, payload)
				}
			}
		}
	}
	return payloads, nil
}

// locateAppIDDir 查找名为 appID 的目录。优先在清单所在目录内查找，
// 找不到时再回退到整个解压根目录。
func locateAppIDDir(manifestDir, rootDir, appID string) string {
	for _, root := range []string{manifestDir, rootDir} {
		var found string
		_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil || found != "" {
				return nil
			}
			if d.IsDir() && strings.EqualFold(d.Name(), appID) {
				found = path
				return filepath.SkipAll
			}
			return nil
		})
		if found != "" {
			return found
		}
	}
	return ""
}

// offlinePayloadFiles 返回 appid 目录中需要解压的载荷文件。
// 优先采用清单中声明的包名，未匹配到时回退为目录下的压缩包/安装器。
func offlinePayloadFiles(dir string, app offlineManifestApp) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var declared, fallback []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()

		for _, pkg := range app.Packages {
			if strings.EqualFold(name, pkg.Name) {
				declared = append(declared, filepath.Join(dir, name))
				break
			}
		}
		if isExtractablePayload(name) {
			fallback = append(fallback, filepath.Join(dir, name))
		}
	}

	if len(declared) > 0 {
		return declared
	}
	return fallback
}

// isExtractablePayload 判断文件名是否可能是浏览器载荷包。
func isExtractablePayload(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasSuffix(lower, ".7z") ||
		strings.HasSuffix(lower, ".exe") ||
		strings.HasSuffix(lower, ".zip")
}

// extractOfflinePayload 解压离线安装包中的载荷，并在成功后移除源文件。
func extractOfflinePayload(srcPath string) error {
	extractDir := deriveExtractDir(srcPath)
	if _, err := os.Stat(extractDir); err == nil {
		extractDir = extractDir + "_extracted"
	}

	if err := extractWithoutExecuting(srcPath, extractDir); err != nil {
		return err
	}

	if err := os.Remove(srcPath); err != nil {
		_ = os.Rename(srcPath, srcPath+".extracted")
	}
	return nil
}

// extractWithoutExecuting 只使用纯 Go 的解压方式提取载荷，绝不执行安装器。
// 离线安装包内的 exe 是 7z 自解压包，直接运行它可能会触发真实的系统安装。
func extractWithoutExecuting(srcPath, destDir string) error {
	if strings.EqualFold(filepath.Ext(srcPath), ".exe") {
		if isZipFile(srcPath) {
			return extractZip(srcPath, destDir)
		}
		return extractEmbedded7z(srcPath, destDir)
	}
	return Extract(srcPath, destDir)
}
