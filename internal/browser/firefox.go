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
	//
	// app.update.background.enabled is critical on Linux: Firefox ships an
	// `updater` binary inside the install directory that the background
	// updater can launch in a child process. Setting only app.update.enabled
	// does NOT disable it, so on Linux Firefox would still download staged
	// updates in the background and show the "we need to restart" page on
	// every launch.
	StandardPrefs: []string{
		`user_pref("app.update.enabled", false);`,
		`user_pref("app.update.auto", false);`,
		`user_pref("app.update.background.enabled", false);`,
		`user_pref("app.update.doorhanger", false);`,
		`user_pref("app.update.silent", false);`,
		`user_pref("browser.shell.checkDefaultBrowser", false);`,
	},

	// Linux-specific environment variables for Firefox. These are applied
	// in addition to runtime-detected ones in launch.go.
	//
	// MOZ_ENABLE_WAYLAND=0 forces Firefox to use X11 instead of Wayland.
	// Wayland support in Firefox on some desktop environments (GNOME 46+
	// on Ubuntu 24.04, KDE Plasma, etc.) is known to cause "tab crashed"
	// pages when combined with bws's profile isolation, because the
	// content process can't acquire an X connection in time.
	//
	// The launch manager may additionally set MOZ_DISABLE_SANDBOX=1
	// at runtime if the host kernel does not permit unprivileged user
	// namespaces (the default on Ubuntu 24.04 for security reasons).
	EnvVars: map[string]string{
		"MOZ_ENABLE_WAYLAND": "0",
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
