package browser

// Chromium is the descriptor for Chromium.
var Chromium = &BrowserDescriptor{
	Name:        "chromium",
	DisplayName: "Chromium",
	Icon:        "🔵",

	ExecutableCandidates: map[string]map[string][]string{
		"windows": {
			"amd64": {"chrome.exe", "chromium.exe"},
			"386":   {"chrome.exe"},
		},
		"darwin": {
			"amd64": {"Chromium.app/Contents/MacOS/Chromium"},
			"arm64": {"Chromium.app/Contents/MacOS/Chromium"},
		},
		"linux": {
			"amd64": {"chromium", "chromium-browser", "chrome"},
		},
	},

	ProfileArg:      "--user-data-dir=",
	ProfileSeparate: false,

	MultiInstanceArgs: []string{
		"--no-default-browser-check",
		"--no-first-run",
	},
	DisableUpdateArgs: []string{},
	FirstRunSkipArgs:  []string{"--no-first-run"},

	PackageFormats: []string{"zip", "tar.gz", "tar.bz2"},
	Channels:       []string{"stable", "beta", "dev", "canary"},
	DefaultChannel: "stable",
	VersionSegments: 4,

	Features: BrowserFeatures{
		SupportsHeadless:  true,
		SupportsIncognito: true,
		SupportsProfile:   true,
		CanMultiInstance:  true,
		HasUserDirArg:     true,
	},
}
