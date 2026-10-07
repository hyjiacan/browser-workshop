# Commands Reference

This document lists all commands of `bws` with detailed descriptions, including usage, examples, and arguments.

> Version information is obtained via the global flag `--version` / `-v`, for example `bws --version` or `bws -v`.

## Command Overview

| Command | Description |
|---------|-------------|
| `bws list` / `bws ls` | List installed browser versions |
| `bws info` / `bws show` | Show detailed version information |
| `bws run` / `bws r` / `bws open` | Run a specific browser version |
| `bws install` / `bws i` | Install a browser version |
| `bws shortcut` / `bws sc` | Manage desktop shortcuts |
| `bws uninstall` / `bws rm` / `bws remove` | Uninstall a browser version |
| `bws use` / `bws u` | Set the default browser version |
| `bws download` / `bws dl` | Download only, do not install |
| `bws profile` / `bws pf` | Manage browser profiles |
| `bws alias` | Manage version aliases |
| `bws update` / `bws upgrade` / `bws up` | Update bws to the latest version from the configured source |
| `bws serve` / `bws sv` / `bws server` | Start the HTTP distribution service |
| `bws config` / `bws cfg` | Manage configuration |
| `bws repo` | Manage the local binary repository |
| `bws cache` / `bws cc` | Manage download cache |
| `bws plugin` / `bws pl` | Plugin management |
| `bws driver` / `bws drv` | Manage automation drivers (chromedriver) |
| `bws where` / `bws path` | Print the browser binary path (for Cypress and other frameworks) |
| `bws endpoint` | Print the CDP / WebDriver endpoints of an instance |
| `bws ps` | List running background instances |
| `bws stop` / `bws kill` | Stop running background instances |
| `bws doctor` / `bws dt` | System health check |
| `bws help` / `bws h` | Display help information |

---

## bws list (alias: ls)

List installed browser versions.

### Usage

```bash
bws ls [browser[@version]] [options]
```

### Arguments

| Argument | Description |
|----------|-------------|
| `browser[@version]` | Optional, filter by browser and version prefix |

### Options

| Option | Short | Description |
|--------|-------|-------------|
| `--remote` | `-R` | List remote available versions |
| `--all` | `-a` | Show all browsers |
| `--no-system` | - | Do not show system browsers |
| `--channel <channel>` | `-c` | Specify channel (only valid for remote listing) |
| `--limit <number>` | `-n` | Limit the number of results (default 20, only valid for remote listing) |
| `--refresh` | - | Force refresh remote source cache (only valid for remote listing) |
| `--json` | - | Output in JSON format (local mode; emits `installed[]`) |

### Examples

> `bws list` (alias `bws ls`)

```bash
# List all installed versions
bws ls

# List only Chrome
bws ls chrome

# Use short alias
bws ls gc

# Filter by version prefix
bws ls chrome@79

# List remote available versions
bws ls -R chrome

# Show all browsers
bws ls -a

# Do not show system browsers
bws ls --no-system

# List remote versions for a specified channel
bws ls -R chrome -c beta

# Limit the number of remote results
bws ls -R chrome -n 5

# Force refresh remote source cache (bypass cache, fetch latest from network)
bws ls -R firefox --refresh

# Output in JSON format
bws ls --json
```

JSON structure:

```json
{
  "ok": true,
  "command": "ls",
  "data": {
    "installed": [
      { "browser": "chrome", "version": "120.0.6099.109", "type": "bws", "channel": "stable" },
      { "browser": "chrome", "version": "125.0.6422.112", "type": "system", "channel": "stable" }
    ]
  }
}
```

---

## bws info (alias: show)

Show detailed information of a specific version.

### Usage

```bash
bws show <browser@version>
```

### Arguments

| Argument | Description |
|----------|-------------|
| `browser@version` | The browser version to view (supports partial version numbers) |

### Examples

> `bws info` (alias `bws show`)

```bash
# View details of a specific version
bws show chrome@120

# View the full version
bws show chrome@120.0.6099.109

# View system browser information
bws show chrome@system

# Use short alias
bws show ff@121
```

### Output Content

- Browser name and version number
- Release channel
- Installation path
- Architecture information
- Profile path
- Executable file path
- Installation source

---

## bws run (alias: r, open)

Run a specific browser version.

### Usage

```bash
bws r <browser[@version]> [URL] [options] [-- native arguments]
```

### Arguments

| Argument | Description |
|----------|-------------|
| `browser[@version]` | The browser version to run (required), e.g. `chrome@120` |
| `URL` | Optional, the URL to open on startup |

### Options

| Option | Short | Description |
|--------|-------|-------------|
| `--headless` | `-H` | Headless mode |
| `--incognito` | `-i` | Incognito / private browsing mode |
| `--new-window` | `-w` | Open in a new window |
| `--profile <name>` | `-p` | Specify a named profile |
| `--native` | `-n` | Native mode (use system profile) |
| `--detached` | `-d` | Run in the background (do not wait for the process) |
| `--dry-run` | - | Dry run (do not actually start) |
| `--proxy <url>` | - | Proxy URL (e.g. `socks5://127.0.0.1:1080`), empty uses global config |
| `--no-proxy` | - | Disable proxy (overrides global config) |
| `--fingerprint <preset>` | `-fp` | Fingerprint isolation preset (`standard`/`random`/`none`), or JSON config/@file path |
| `--plugin <names>` | - | Activate plugins (comma-separated) |
| `--automation` | - | Automation mode: inject CDP flags, manage a matching driver, and expose the endpoints |
| `--cdp` | - | Enable the CDP endpoint only (subset of `--automation`) |
| `--webdriver` | - | Enable the WebDriver endpoint only (subset of `--automation`) |
| `--daemon` | - | Run in the background and register a manageable instance (equivalent to `--detached` + registry) |
| `--driver-port <port>` | - | chromedriver listen port (`0` picks a free port) |
| `--driver-no-download` | - | Do not auto-download the driver in automation mode (warn only if missing) |
| `--endpoint-timeout <sec>` | - | Endpoint discovery timeout in seconds (default 10) |
| `--json` | - | Emit JSON (requires `--automation`/`--cdp`/`--webdriver`/`--daemon`) |
| `--` | - | Arguments after this are passed directly to the browser |

### Examples

> `bws run` (alias `bws r`, `bws open`)

```bash
# Run a specific version
bws r chrome@120

# Run the default version
bws r chrome

# Run the system version
bws r chrome@system

# Open a specific URL
bws r chrome@120 https://example.com

# Headless mode
bws r chrome@120 -H

# Incognito mode
bws r chrome@120 -i

# Specify a named profile
bws r chrome@120 -p work

# Run in the background
bws r chrome@120 -d

# Pass native arguments
bws r chrome@120 -- --disable-gpu --no-sandbox

# Dry run
bws r chrome@120 --dry-run

# Use a proxy
bws r chrome@120 --proxy socks5://127.0.0.1:1080

# Disable proxy (overrides global config)
bws r chrome@120 --no-proxy

# Fingerprint isolation: random fingerprint
bws r chrome@120 --fingerprint random

# Fingerprint isolation: standard protection
bws r chrome@120 --fingerprint standard

# Fingerprint isolation: custom JSON
bws r chrome@120 --fingerprint '{"userAgent":"...","language":"en-US","webrtc":"disabled"}'

# Automation mode: inject CDP flags, manage a matching driver, and expose the endpoints
bws r chrome@120 --automation

# Automation mode with a fixed driver port
bws r chrome@120 --automation --driver-port 9515

# Enable the CDP endpoint only
bws r chrome@120 --cdp

# Enable the WebDriver endpoint only
bws r chrome@120 --webdriver

# Run in the background and register a manageable instance
bws r chrome@120 --automation --daemon --profile test-01

# Automation mode with JSON output
bws r chrome@120 --automation --json

# Use the open alias
bws open chrome@120
```
### Automation Mode

`--automation` is the superset switch for automation: it injects CDP flags, prepares a driver (chromedriver) matching the launched version, and prints both the CDP and WebDriver endpoints so Playwright, Puppeteer, Selenium, WebdriverIO and other frameworks can attach.

**Flow:**

1. Resolve the Chrome version being launched (e.g. `120.0.6099.109`)
2. Inject `--remote-debugging-port=0`, `--remote-debugging-address=127.0.0.1`, `--disable-blink-features=AutomationControlled`
3. Launch the browser and discover the CDP endpoint from the `DevToolsActivePort` file (falling back to `GET /json/version`)
4. Look up the Chrome for Testing manifest, download and extract chromedriver to `bws-data/drivers/chromedriver/120/`
5. Start chromedriver, listen on a port, and print the WebDriver endpoint

```bash
# Launch the browser and prepare CDP + driver automatically
bws r chrome@120 --automation --profile test-01
# Output:
# Instance:  bws-chrome-120-test-01
# CDP:       ws://127.0.0.1:54321/devtools/browser/xxxxx
# WebDriver: http://127.0.0.1:9515
```

**JSON output contract:**

```bash
bws r chrome@120 --automation --json
```

```json
{
  "ok": true,
  "command": "run",
  "data": {
    "instance": "bws-chrome-120",
    "browser": "chrome",
    "version": "120.0.6099.109",
    "pid": 12345,
    "binary": "C:\\bws\\bws-data\\versions\\chrome\\120.0.6099.109\\chrome.exe",
    "profile": "C:\\bws\\bws-data\\runtime\\chrome\\120.0.6099.109\\test-01",
    "cdp": "ws://127.0.0.1:54321/devtools/browser/xxxxx",
    "webdriver": "http://127.0.0.1:9515",
    "daemon": false
  }
}
```

Endpoint fields render as `null` when unavailable (the key always exists). Endpoint-related failures **never block the browser launch**; they only warn on stderr.

**Related options:**

| Option | Description |
|--------|-------------|
| `--cdp` | Subset of `--automation`, only enables the CDP endpoint |
| `--webdriver` | Subset of `--automation`, only enables the WebDriver endpoint |
| `--daemon` | Run in the background and register a manageable instance (with `ps` / `stop`) |
| `--driver-port <port>` | Fixed driver listen port; `0` picks a free port |
| `--driver-no-download` | Do not auto-download the driver; warn only if missing |
| `--endpoint-timeout <sec>` | Endpoint discovery timeout in seconds (default 10) |
| `--json` | Emit the launch contract as JSON |

**User parameters win:** if you set `--remote-debugging-port=9222` after `--`, bws keeps that value instead of injecting `=0`.

**Notes:**

- CDP is only supported on Chrome/Chromium/Edge; WebDriver currently only manages chromedriver for Chrome/Chromium
- A driver start failure or endpoint discovery timeout never aborts the browser launch; it only warns on stderr and the field is `null`
- When a serve source is configured, drivers are resolved and downloaded through it first
- Drivers can also be resolved, installed and uninstalled separately with the [`bws driver`](#bws-driver-alias-drv) command

> For a complete automation integration guide (Playwright/Puppeteer/Selenium/Cypress), see [Automation Framework Integration](./automation.md).

### Fingerprint Isolation

The `--fingerprint` (short `-fp`) option adds fingerprint masking when launching the browser, reducing the accuracy of website fingerprinting.

**Preset modes:**

| Preset | Description |
|--------|-------------|
| `standard` | Basic protection: disables WebRTC, uses fake media devices |
| `random` | Random fingerprint: generates random UA, language, resolution each time |
| `none` | No fingerprint isolation (default) |

**Custom configuration:**

```bash
# Direct JSON
bws r chrome@120 --fingerprint '{"userAgent":"...","language":"en-US","webrtc":"disabled","disableWebGL":true,"fakeMediaDevices":true,"windowWidth":1280,"windowHeight":720,"devicePixelRatio":1}'

# From file
bws r chrome@120 --fingerprint @./fingerprint.json
```

**JSON config fields:**

| Field | Type | Description |
|-------|------|-------------|
| `preset` | string | Preset identifier (`custom`) |
| `userAgent` | string | HTTP User-Agent header |
| `language` | string | Browser language |
| `windowWidth` | int | Window width |
| `windowHeight` | int | Window height |
| `devicePixelRatio` | float | Device pixel ratio |
| `webrtc` | string | WebRTC policy: `disabled`/`proxied`/`default` |
| `disableWebGL` | bool | Disable WebGL |
| `disableCanvasRead` | bool | Disable canvas readback |
| `fakeMediaDevices` | bool | Use fake media devices |

**Browser implementation differences:**

| Dimension | Chrome/Chromium | Firefox |
|-----------|:---:|:---:|
| User-Agent | `--user-agent` CLI flag | `general.useragent.override` pref |
| Language | `--lang` CLI flag | `intl.accept_languages` pref |
| Window size | `--window-size` CLI flag | Managed by RFP |
| DPR | `--force-device-scale-factor` | Managed by RFP |
| WebRTC | `--force-webrtc-ip-handling-policy` | `media.peerconnection.*` prefs |
| Comprehensive | Per-flag CLI control | `privacy.resistFingerprinting` one-click |

> **Note**: Chrome's CLI flags only control the HTTP layer and some browser behaviors. They **cannot override JS-side `navigator.userAgent`, `screen` objects, or Canvas/WebGL rendering results**. These require Chrome DevTools Protocol or browser extensions to inject JS scripts. Firefox's `resistFingerprinting` provides more comprehensive built-in protection.

---

## bws install (alias: i)

Install a browser version.

### Usage

```bash
bws i <browser@version> [options]
bws i -d <directory> [browser@version]
bws i --from-file <file> [browser@version]
```

### Arguments

| Argument | Description |
|----------|-------------|
| `browser@version` | The browser version to install (supports latest, beta, partial version numbers, etc.) |

### Options

| Option | Short | Description |
|--------|-------|-------------|
| `--from-dir <path>` | `-d` | Install from a local directory |
| `--from-file <path>` | - | Install from a local archive |
| `--channel <channel>` | `-c` | Specify the release channel |
| `--force` | `-f` | Force reinstall |
| `--refresh` | - | Force re-download from serve (ignore local cache) |

### Examples

> `bws install` (alias `bws i`)

```bash
# Install the latest stable version
bws i chrome@latest

# Install a specific channel
bws i chrome@beta

# Install a specific full version
bws i chrome@120.0.6478.114

# Install a partial version number
bws i chrome@85

# Install from a directory
bws i -d /path/to/browser-dir

# Install from a directory and specify the version
bws i -d /path/to/browser-dir chrome@120

# Install from a file
bws i --from-file /path/to/chrome-setup.exe chrome@120

# Force reinstall
bws i chrome@120 --force

# Force re-download from serve (ignore local cache)
bws i chrome@120 --refresh
```

---

## bws shortcut (alias: sc)

Create, remove, or list desktop shortcuts for installed browsers. Shortcuts point directly to the browser executable and can be launched by double-clicking.

### Usage

```bash
bws sc <subcommand> [browser[@version]] [options]
```

### Subcommands

| Subcommand | Aliases | Description |
|------------|---------|-------------|
| `create` | `c`, `add` | Create a desktop shortcut |
| `remove` | `rm`, `del` | Remove a desktop shortcut |
| `list` | `ls` | List created shortcuts |

### Arguments

| Argument | Description |
|----------|-------------|
| `browser[@version]` | Optional, specify browser and version (supports latest, stable, etc.) |

### Options

| Option | Short | Description |
|--------|-------|-------------|
| `--profile <name>` | `-p` | Specify profile name |
| `--native` | `-n` | Native mode (no profile) |
| `--all` | `-a` | Create/remove for all installed versions |
| `--name <name>` | - | Custom shortcut name |

### Examples

> `bws shortcut` (alias `bws sc`)

```bash
# Create a shortcut for a specific version
bws sc create chrome@120

# Create with a specific profile
bws sc create firefox@latest --profile dev

# Create shortcuts for all installed versions
bws sc create --all

# Remove a shortcut
bws sc remove chrome@120

# Remove all shortcuts
bws sc remove --all

# List created shortcuts
bws sc list
```

### Cross-platform Notes

| Platform | Shortcut Type | Location |
|----------|--------------|----------|
| Windows | `.lnk` | Desktop |
| Linux | `.desktop` | Desktop + `~/.local/share/applications/` |
| macOS | `.app` bundle | Desktop |

---

## bws uninstall (alias: rm, remove)

Uninstall a specific browser version.

### Usage

```bash
bws rm <browser@version>
```

### Arguments

| Argument | Description |
|----------|-------------|
| `browser@version` | The browser version to uninstall (supports partial version numbers) |

### Examples

> `bws uninstall` (alias `bws rm`, `bws remove`)

```bash
# Uninstall a specific version
bws rm chrome@120

# Uninstall the latest version matching a partial version number
bws rm chrome@85
```

### Notes

- Uninstall only removes program files, not profile data
- System-installed browsers cannot be uninstalled via bws

---

## bws use (alias: u)

Set the default browser version.

### Usage

```bash
bws u <browser@version>
```

### Arguments

| Argument | Description |
|----------|-------------|
| `browser@version` | The browser version to set as default (supports partial version numbers) |

### Examples

> `bws use` (alias `bws u`)

```bash
# Set Chrome 120 as the default version
bws u chrome@120

# Use short alias
bws u gc@120

# Run directly after setting
bws r chrome
```

---

## bws download (alias: dl)

Download the installer only, do not install.

### Usage

```bash
bws dl <browser@version> [options]
```

### Arguments

| Argument | Description |
|----------|-------------|
| `browser@version` | The browser version to download |

### Options

| Option | Short | Description |
|--------|-------|-------------|
| `--output <directory>` | `-o` | Specify the output directory |
| `--channel <channel>` | `-c` | Specify the release channel |

### Examples

> `bws download` (alias `bws dl`)

```bash
# Download the latest stable version
bws dl chrome@latest

# Download a specific version
bws dl chrome@120.0.6478.114

# Download a partial version number
bws dl chrome@85

# Specify the output directory
bws dl chrome@latest -o ~/downloads

# Download a specific channel
bws dl chrome@beta -c beta
```

---

## bws profile (alias: pf)

Manage browser profiles.

### Usage

```bash
bws pf <subcommand> [arguments] [options]
```

### Subcommands

| Subcommand | Description |
|------------|-------------|
| `list` | List all profiles |
| `path` | View profile path |
| `reset` | Reset profile |
| `clean` | Clean up orphaned profiles |

### Examples

> `bws profile` (alias `bws pf`)

### profile list

| Option | Short | Description |
|--------|-------|-------------|
| `--browser <name>` | `-b` | Specify browser |

```bash
# List all profiles
bws pf list

# List profiles for a specific browser
bws pf list chrome
bws pf list --browser firefox
```

### profile path

```bash
# View default browser profile path
bws pf path

# View a specific browser's profile path
bws pf path chrome

# View a named profile path
bws pf path chrome myprofile
```

### profile reset

```bash
# Reset the default profile
bws pf reset chrome@120

# Reset a named profile
bws pf reset chrome@120 myprofile

# Skip confirmation
bws pf reset chrome@120 -f
```

### profile clean

```bash
# Clean up all orphaned profiles
bws pf clean

# Clean up orphaned profiles for a specific browser
bws pf clean chrome

# Skip confirmation
bws pf clean -f
```

---

## bws alias

Manage version aliases.

### Usage

```bash
bws alias <subcommand> [arguments]
```

### Subcommands

| Subcommand | Description |
|------------|-------------|
| `list` | List all aliases |
| `add` | Add an alias |
| `remove` | Remove an alias |

### Examples

> `bws alias` (no short alias)

```bash
# List all aliases
bws alias list

# Add an alias
bws alias add mychrome chrome@120.0.6099.109

# Remove an alias
bws alias remove mychrome
```

---

## bws update (alias: upgrade, up)

Download and update bws to the latest version from the configured serve source.

### Usage

```bash
bws update
```

### Notes

- Requires configuring a serve source via `bws cfg set source <url>` first
- Automatically fetches the latest binary matching the current platform/arch from `/api/v1/bin`
- No action is taken if the current version is already the latest
- Upgrade process: download new version → backup old version → replace → cleanup

### Examples

> `bws update` (aliases `bws upgrade`, `bws up`)

```bash
# Configure serve source
bws cfg set source http://192.168.1.1:8080

# Check for and install the latest version
bws update
bws up
```

---

## bws serve (alias: sv, server)

Start the HTTP distribution service. Configuration is managed through the `bws-serve.ini` file, which is automatically created with default settings on the first run.

### Usage

```bash
bws sv [-d <directory>]
```

### Options

| Option | Description |
|--------|-------------|
| `-d, --dir` | Base directory (contains packages/ and bin/), defaults to the program directory |

### Configuration File (bws-serve.ini)

The first time you run `bws sv`, a configuration file is automatically created in the same directory as the bws executable. Edit it and rerun to start the service.

| Configuration Item | Default Value | Description |
|--------------------|---------------|-------------|
| `host` | `0.0.0.0` | Listening host address |
| `port` | `8080` | Listening port |
| `packages-dir` | Program directory/packages | Directory for storing browser installation packages |
| `bin-dir` | Program directory/bin | Directory for storing client binaries |
| `sync` | `false` | Whether to enable auto-sync |
| `sync-interval` | `24h` | Sync interval (supports 30d, 24h, 30m format) |
| `sync-browsers` | All | List of browsers to sync, comma-separated |
| `sync-channels` | `stable` | List of channels to sync, comma-separated |
| `online-fallback` | `true` | Online fallback: automatically fetch packages from online sources when not cached locally |
| `scan-workers` | `0` | Parallel scan threads, 0 means auto (use CPU core count), range [1, 32] |

### Examples

> `bws serve` (alias `bws sv`, `bws server`)

```bash
# First run (automatically creates configuration file)
bws sv
# Output: Configuration file created: D:\bws\bws-serve.ini
# Edit the configuration file and rerun

# Start the service after editing configuration
bws sv

# Specify the base directory
bws sv -d D:\bws-data

# Use the server alias
bws server
```

### Running in the Background

Refer to the [Serve Service Documentation](/guide/serve#running-in-the-background) for instructions on configuring as a system service using systemd or nssm.

---

## bws config (alias: cfg)

Manage configuration.

### Usage

```bash
bws cfg <subcommand> [arguments]
```

### Subcommands

| Subcommand | Description |
|------------|-------------|
| `show` | View all configurations |
| `get <key>` | Get the value of a specific configuration item |
| `set <key> <value>` | Set the value of a specific configuration item |
| `path` | Display the configuration file path |

### Configuration Items

| Configuration Item | Description | Default Value |
|--------------------|-------------|---------------|
| `data-dir` | Data storage directory | Empty (portable mode) |
| `default-browser` | Default browser | `chrome` |
| `default-channel` | Default channel | `stable` |
| `language` | Interface language (zh/en) | Auto-detected |
| `log-level` | Console log level | `info` |
| `repo-path` | Local repository path | Empty |
| `source` | Offline source address | Empty |
| `source-serve` | Serve source switch | `true` |
| `source-firefox-ftp` | Firefox data source switch | `true` |
| `disk-threshold` | Disk space alert threshold (GB) | `5` |
| `proxy` | Proxy URL (for downloads and browser launching) | empty |

### Examples

> `bws config` (alias `bws cfg`)

```bash
# View all configurations
bws cfg show

# Get a configuration item
bws cfg get default-browser

# Set a configuration item
bws cfg set default-browser firefox
bws cfg set log-level debug
bws cfg set source http://server:8080

# Set a proxy
bws cfg set proxy socks5://127.0.0.1:1080
bws cfg set proxy http://proxy.example.com:8080

# Clear proxy
bws cfg set proxy none

# Display the configuration file path
bws cfg path
```

---

## bws repo

Manage the local binary repository.

### Usage

```bash
bws repo <subcommand> [arguments]
```

### Subcommands

| Subcommand | Description |
|------------|-------------|
| `path` | Display the current repository path |
| `set <path>` | Set the repository path |
| `scan` | Scan browser versions in the repository |
| `import` | Import browser versions from the repository (supports `--force` / `-f` for force reinstall) |

### Examples

> `bws repo` (no short alias)

```bash
# View the current repository path
bws repo path

# Set the repository path
bws repo set /path/to/repo

# Scan the repository
bws repo scan

# Import from the repository
bws repo import

# Force re-import
bws repo import -f
```

---

## bws cache (alias: cc)

Manage the download cache. Installer packages downloaded during remote installation are permanently cached locally (in the `cache/downloads/` directory). Subsequent installations of the same version reuse the cached file without re-downloading. When the file on the serve side is updated (file size changes), it is automatically re-downloaded.

### Usage

```bash
bws cc <subcommand>
```

### Subcommands

| Subcommand | Description |
|------------|-------------|
| `clear` | Clear all cached download files (requires confirmation) |
| `info` | Display cache information (directory, file count, total size, file list) |

### Examples

> `bws cache` (alias `bws cc`)

```bash
# View cache information
bws cc info

# Clear cache
bws cc clear
```

---

## bws plugin (alias: pl)

Manage bws plugins. Plugins can modify browser launch args or execute actions, with two types:

- **Lua scripts** (`.lua`): Simple logic like modifying args, writing config files
- **IPC plugins** (executable files): Communicate via stdin/stdout JSON-RPC, can be written in any language

### Subcommands

| Subcommand | Aliases | Description |
|------------|---------|-------------|
| `list` | `ls`, `l` | List installed plugins |
| `install` | `i`, `add` | Install a plugin (local file or remote registry) |
| `uninstall` | `rm`, `remove`, `del` | Uninstall a plugin |
| `update` | `up`, `u` | Update a plugin to the latest version |
| `search` | `s`, `find` | Search remote plugins |

### Examples

> `bws plugin` (alias `bws pl`)

```bash
# List installed plugins
bws plugin list

# Install from local Lua file
bws plugin install ./my-plugin.lua

# Install from local IPC plugin (any executable)
bws plugin install ./my-plugin.py

# Install from registry
bws plugin install fingerprint-enhanced

# Update a plugin (only for registry-sourced plugins)
bws plugin update fingerprint-enhanced

# Uninstall
bws plugin uninstall fingerprint-enhanced

# Search
bws plugin search fingerprint
```

### Using plugins when launching

```bash
# Activate a plugin
bws r chrome@120 --plugin auto-arg

# Multiple plugins (comma-separated)
bws r chrome@120 --plugin auto-arg,fingerprint-enhanced
```

### Writing plugins

**Lua plugins** are `.lua` files in the `bws-data/plugins/` (portable) or `~/.bws/plugins/` directory.

**Available ctx API:**

| Function/Field | Description |
|----------------|-------------|
| `ctx.browser` | Browser name (e.g. "chrome", "firefox") |
| `ctx.version` | Version number |
| `ctx.profile` | Profile name |
| `ctx.profile_dir` | Profile directory absolute path |
| `ctx.config(key)` | Read bws config value |
| `ctx.add_arg(arg)` | Add a browser launch argument |
| `ctx.set_env(key, value)` | Set an environment variable |
| `ctx.write_file(path, content)` | Write a file (returns nil on success, or error string) |
| `ctx.read_file(path)` | Read a file (returns content, error) |
| `ctx.log(message)` | Log message to stderr |

**IPC plugins** are any executable files that communicate via stdin/stdout JSON-RPC:

- **Request** (stdin): `{"event":"pre_run","browser":"chrome","version":"120","profile":"default","profileDir":"..."}`
- **Response** (stdout): `{"extraArgs":["--flag"],"env":{"KEY":"val"},"error":""}`
- Timeout: 10 seconds, process is killed after timeout
- See `plugins/README.md` and `plugins/examples/browser-alias.py` for details

**Plugins can define a `pre_run()` function, called before browser launch.**

---

## bws driver (alias: drv)

Manage automation drivers (currently chromedriver).

A driver version is bound to the browser version: a chromedriver only drives a Chrome/Chromium with the same major version. bws therefore does not ship a fixed version but resolves, downloads, and starts a matching driver on demand.

### Sources

| Chrome version | Source |
|----------------|--------|
| `>= 115` | Chrome for Testing known-good versions manifest (`known-good-versions-with-downloads.json`) |
| `< 115` | Legacy chromedriver storage bucket |

> When a serve source is configured (`bws cfg set source <url>`), bws resolves and downloads drivers through serve first, which suits intranet/air-gapped deployments.

### Usage

```bash
bws driver <subcommand> [arguments] [options]
```

### Subcommands

| Subcommand | Alias | Description |
|------------|-------|-------------|
| `list` | `ls` | List installed drivers |
| `install` | `i` | Download and install the driver matching a Chrome version |
| `start` | - | Start an installed driver and listen on a port |
| `uninstall` | `rm`, `remove` | Uninstall a driver major version |

### Examples

> `bws driver` (alias `bws drv`)

```bash
# List installed drivers
bws driver ls

# Install the driver matching Chrome 120
bws driver install chrome@120

# Resolve and show download info only
bws driver install 120 --dry-run

# Force reinstall
bws driver install chrome@120 --force

# Start the driver (free port)
bws driver start 120

# Start the driver on a fixed port
bws driver start chrome@120 --port 9515

# Start the driver in the background
bws driver start 120 --detach

# Uninstall the driver
bws driver uninstall 120
```

### driver install options

| Option | Short | Description |
|--------|-------|-------------|
| `--force` | `-f` | Force reinstall |
| `--dry-run` | - | Resolve and show download info only, without installing |

### driver start options

| Option | Short | Description |
|--------|-------|-------------|
| `--port <port>` | `-p` | Listen port (`0` picks a free port) |
| `--detach` | `-d` | Run in the background (do not wait for the process) |
| `--allowed-ips <ips>` | - | Allowed client IPs (default `127.0.0.1`) |
| `--log <file>` | - | Driver log file path |

### driver uninstall options

| Option | Short | Description |
|--------|-------|-------------|
| `--force` | `-f` | Skip the confirmation prompt |

### Directory layout

Drivers are installed under `drivers/chromedriver/<major>/` in the data directory:

```
bws-data/drivers/chromedriver/120/
```

Each major directory holds the driver binary and a metadata file (`.bws-driver.json`) recording the exact version, platform, install time, and more.

> Combine with `bws r --automation` to launch the browser and prepare the driver in one step; see [Automation Mode](#automation-mode).

---

## bws where (alias: path)

Print the local path of a browser, mainly for command substitution — for example letting Cypress use a bws-managed browser:

```bash
cypress run --browser "$(bws where chrome@120)"
```

### Usage

```bash
bws where <browser[@version]> [options]
```

### Arguments

| Argument | Description |
|----------|-------------|
| `browser[@version]` | Required, e.g. `chrome@120` |

### Options

| Option | Description |
|--------|-------------|
| `--dir` | Print the installation directory instead of the binary path |
| `--profile` | Print the profile directory |
| `--profile-name <name>` | Specify the profile name (with `--profile`) |
| `--json` | Emit JSON |

### Examples

```bash
# Default: print the binary path
bws where chrome@120

# Print the installation directory
bws where chrome@120 --dir

# Print the profile directory
bws where chrome@120 --profile --profile-name test-01

# JSON output
bws where chrome@120 --json
```

JSON structure:

```json
{
  "ok": true,
  "command": "where",
  "data": {
    "browser": "chrome",
    "version": "120.0.6099.109",
    "binary": "C:\\bws\\bws-data\\versions\\chrome\\120.0.6099.109\\chrome.exe",
    "dir": "C:\\bws\\bws-data\\versions\\chrome\\120.0.6099.109",
    "profile": "C:\\bws\\bws-data\\runtime\\chrome\\120.0.6099.109\\test-01"
  }
}
```

---

## bws endpoint

Print the CDP / WebDriver endpoints of a background instance.

### Usage

```bash
bws endpoint <instance-name> [options]
```

### Options

| Option | Description |
|--------|-------------|
| `--json` | Emit JSON |

### Examples

```bash
bws endpoint bws-chrome-120-test-01
bws endpoint bws-chrome-120 --json
```

```
CDP:       ws://127.0.0.1:54321/devtools/browser/xxxxx
WebDriver: http://127.0.0.1:9515
```

> The instance name can be found with [`bws ps`](#bws-ps), and instances are created with `bws r ... --daemon`.

---

## bws ps

List running background instances (registered by `bws r --daemon`).

### Usage

```bash
bws ps [options]
```

### Options

| Option | Description |
|--------|-------------|
| `--json` | Emit JSON |

### Examples

```bash
bws ps
bws ps --json
```

```
NAME                     BROWSER   VERSION          PROFILE   PID     CDP
bws-chrome-120-test-01   chrome    120.0.6099.109   test-01   12345   ws://127.0.0.1:54321/…
```

- Prints `No running instances.` when there are none
- Checks PID liveness on query; stale entries of exited processes are pruned automatically with a warning on stderr
- The CDP column in the table is truncated; use `--json` or [`bws endpoint`](#bws-endpoint) for the full value

> `ps` and [`ls`](#bws-list-alias-ls) do not overlap: `ls` lists versions installed on disk, while `ps` lists running processes in the registry.

---

## bws stop (alias: kill)

Stop running background instances.

### Usage

```bash
bws stop <instance-name> [options]
bws stop --all [options]
```

### Options

| Option | Short | Description |
|--------|-------|-------------|
| `--all` | `-a` | Stop all instances |
| `--json` | - | Emit JSON |

### Examples

```bash
# Stop a specific instance
bws stop bws-chrome-120-test-01

# Stop all instances
bws stop --all

# JSON output
bws stop --all --json
```

### Behavior

| Behavior | Description |
|----------|-------------|
| Terminate the process | Graceful exit first, forced kill after timeout |
| Stop the driver | If the instance has an associated chromedriver, it is stopped too |
| Clean up the registry | The entry is removed from the instance registry |
| Keep the profile | Profile data is not deleted and can be reused or cleaned up manually |

- A missing instance is an error with a non-zero exit code
- When a process has already exited but the registry is stale, prints `Stopped: <name> (already exited)`
- The JSON output contains `stopped` and `failed` arrays

---

## Unified Output Contract

All automation-related commands (`run --automation/--daemon`, `where`, `endpoint`, `ps`, `stop`, `ls`) support `--json` and follow a unified envelope:

```json
{
  "ok": true,
  "command": "<command name>",
  "data": { }
}
```

On failure:

```json
{
  "ok": false,
  "command": "<command name>",
  "error": {
    "code": "NOT_FOUND",
    "message": "instance not found: bws-chrome-120"
  }
}
```

Conventions:

- **stdout carries content only; stderr carries metadata only** (startup banner, hints, warnings), which keeps pipelines clean
- Descriptive fields use stable enum values (e.g. `type: system|bws`, `status: running`); localized text appears only in human-readable output
- Optional fields such as endpoints always exist and are `null` when unavailable
- Error codes are language-neutral stable identifiers: `NOT_FOUND`, `INVALID_ARGUMENT`, `UNSUPPORTED`, `CONFLICT`, `INTERNAL`

---

## bws doctor (alias: dt)

System health check.

### Usage

```bash
bws dt
```

### Check Content

- Directory structure integrity
- Configuration file validity
- Browser descriptor count
- Installed version integrity
- System browser detection
- Remote version query availability
- Download manager availability

### Examples

> `bws doctor` (alias `bws dt`)

```bash
bws dt
```

---

## bws help (alias: h)

Display help information.

### Usage

```bash
bws help [command]
bws h [command]
```

### Smart Typo Suggestion

When an entered command name does not exist, bws will automatically detect similar commands and provide a suggestion. For example, entering `bws imfo` will prompt:

```
Did you mean "info"? (similarity: 75%)
```

### Examples

> `bws help` (alias `bws h`)

```bash
# Display general help
bws help

# Display help for a specific command
bws help r
bws help i

# Use the h alias
bws h ls
```
