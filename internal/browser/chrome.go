package browser

// Chrome is the descriptor for Google Chrome.
var Chrome = &BrowserDescriptor{
	Name:        "chrome",
	DisplayName: "Google Chrome",
	Icon:        "🌐",

	ExecutableCandidates: map[string]map[string][]string{
		"windows": {
			"amd64": {"chrome.exe", "Google Chrome.exe"},
			"386":   {"chrome.exe"},
		},
		"darwin": {
			"amd64": {"Google Chrome.app/Contents/MacOS/Google Chrome"},
			"arm64": {"Google Chrome.app/Contents/MacOS/Google Chrome"},
		},
		"linux": {
			"amd64": {"chrome", "google-chrome", "google-chrome-stable"},
		},
	},

	ProfileArg:      "--user-data-dir=",
	ProfileSeparate: false,

	MultiInstanceArgs: []string{
		"--no-default-browser-check",
		"--no-first-run",
	},
	DisableUpdateArgs: []string{"--disable-update"},
	FirstRunSkipArgs:  []string{"--no-first-run"},

	PackageFormats: []string{"zip", "exe", "msi"},
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

// IncognitoArg returns the incognito mode flag for Chrome.
func (d *BrowserDescriptor) IncognitoArg() string {
	switch d.Name {
	case "chrome", "chromium":
		return "--incognito"
	case "firefox":
		return "-private"
	default:
		return "--incognito"
	}
}

// HeadlessArgs returns the headless mode flags for the browser.
func (d *BrowserDescriptor) HeadlessArgs() []string {
	switch d.Name {
	case "chrome", "chromium":
		return []string{"--headless", "--disable-gpu"}
	case "firefox":
		return []string{"-headless"}
	default:
		return []string{"--headless"}
	}
}
