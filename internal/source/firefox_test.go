package source

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestFirefoxSource_Name 验证源名称为 firefox-ftp。
func TestFirefoxSource_Name(t *testing.T) {
	src := NewFirefoxSource()
	if name := src.Name(); name != "firefox-ftp" {
		t.Errorf("Name() = %q, want %q", name, "firefox-ftp")
	}
}

// TestFirefoxSource_SupportsBrowser 验证 FirefoxSource 仅支持 firefox。
func TestFirefoxSource_SupportsBrowser(t *testing.T) {
	src := NewFirefoxSource()

	if !src.SupportsBrowser("firefox") {
		t.Error("FirefoxSource should support 'firefox'")
	}
	if !src.SupportsBrowser("Firefox") {
		t.Error("FirefoxSource should support 'Firefox' (case insensitive)")
	}
	if src.SupportsBrowser("chrome") {
		t.Error("FirefoxSource should NOT support 'chrome'")
	}
	if src.SupportsBrowser("chromium") {
		t.Error("FirefoxSource should NOT support 'chromium'")
	}
}

// TestParseDirectoryEntries 验证 HTML 目录列表解析逻辑。
func TestParseDirectoryEntries(t *testing.T) {
	html := `<!DOCTYPE html>
<html>
	<head><title>Directory Listing</title></head>
	<body>
		<h1>Index of /pub/firefox/releases/</h1>
		<table>
			<tr><td>Dir</td><td><a href="/pub/firefox/">..</a></td></tr>
			<tr><td>Dir</td><td><a href="/pub/firefox/releases/141.0/">141.0/</a></td></tr>
			<tr><td>Dir</td><td><a href="/pub/firefox/releases/141.0b1/">141.0b1/</a></td></tr>
			<tr><td>Dir</td><td><a href="/pub/firefox/releases/140.0esr/">140.0esr/</a></td></tr>
			<tr><td>Dir</td><td><a href="/pub/firefox/releases/13.0.1-funnelcake11/">13.0.1-funnelcake11/</a></td></tr>
			<tr><td>File</td><td><a href="/pub/firefox/releases/KEY">KEY</a></td></tr>
		</table>
	</body>
</html>`

	dirs := parseDirectoryEntries(html)

	// 应返回 4 个目录条目（跳过 .. 和文件）
	if len(dirs) != 4 {
		t.Fatalf("parseDirectoryEntries returned %d dirs, want 4", len(dirs))
	}

	expected := []string{"141.0/", "141.0b1/", "140.0esr/", "13.0.1-funnelcake11/"}
	for i, want := range expected {
		if dirs[i] != want {
			t.Errorf("dirs[%d] = %q, want %q", i, dirs[i], want)
		}
	}
}

// TestVersionDirRegex 验证版本目录名过滤正则。
func TestVersionDirRegex(t *testing.T) {
	valid := []string{
		"141.0", "141.0.1", "141.0.3", "128.10.0esr", "140.0esr",
		"141.0b1", "141.0b2", "3.0", "3.6.28", "115.32.1esr",
	}
	invalid := []string{
		"13.0.1-funnelcake11", "1.0rc1", "3.0.16-real",
		"1.cdn_test", "3.6.3plugin1", "0.10rc", "KEY",
		"SHA256SUMS", "source", "update", "jsshell",
	}

	for _, v := range valid {
		if !versionDirRegex.MatchString(v) {
			t.Errorf("版本 %q 应该匹配正则，但未匹配", v)
		}
	}
	for _, v := range invalid {
		if versionDirRegex.MatchString(v) {
			t.Errorf("无效版本 %q 不应匹配正则，但匹配了", v)
		}
	}
}

// TestClassifyFirefoxChannel 验证渠道分类逻辑。
func TestClassifyFirefoxChannel(t *testing.T) {
	tests := []struct {
		version string
		want    Channel
	}{
		{"141.0", ChannelStable},
		{"141.0.1", ChannelStable},
		{"140.0esr", ChannelESR},
		{"128.10.0esr", ChannelESR},
		{"141.0b1", ChannelBeta},
		{"141.0b9", ChannelBeta},
		{"3.6.28", ChannelStable},
		{"115.32.1esr", ChannelESR},
	}
	for _, tt := range tests {
		got := classifyFirefoxChannel(tt.version)
		if got != tt.want {
			t.Errorf("classifyFirefoxChannel(%q) = %v, want %v", tt.version, got, tt.want)
		}
	}
}

// TestBuildDownloadURL 验证下载 URL 拼接。
func TestBuildDownloadURL(t *testing.T) {
	src := NewFirefoxSource()

	tests := []struct {
		name     string
		version  string
		platform Platform
		arch     Arch
		want     string
	}{
		{
			name:     "Windows x64 stable",
			version:  "141.0",
			platform: PlatformWindows,
			arch:     ArchAMD64,
			want:     "https://ftp.mozilla.org/pub/firefox/releases/141.0/win64/zh-CN/Firefox%20Setup%20141.0.exe",
		},
		{
			name:     "Windows x64 ESR",
			version:  "140.0esr",
			platform: PlatformWindows,
			arch:     ArchAMD64,
			want:     "https://ftp.mozilla.org/pub/firefox/releases/140.0esr/win64/zh-CN/Firefox%20Setup%20140.0esr.exe",
		},
		{
			name:     "Linux x64 stable",
			version:  "141.0",
			platform: PlatformLinux,
			arch:     ArchAMD64,
			want:     "https://ftp.mozilla.org/pub/firefox/releases/141.0/linux-x86_64/zh-CN/firefox-141.0.tar.bz2",
		},
		{
			name:     "macOS stable",
			version:  "141.0",
			platform: PlatformMacOS,
			arch:     ArchAMD64,
			want:     "https://ftp.mozilla.org/pub/firefox/releases/141.0/mac/zh-CN/Firefox%20141.0.dmg",
		},
		{
			name:     "Windows ARM64",
			version:  "141.0",
			platform: PlatformWindows,
			arch:     ArchARM64,
			want:     "https://ftp.mozilla.org/pub/firefox/releases/141.0/win64-aarch64/zh-CN/Firefox%20Setup%20141.0.exe",
		},
		{
			name:     "Linux ARM64",
			version:  "141.0",
			platform: PlatformLinux,
			arch:     ArchARM64,
			want:     "https://ftp.mozilla.org/pub/firefox/releases/141.0/linux-aarch64/zh-CN/firefox-141.0.tar.bz2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := src.buildDownloadURL(tt.version, tt.platform, tt.arch)
			if got != tt.want {
				t.Errorf("buildDownloadURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestFirefoxSource_List 验证使用模拟 HTML 目录列表的版本列表获取。
func TestFirefoxSource_List(t *testing.T) {
	// 模拟 FTP 目录列表 HTML
	directoryHTML := `<!DOCTYPE html>
<html><body>
<table>
<tr><td>Dir</td><td><a href="/pub/firefox/">..</a></td></tr>
<tr><td>Dir</td><td><a href="/pub/firefox/releases/141.0/">141.0/</a></td></tr>
<tr><td>Dir</td><td><a href="/pub/firefox/releases/141.0.1/">141.0.1/</a></td></tr>
<tr><td>Dir</td><td><a href="/pub/firefox/releases/141.0b1/">141.0b1/</a></td></tr>
<tr><td>Dir</td><td><a href="/pub/firefox/releases/140.0esr/">140.0esr/</a></td></tr>
<tr><td>Dir</td><td><a href="/pub/firefox/releases/13.0.1-funnelcake11/">13.0.1-funnelcake11/</a></td></tr>
<tr><td>File</td><td><a href="/pub/firefox/releases/KEY">KEY</a></td></tr>
</table>
</body></html>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(directoryHTML))
	}))
	defer server.Close()

	src := &FirefoxSource{
		baseURL:    server.URL + "/",
		httpClient: server.Client(),
	}

	ctx := context.Background()
	versions, err := src.List(ctx, &Filter{Browser: "firefox"})
	if err != nil {
		t.Fatalf("List 失败: %v", err)
	}

	// 应返回 4 个有效版本（过滤掉 funnelcake）
	if len(versions) != 4 {
		t.Fatalf("返回 %d 个版本，期望 4 个", len(versions))
	}

	// 验证按降序排序：141.0.1 > 141.0 > 141.0b1 > 140.0esr
	expected := []string{"141.0.1", "141.0", "141.0b1", "140.0esr"}
	for i, want := range expected {
		if versions[i].Version != want {
			t.Errorf("versions[%d].Version = %q, want %q", i, versions[i].Version, want)
		}
	}

	// 验证渠道分类
	channelMap := map[string]Channel{
		"141.0.1":  ChannelStable,
		"141.0":    ChannelStable,
		"141.0b1":  ChannelBeta,
		"140.0esr": ChannelESR,
	}
	for _, v := range versions {
		if want, ok := channelMap[v.Version]; ok {
			if v.Channel != want {
				t.Errorf("版本 %s 渠道 = %v, 期望 %v", v.Version, v.Channel, want)
			}
		}
	}
}

// TestFirefoxSource_List_ChannelFilter 验证渠道过滤。
func TestFirefoxSource_List_ChannelFilter(t *testing.T) {
	directoryHTML := `<!DOCTYPE html>
<html><body>
<table>
<tr><td>Dir</td><td><a href="/pub/firefox/releases/141.0/">141.0/</a></td></tr>
<tr><td>Dir</td><td><a href="/pub/firefox/releases/141.0b1/">141.0b1/</a></td></tr>
<tr><td>Dir</td><td><a href="/pub/firefox/releases/140.0esr/">140.0esr/</a></td></tr>
</table>
</body></html>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(directoryHTML))
	}))
	defer server.Close()

	src := &FirefoxSource{
		baseURL:    server.URL + "/",
		httpClient: server.Client(),
	}

	ctx := context.Background()

	// 仅 ESR
	versions, err := src.List(ctx, &Filter{Browser: "firefox", Channel: ChannelESR})
	if err != nil {
		t.Fatalf("List 失败: %v", err)
	}
	if len(versions) != 1 {
		t.Fatalf("ESR 过滤返回 %d 个版本，期望 1 个", len(versions))
	}
	if versions[0].Version != "140.0esr" {
		t.Errorf("ESR 版本 = %q, 期望 %q", versions[0].Version, "140.0esr")
	}

	// 仅 Beta
	versions, err = src.List(ctx, &Filter{Browser: "firefox", Channel: ChannelBeta})
	if err != nil {
		t.Fatalf("List 失败: %v", err)
	}
	if len(versions) != 1 {
		t.Fatalf("Beta 过滤返回 %d 个版本，期望 1 个", len(versions))
	}
	if versions[0].Version != "141.0b1" {
		t.Errorf("Beta 版本 = %q, 期望 %q", versions[0].Version, "141.0b1")
	}
}

// TestFirefoxSource_Resolve_Exact 验证精确版本匹配。
func TestFirefoxSource_Resolve_Exact(t *testing.T) {
	directoryHTML := `<!DOCTYPE html>
<html><body>
<table>
<tr><td>Dir</td><td><a href="/pub/firefox/releases/141.0/">141.0/</a></td></tr>
<tr><td>Dir</td><td><a href="/pub/firefox/releases/141.0.1/">141.0.1/</a></td></tr>
<tr><td>Dir</td><td><a href="/pub/firefox/releases/140.0esr/">140.0esr/</a></td></tr>
</table>
</body></html>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(directoryHTML))
	}))
	defer server.Close()

	src := &FirefoxSource{
		baseURL:    server.URL + "/",
		httpClient: server.Client(),
	}

	ctx := context.Background()
	v, err := src.Resolve(ctx, "firefox", "141.0", PlatformWindows, ArchAMD64)
	if err != nil {
		t.Fatalf("Resolve 失败: %v", err)
	}
	if v.Version != "141.0" {
		t.Errorf("版本 = %q, 期望 %q", v.Version, "141.0")
	}
}

// TestFirefoxSource_Resolve_Prefix 验证前缀匹配返回最高版本。
func TestFirefoxSource_Resolve_Prefix(t *testing.T) {
	directoryHTML := `<!DOCTYPE html>
<html><body>
<table>
<tr><td>Dir</td><td><a href="/pub/firefox/releases/141.0/">141.0/</a></td></tr>
<tr><td>Dir</td><td><a href="/pub/firefox/releases/141.0.1/">141.0.1/</a></td></tr>
<tr><td>Dir</td><td><a href="/pub/firefox/releases/141.0.2/">141.0.2/</a></td></tr>
<tr><td>Dir</td><td><a href="/pub/firefox/releases/140.0esr/">140.0esr/</a></td></tr>
</table>
</body></html>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(directoryHTML))
	}))
	defer server.Close()

	src := &FirefoxSource{
		baseURL:    server.URL + "/",
		httpClient: server.Client(),
	}

	ctx := context.Background()
	// "141" 应匹配 141.0.2（最高）
	v, err := src.Resolve(ctx, "firefox", "141", PlatformWindows, ArchAMD64)
	if err != nil {
		t.Fatalf("Resolve 失败: %v", err)
	}
	if v.Version != "141.0.2" {
		t.Errorf("前缀匹配版本 = %q, 期望 %q", v.Version, "141.0.2")
	}
}

// TestFirefoxSource_Resolve_NotFound 验证版本不存在时返回错误。
func TestFirefoxSource_Resolve_NotFound(t *testing.T) {
	directoryHTML := `<!DOCTYPE html>
<html><body>
<table>
<tr><td>Dir</td><td><a href="/pub/firefox/releases/141.0/">141.0/</a></td></tr>
</table>
</body></html>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(directoryHTML))
	}))
	defer server.Close()

	src := &FirefoxSource{
		baseURL:    server.URL + "/",
		httpClient: server.Client(),
	}

	ctx := context.Background()
	_, err := src.Resolve(ctx, "firefox", "999.0", PlatformWindows, ArchAMD64)
	if err == nil {
		t.Fatal("期望返回错误，但成功了")
	}
}

// TestFirefoxSource_Latest 验证获取最新版本。
func TestFirefoxSource_Latest(t *testing.T) {
	directoryHTML := `<!DOCTYPE html>
<html><body>
<table>
<tr><td>Dir</td><td><a href="/pub/firefox/releases/141.0/">141.0/</a></td></tr>
<tr><td>Dir</td><td><a href="/pub/firefox/releases/141.0.1/">141.0.1/</a></td></tr>
<tr><td>Dir</td><td><a href="/pub/firefox/releases/140.0esr/">140.0esr/</a></td></tr>
</table>
</body></html>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(directoryHTML))
	}))
	defer server.Close()

	src := &FirefoxSource{
		baseURL:    server.URL + "/",
		httpClient: server.Client(),
	}

	ctx := context.Background()

	// 默认渠道 (stable)
	v, err := src.Latest(ctx, &Filter{Browser: "firefox"})
	if err != nil {
		t.Fatalf("Latest 失败: %v", err)
	}
	if v.Version != "141.0.1" {
		t.Errorf("最新 stable 版本 = %q, 期望 %q", v.Version, "141.0.1")
	}

	// ESR 渠道
	v, err = src.Latest(ctx, &Filter{Browser: "firefox", Channel: ChannelESR})
	if err != nil {
		t.Fatalf("Latest ESR 失败: %v", err)
	}
	if v.Version != "140.0esr" {
		t.Errorf("最新 ESR 版本 = %q, 期望 %q", v.Version, "140.0esr")
	}
}

// TestParseFileEntries 验证从 HTML 目录列表中提取文件名（非目录）。
func TestParseFileEntries(t *testing.T) {
	html := `<!DOCTYPE html>
<html><body>
<table>
<tr><td>Dir</td><td><a href="/pub/firefox/">..</a></td></tr>
<tr><td>Dir</td><td><a href="/pub/firefox/releases/141.0/">141.0/</a></td></tr>
<tr><td>File</td><td><a href="/pub/firefox/releases/141.0/linux-x86_64/zh-CN/firefox-141.0.tar.xz">firefox-141.0.tar.xz</a></td></tr>
<tr><td>File</td><td><a href="/pub/firefox/releases/141.0/linux-x86_64/zh-CN/firefox-141.0.deb">firefox-141.0.deb</a></td></tr>
<tr><td>File</td><td><a href="/pub/firefox/releases/KEY">KEY</a></td></tr>
</table>
</body></html>`

	files := parseFileEntries(html)

	// 应返回 3 个文件条目（跳过 .. 和目录）
	if len(files) != 3 {
		t.Fatalf("parseFileEntries returned %d files, want 3", len(files))
	}

	expected := []string{"firefox-141.0.tar.xz", "firefox-141.0.deb", "KEY"}
	for i, want := range expected {
		if files[i] != want {
			t.Errorf("files[%d] = %q, want %q", i, files[i], want)
		}
	}
}

// TestParseSums 验证校验文件解析逻辑（SHA256SUMS 格式）。
func TestParseSums(t *testing.T) {
	content := `abc123def4567890abc123def4567890abc123def4567890abc123def4567890  linux-x86_64/zh-CN/firefox-141.0.tar.xz
fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210  win64/zh-CN/Firefox Setup 141.0.exe
# this is a comment line

short  linux-x86_64/zh-CN/invalid.tar.xz
0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef  mac/zh-CN/Firefox 141.0.dmg
`

	sums := parseSums(content, 64)

	// 应返回 3 个有效条目（跳过注释、空行和短哈希行）
	if len(sums) != 3 {
		t.Fatalf("parseSums returned %d entries, want 3", len(sums))
	}

	// 验证路径到哈希的映射
	wantHash := "abc123def4567890abc123def4567890abc123def4567890abc123def4567890"
	if h, ok := sums["linux-x86_64/zh-CN/firefox-141.0.tar.xz"]; !ok {
		t.Error("missing linux-x86_64/zh-CN/firefox-141.0.tar.xz")
	} else if h != wantHash {
		t.Errorf("hash for firefox-141.0.tar.xz = %q, want %q", h, wantHash)
	}

	wantHash2 := "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"
	if h, ok := sums["win64/zh-CN/Firefox Setup 141.0.exe"]; !ok {
		t.Error("missing win64/zh-CN/Firefox Setup 141.0.exe")
	} else if h != wantHash2 {
		t.Errorf("hash for Firefox Setup 141.0.exe = %q, want %q", h, wantHash2)
	}

	// 验证短哈希行被跳过
	if _, ok := sums["linux-x86_64/zh-CN/invalid.tar.xz"]; ok {
		t.Error("短哈希行不应被解析")
	}
}

// TestFindMatchingFile 验证文件匹配逻辑。
func TestFindMatchingFile(t *testing.T) {
	files := []string{
		"firefox-141.0.tar.xz",
		"firefox-141.0.tar.bz2",
		"firefox-141.0.deb",
		"Firefox Setup 141.0.exe",
		"Firefox Setup 141.0.msi",
		"Firefox 141.0.dmg",
		"KEY",
	}

	tests := []struct {
		name     string
		version  string
		platform Platform
		want     string
	}{
		{"Linux bz2 preferred", "141.0", PlatformLinux, "firefox-141.0.tar.bz2"},
		{"Windows exe preferred", "141.0", PlatformWindows, "Firefox Setup 141.0.exe"},
		{"macOS dmg", "141.0", PlatformMacOS, "Firefox 141.0.dmg"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findMatchingFile(files, tt.version, tt.platform)
			if got != tt.want {
				t.Errorf("findMatchingFile() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestFindMatchingFile_XzFallback 验证当 .tar.bz2 不存在时回退到 .tar.xz。
func TestFindMatchingFile_XzFallback(t *testing.T) {
	files := []string{
		"firefox-141.0.tar.xz",
		"firefox-141.0.deb",
	}
	got := findMatchingFile(files, "141.0", PlatformLinux)
	if got != "firefox-141.0.tar.xz" {
		t.Errorf("findMatchingFile() = %q, want %q", got, "firefox-141.0.tar.xz")
	}
}

// TestFindMatchingFile_NotFound 验证未找到匹配文件时返回空字符串。
func TestFindMatchingFile_NotFound(t *testing.T) {
	files := []string{"KEY", "some-other-file.txt"}
	got := findMatchingFile(files, "141.0", PlatformLinux)
	if got != "" {
		t.Errorf("findMatchingFile() = %q, want empty string", got)
	}
}

// TestResolveDownloadURL 验证 ResolveDownloadURL 从 FTP 目录列表获取真实文件名和 SHA256。
func TestResolveDownloadURL(t *testing.T) {
	// 模拟 FTP 目录结构和 SHA256SUMS 文件
	fileListingHTML := `<!DOCTYPE html>
<html><body>
<table>
<tr><td>Dir</td><td><a href="/pub/firefox/">..</a></td></tr>
<tr><td>File</td><td><a href="firefox-141.0.tar.xz">firefox-141.0.tar.xz</a></td></tr>
<tr><td>File</td><td><a href="firefox-141.0.deb">firefox-141.0.deb</a></td></tr>
</table>
</body></html>`

	sha256Content := `abc123def4567890abc123def4567890abc123def4567890abc123def4567890  linux-x86_64/zh-CN/firefox-141.0.tar.xz
fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210  win64/zh-CN/Firefox Setup 141.0.exe
`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if strings.HasSuffix(path, "/SHA256SUMS") {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(sha256Content))
			return
		}
		// 目录列表请求
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fileListingHTML))
	}))
	defer server.Close()

	src := &FirefoxSource{
		baseURL:    server.URL + "/pub/firefox/releases/",
		httpClient: server.Client(),
	}

	ctx := context.Background()
	dlURL, err := src.ResolveDownloadURL(ctx, "141.0", PlatformLinux, ArchAMD64)
	if err != nil {
		t.Fatalf("ResolveDownloadURL 失败: %v", err)
	}

	// 验证 URL 使用了真实文件名（.tar.xz）
	expectedURL := server.URL + "/pub/firefox/releases/141.0/linux-x86_64/zh-CN/firefox-141.0.tar.xz"
	if dlURL != expectedURL {
		t.Errorf("dlURL = %q, want %q", dlURL, expectedURL)
	}

	// 验证 GetChecksum 返回正确的带算法前缀的校验和
	checksum, err := src.GetChecksum(ctx, "141.0", PlatformLinux, ArchAMD64)
	if err != nil {
		t.Fatalf("GetChecksum 失败: %v", err)
	}
	expectedSHA := "sha256:abc123def4567890abc123def4567890abc123def4567890abc123def4567890"
	if checksum != expectedSHA {
		t.Errorf("checksum = %q, want %q", checksum, expectedSHA)
	}
}

// TestResolveDownloadURL_Cache 验证 URL 缓存机制。
func TestResolveDownloadURL_Cache(t *testing.T) {
	requestCount := 0
	fileListingHTML := `<!DOCTYPE html>
<html><body>
<table>
<tr><td>File</td><td><a href="firefox-141.0.tar.xz">firefox-141.0.tar.xz</a></td></tr>
</table>
</body></html>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if strings.HasSuffix(r.URL.Path, "/SHA256SUMS") {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef  linux-x86_64/zh-CN/firefox-141.0.tar.xz\n"))
			return
		}
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fileListingHTML))
	}))
	defer server.Close()

	src := &FirefoxSource{
		baseURL:    server.URL + "/pub/firefox/releases/",
		httpClient: server.Client(),
	}

	ctx := context.Background()

	// 第一次调用：发起网络请求
	_, err := src.ResolveDownloadURL(ctx, "141.0", PlatformLinux, ArchAMD64)
	if err != nil {
		t.Fatalf("第一次 ResolveDownloadURL 失败: %v", err)
	}
	firstCount := requestCount

	// 第二次调用：应命中缓存，不再发起网络请求
	_, err = src.ResolveDownloadURL(ctx, "141.0", PlatformLinux, ArchAMD64)
	if err != nil {
		t.Fatalf("第二次 ResolveDownloadURL 失败: %v", err)
	}
	if requestCount != firstCount {
		t.Errorf("第二次调用应命中缓存，但发起了额外请求: first=%d, second=%d", firstCount, requestCount)
	}
}

// TestResolveDownloadURL_SHA256NotFound 验证 SHA256SUMS 不存在时优雅处理。
func TestResolveDownloadURL_SHA256NotFound(t *testing.T) {
	fileListingHTML := `<!DOCTYPE html>
<html><body>
<table>
<tr><td>File</td><td><a href="firefox-141.0.tar.xz">firefox-141.0.tar.xz</a></td></tr>
</table>
</body></html>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/SHA256SUMS") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fileListingHTML))
	}))
	defer server.Close()

	src := &FirefoxSource{
		baseURL:    server.URL + "/pub/firefox/releases/",
		httpClient: server.Client(),
	}

	ctx := context.Background()
	dlURL, err := src.ResolveDownloadURL(ctx, "141.0", PlatformLinux, ArchAMD64)
	if err != nil {
		t.Fatalf("ResolveDownloadURL 失败: %v", err)
	}
	// URL 仍应正确
	if !strings.HasSuffix(dlURL, "firefox-141.0.tar.xz") {
		t.Errorf("dlURL = %q, should end with firefox-141.0.tar.xz", dlURL)
	}
}

// TestResolveDownloadURL_FallbackToPattern 验证目录列表不可用时回退到模式构造。
func TestResolveDownloadURL_FallbackToPattern(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 目录列表返回 404
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	src := &FirefoxSource{
		baseURL:    server.URL + "/pub/firefox/releases/",
		httpClient: server.Client(),
	}

	ctx := context.Background()
	dlURL, err := src.ResolveDownloadURL(ctx, "141.0", PlatformLinux, ArchAMD64)
	if err != nil {
		t.Fatalf("ResolveDownloadURL 失败: %v", err)
	}
	// 应回退到模式构造的 .tar.bz2 文件名
	if !strings.HasSuffix(dlURL, "firefox-141.0.tar.bz2") {
		t.Errorf("dlURL = %q, should end with firefox-141.0.tar.bz2 (pattern fallback)", dlURL)
	}
}

// TestResolveDownloadURL_Bz2File 验证旧版本使用 .tar.bz2 时的文件名解析。
func TestResolveDownloadURL_Bz2File(t *testing.T) {
	fileListingHTML := `<!DOCTYPE html>
<html><body>
<table>
<tr><td>File</td><td><a href="firefox-68.9.0esr.tar.bz2">firefox-68.9.0esr.tar.bz2</a></td></tr>
<tr><td>File</td><td><a href="firefox-68.9.0esr.deb">firefox-68.9.0esr.deb</a></td></tr>
</table>
</body></html>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/SHA256SUMS") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fileListingHTML))
	}))
	defer server.Close()

	src := &FirefoxSource{
		baseURL:    server.URL + "/pub/firefox/releases/",
		httpClient: server.Client(),
	}

	ctx := context.Background()
	dlURL, err := src.ResolveDownloadURL(ctx, "68.9.0esr", PlatformLinux, ArchAMD64)
	if err != nil {
		t.Fatalf("ResolveDownloadURL 失败: %v", err)
	}
	// 应使用 .tar.bz2 而非 .tar.xz
	if !strings.HasSuffix(dlURL, "firefox-68.9.0esr.tar.bz2") {
		t.Errorf("dlURL = %q, should end with firefox-68.9.0esr.tar.bz2", dlURL)
	}
}

// TestGetChecksum_Cache 验证 sumsCache 缓存机制：
// 同一版本的第二次调用应命中缓存，不发起额外 HTTP 请求。
func TestGetChecksum_Cache(t *testing.T) {
	requestCount := 0
	fileListingHTML := `<!DOCTYPE html>
<html><body>
<table>
<tr><td>File</td><td><a href="firefox-141.0.tar.xz">firefox-141.0.tar.xz</a></td></tr>
</table>
</body></html>`

	sha256Content := `abc123def4567890abc123def4567890abc123def4567890abc123def4567890  linux-x86_64/zh-CN/firefox-141.0.tar.xz
fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210  win64/zh-CN/Firefox Setup 141.0.exe
`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if strings.HasSuffix(r.URL.Path, "/SHA256SUMS") {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(sha256Content))
			return
		}
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fileListingHTML))
	}))
	defer server.Close()

	src := &FirefoxSource{
		baseURL:    server.URL + "/pub/firefox/releases/",
		httpClient: server.Client(),
	}

	ctx := context.Background()

	// 第一次调用：从网络获取 SHA256SUMS
	chk1, err := src.GetChecksum(ctx, "141.0", PlatformLinux, ArchAMD64)
	if err != nil {
		t.Fatalf("第一次 GetChecksum 失败: %v", err)
	}
	if chk1 != "sha256:abc123def4567890abc123def4567890abc123def4567890abc123def4567890" {
		t.Errorf("chk1 = %q, want sha256:abc123...", chk1)
	}
	firstCount := requestCount

	// 第二次调用：同一版本不同平台，应命中 sumsCache，不发起 SHA256SUMS 请求
	chk2, err := src.GetChecksum(ctx, "141.0", PlatformWindows, ArchAMD64)
	if err != nil {
		t.Fatalf("第二次 GetChecksum 失败: %v", err)
	}
	if chk2 != "sha256:fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210" {
		t.Errorf("chk2 = %q, want sha256:fedcba...", chk2)
	}
	// 最多只多出一次 ResolveDownloadURL 的文件列表请求（URL缓存可能已命中）
	if requestCount > firstCount+1 {
		t.Errorf("第二次调用应命中 sumsCache，请求数: first=%d, second=%d", firstCount, requestCount)
	}
}

// TestGetChecksum_EmptyWhenNotFound 验证 SHA256SUMS 不存在时返回空字符串。
func TestGetChecksum_EmptyWhenNotFound(t *testing.T) {
	fileListingHTML := `<!DOCTYPE html>
<html><body>
<table>
<tr><td>File</td><td><a href="firefox-141.0.tar.xz">firefox-141.0.tar.xz</a></td></tr>
</table>
</body></html>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/SHA256SUMS") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/SHA1SUMS") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fileListingHTML))
	}))
	defer server.Close()

	src := &FirefoxSource{
		baseURL:    server.URL + "/pub/firefox/releases/",
		httpClient: server.Client(),
	}

	ctx := context.Background()
	chk, err := src.GetChecksum(ctx, "141.0", PlatformLinux, ArchAMD64)
	if err != nil {
		t.Fatalf("GetChecksum 失败: %v", err)
	}
	if chk != "" {
		t.Errorf("checksum = %q, want empty string when checksum files not found", chk)
	}
}
