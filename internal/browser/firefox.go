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
