package serve

import (
	"testing"
)

func TestDetectPlatformArch(t *testing.T) {
	tests := []struct {
		filename string
		platform string
		arch     string
	}{
		// darwin
		{"bws-darwin-amd64", "darwin", "amd64"},
		{"bws_darwin_amd64", "darwin", "amd64"},
		{"bws-darwin-arm64", "darwin", "arm64"},
		{"bws_darwin_arm64", "darwin", "arm64"},
		{"bws-macos-x64", "darwin", "amd64"},
		{"bws_macos_x64", "darwin", "amd64"},
		{"bws-mac-x64", "darwin", "amd64"},
		{"bws_v1.0.0_darwin_amd64.zip", "darwin", "amd64"},
		{"bws_v1.0.0-beta-29-g48b7a37_darwin_amd64.zip", "darwin", "amd64"},

		// windows
		{"bws-windows-amd64.exe", "windows", "amd64"},
		{"bws_windows_amd64.exe", "windows", "amd64"},
		{"bws-windows-x64.exe", "windows", "amd64"},
		{"bws-windows-arm64.exe", "windows", "arm64"},
		{"bws-v1.0.0-windows-amd64.exe", "windows", "amd64"},
		{"bws_v1.0.0-beta-29-g48b7a37_windows_amd64.zip", "windows", "amd64"},

		// linux
		{"bws-linux-amd64", "linux", "amd64"},
		{"bws_linux_amd64", "linux", "amd64"},
		{"bws-linux-arm64", "linux", "arm64"},
		{"bws-linux-i386", "linux", "386"},
		{"bws-linux-x86", "linux", "386"},
		{"bws_v1.0.0-beta-29-g48b7a37_linux_amd64.zip", "linux", "amd64"},

		// arch edge cases
		{"bws-linux-aarch64", "linux", "arm64"},
		{"bws-linux-x86_64", "linux", "amd64"},
		{"bws-windows-x86.exe", "windows", "386"},
	}

	for _, tt := range tests {
		plat, arch := detectPlatformArch(tt.filename)
		if plat != tt.platform {
			t.Errorf("%s: platform = %q, want %q", tt.filename, plat, tt.platform)
		}
		if arch != tt.arch {
			t.Errorf("%s: arch = %q, want %q", tt.filename, arch, tt.arch)
		}
	}
}
