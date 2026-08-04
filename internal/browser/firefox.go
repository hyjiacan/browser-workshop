package browser

// Firefox is the descriptor for Mozilla Firefox.
var Firefox = &BrowserDescriptor{
	Name:        "firefox",
	DisplayName: "Mozilla Firefox",
	Icon:        "🦊",

	ExecutableCandidates: map[string]map[string][]string{
		"windows": {
			"amd64": {"firefox.exe"},
			"386":   {"firefox.exe"},
		},
		"darwin": {
			"amd64": {"Firefox.app/Contents/MacOS/firefox"},
			"arm64": {"Firefox.app/Contents/MacOS/firefox"},
		},
		"linux": {
			"amd64": {"firefox", "firefox-esr"},
		},
	},

	ProfileArg:      "-profile",
	ProfileSeparate: true,

	MultiInstanceArgs: []string{"-no-remote"},
	DisableUpdateArgs: []string{},
	FirstRunSkipArgs:  []string{},

	// Firefox does not support command-line flags for disabling updates or
	// default-browser checks. These preferences are written to user.js in the
	// profile directory at launch time.
	StandardPrefs: []string{
		`user_pref("app.update.enabled", false);`,
		`user_pref("app.update.auto", false);`,
		`user_pref("app.update.doorhanger", false);`,
		`user_pref("app.update.silent", false);`,
		`user_pref("browser.shell.checkDefaultBrowser", false);`,
	},

	PackageFormats: []string{"zip", "tar.bz2", "exe", "dmg"},
	Channels:       []string{"release", "beta", "esr", "nightly"},
	DefaultChannel: "release",
	VersionSegments: 3,

	Features: BrowserFeatures{
		SupportsHeadless:  true,
		SupportsIncognito: true,
		SupportsProfile:   true,
		CanMultiInstance:  true,
		HasUserDirArg:     true,
	},
}
