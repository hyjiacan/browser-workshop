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
| `bws import` / `bws imp` | Batch import from a directory (auto-detect) |
| `bws uninstall` / `bws rm` / `bws remove` | Uninstall a browser version |
| `bws use` / `bws u` | Set the default browser version |
| `bws download` / `bws dl` | Download only, do not install |
| `bws profile` / `bws pf` | Manage browser profiles |
| `bws alias` | Manage version aliases |
| `bws serve` / `bws sv` / `bws server` | Start the HTTP distribution service |
| `bws config` / `bws cfg` | Manage configuration |
| `bws repo` | Manage the local binary repository |
| `bws cache` / `bws cc` | Manage download cache |
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
| `--json` | - | Output in JSON format |

### Examples

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

# Output in JSON format
bws ls --json
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
bws r [browser[@version]] [URL] [options] [-- native arguments]
```

### Arguments

| Argument | Description |
|----------|-------------|
| `browser[@version]` | The browser version to run; if omitted, the default browser and default version are used |
| `URL` | Optional, the URL to open on startup |

### Options

| Option | Short | Description |
|--------|-------|-------------|
| `--headless` | `-H` | Headless mode |
| `--incognito` | `-i` | Incognito / private browsing mode |
| `--new-window` | `-w` | Open in a new window |
| `--profile <name>` | `-p` | Specify a named profile |
| `--native` | - | Native mode (use system profile) |
| `--detach` | `-d` | Run in the background (do not wait for the process) |
| `--dry-run` | - | Dry run (do not actually start) |
| `--proxy <url>` | - | Proxy URL (e.g. `socks5://127.0.0.1:1080`), empty uses global config |
| `--no-proxy` | - | Disable proxy (overrides global config) |
| `--fingerprint <preset>` | `-fp` | Fingerprint isolation preset (`standard`/`random`/`none`), or JSON config/@file path |
| `--` | - | Arguments after this are passed directly to the browser |

### Examples

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

# Use the open alias
bws open chrome@120
```
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
| `--dir <path>` | `-d` | Install from a local directory |
| `--from-file <path>` | - | Install from a local archive |
| `--channel <channel>` | - | Specify the release channel |
| `--force` | `-f` | Force reinstall |

### Examples

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

## bws import (alias: imp)

Batch import browser versions from a directory (auto-detect).

### Usage

```bash
bws imp <directory> [options]
```

### Arguments

| Argument | Description |
|----------|-------------|
| `directory` | The directory path containing browser installation packages |

### Options

| Option | Short | Description |
|--------|-------|-------------|
| `--force` | `-f` | Force re-import of already installed versions |

### Examples

```bash
# Batch import
bws imp /path/to/browsers

# Force re-import
bws imp /path/to/browsers -f
```

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

### profile list

```bash
# List all profiles
bws pf list

# List profiles for a specific browser
bws pf list chrome
```

### profile path

```bash
# View the default profile path
bws pf path chrome@120

# View the named profile path
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

```bash
# List all aliases
bws alias list

# Add an alias
bws alias add mychrome chrome@120.0.6099.109

# Remove an alias
bws alias remove mychrome
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

The first time you run `bws sv`, a configuration file is automatically created. Edit it and rerun to start the service.

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

### Examples

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
| `log-level` | Console log level | `info` |
| `repo-path` | Local repository path | Empty |
| `source` | Offline source address | Empty |
| `source-serve` | Serve source switch | `true` |
| `source-omaha` | Omaha source switch | `true` |
| `source-firefox-ftp` | Firefox FTP source switch | `true` |
| `disk-threshold` | Disk space alert threshold (GB) | `5` |
| `proxy` | Proxy URL (for downloads and browser launching) | empty |

### Examples

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

Manage the download cache. Downloaded files are stored in the temporary directory and are automatically cleaned up after installation.

### Usage

```bash
bws cc <subcommand>
```

### Subcommands

| Subcommand | Description |
|------------|-------------|
| `clear` | Clear cached download files (note: files are stored in the temporary directory and are automatically cleaned up) |
| `info` | Display cache information (type: temporary, auto-cleanup) |

### Examples

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
| `uninstall` | `rm`, `remove` | Uninstall a plugin |
| `search` | `s`, `find` | Search remote plugins |

### Examples

```bash
# List installed plugins
bws plugin list

# Install from local Lua file
bws plugin install ./my-plugin.lua

# Install from local IPC plugin (any executable)
bws plugin install ./my-plugin.py

# Install from registry
bws plugin install fingerprint-enhanced

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

## bws doctor (alias: dt)

System health check.

### Usage

```bash
bws dt
```

### Check Content

- Data directory integrity
- Configuration file validity
- Installed version integrity
- Disk space check
- Network connectivity (optional)

### Examples

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

### Examples

```bash
# Display general help
bws help

# Display help for a specific command
bws help r
bws help i

# Use the h alias
bws h ls
```
