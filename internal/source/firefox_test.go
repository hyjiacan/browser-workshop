package source

import (
	"context"
	"net/http"
	"net/http/httptest"
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
			name:    "Windows x64 stable",
			version: "141.0",
			platform: PlatformWindows,
			arch:    ArchAMD64,
			want:    "https://ftp.mozilla.org/pub/firefox/releases/141.0/win64/en-US/Firefox%20Setup%20141.0.exe",
		},
		{
			name:    "Windows x64 ESR",
			version: "140.0esr",
			platform: PlatformWindows,
			arch:    ArchAMD64,
			want:    "https://ftp.mozilla.org/pub/firefox/releases/140.0esr/win64/en-US/Firefox%20Setup%20140.0esr.exe",
		},
		{
			name:    "Linux x64 stable",
			version: "141.0",
			platform: PlatformLinux,
			arch:    ArchAMD64,
			want:    "https://ftp.mozilla.org/pub/firefox/releases/141.0/linux-x86_64/en-US/firefox-141.0.tar.xz",
		},
		{
			name:    "macOS stable",
			version: "141.0",
			platform: PlatformMacOS,
			arch:    ArchAMD64,
			want:    "https://ftp.mozilla.org/pub/firefox/releases/141.0/mac/en-US/Firefox%20141.0.dmg",
		},
		{
			name:    "Windows ARM64",
			version: "141.0",
			platform: PlatformWindows,
			arch:    ArchARM64,
			want:    "https://ftp.mozilla.org/pub/firefox/releases/141.0/win64-aarch64/en-US/Firefox%20Setup%20141.0.exe",
		},
		{
			name:    "Linux ARM64",
			version: "141.0",
			platform: PlatformLinux,
			arch:    ArchARM64,
			want:    "https://ftp.mozilla.org/pub/firefox/releases/141.0/linux-aarch64/en-US/firefox-141.0.tar.xz",
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
