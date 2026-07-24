# Changelog

This page records feature changes for each version of Browser Workshop, listed in reverse version order.

## v1.0.0-beta (Stabilizing)

> Current version, gradually stabilizing toward release. All core features are implemented, with ongoing polish for stability and documentation.

### Core Features

#### Multi-Version Browser Management

- Supports Chrome, Firefox, and Chromium browsers
- Install and manage multiple versions simultaneously, fully isolated from each other
- Commands: `list`/`ls`, `info`/`show`, `install`/`i`, `uninstall`/`rm`, `run`/`r`, `use`/`u`, `download`/`dl`

#### Local Installation

- Auto-detect and install browser versions from directories or archives
- Smart filename recognition, no need to manually specify version info
- Batch directory installation support: `bws install -d <directory>`
- Single file installation support: `bws install --from-file <file>`

#### Remote Download

- Chrome: query and download via Chrome Omaha protocol
- Firefox: fetch version info via Mozilla Product Details API
- Supports multiple release channels: Stable, Beta, Dev, Canary, ESR
- Full or partial version number matching
- Command: `download`/`dl`

#### Isolated Execution

- Each version uses an independent user data directory (Profile), no interference
- Named profiles, profile reset, orphan profile cleanup
- Same named profile can be shared across versions
- Command: `profile`/`pf`

#### Offline Distribution Service

- Built-in `serve` command to set up LAN browser version distribution service
- Auto-sync (download binary packages from online sources), resumable downloads, checksum verification
- HTML page for manual sync triggering
- Scheduled sync (default once per day)
- Configuration persisted to `bws-serve.ini`, with `packages-dir` and `bin-dir` independent path options
- Command: `serve`/`sv`

#### Source Priority Mechanism

- Offline source (serve service) takes priority, built-in online source as fallback
- Source filtering by browser type: only queries sources that support the specified browser
- Source toggles: `serve-source`, `omaha-source`, `firefox-ftp`, independently enable/disable

#### Configuration Management

- All configuration managed uniformly via `bws cfg` command
- Config file automatically created in data directory (`config.json`)
- First-run setup guides data storage directory selection
- `bws cfg get` (no arguments) lists all readable configuration keys and their aliases
- `bws cfg set` (no arguments or key only) lists all writable configuration keys with example values
- Configuration keys support multiple aliases: `language`→`lang`, `default-browser`→`browser`, etc.
- Command: `config`/`cfg`

#### Alias System

- Browser short aliases: `gc` (chrome), `ff` (firefox), `cm` (chromium)
- Command short aliases: `r` (run), `u` (use), `dl` (download), `cfg` (config), `sv` (serve), `cc` (cache), `pf` (profile), `dt` (doctor), `sc` (shortcut)
- Multi-name browser recognition: `chrome`/`googlechrome`/`google-chrome`, etc.

#### Desktop Shortcuts

- Create, remove, and list desktop shortcuts
- Cross-platform support: Windows (`.lnk`), Linux (`.desktop`), macOS (`.app`)
- Command: `shortcut`/`sc`

### Enhanced Features

#### Internationalization (i18n)

- Built-in Chinese and English, configurable via `bws cfg set language`
- External translation file override: create a JSON file at `<data-dir>/i18n/<lang>.json` to override built-in translations
- Auto-detect system language (reads `LANG`/`LANGUAGE` environment variables, defaults to Chinese if not set)
- Language template file `template.json` provided for contributors to add new languages

#### Command Typo Suggestions

- Automatically detects similar commands and suggests them when an unknown command is entered
- Based on Levenshtein edit distance algorithm, with prefix matching weighting and adjacent character swap detection
- Suggestions below 35% similarity are not shown to avoid irrelevant prompts
- Example: typing `bws insall` suggests `Did you mean "install"? (similarity: 96%)`

#### Plugin System

- Two plugin types: Lua script plugins (simple logic) and standalone process plugins (complex logic)
- Hook mechanism for injecting into core flow: `pre-run`, `post-run`, `pre-install`, `post-install`, `on-exit`
- Plugin marketplace via JSON index files hosted on GitHub/Gitee, no self-hosted server needed
- Three installation methods: Registry index, Git repository, local file
- SHA256 hash verification on plugin download to ensure file integrity
- Registry cache for 24 hours to avoid frequent downloads
- Commands: `bws plugin list/install/uninstall/search`

#### Proxy Support

- Global proxy configuration: set via `bws cfg set proxy <url>`, used for downloading browser packages and querying version sources
- Browser launch proxy: specify via `--proxy <url>` or use global config, `--no-proxy` to disable
- Supported protocols: HTTP, HTTPS, SOCKS5, SOCKS5h (DNS resolved through proxy)
- Chrome/Chromium uses `--proxy-server` flag, Firefox uses `user.js` written to profile directory

#### Fingerprint Isolation

- Command-line level basic fingerprint isolation, triggered by `--fingerprint` flag
- Preset modes: `standard` (basic protection), `random` (random fingerprint), `none` (no isolation)
- Random fingerprint includes: User-Agent (Windows/Mac/Linux sets), language (7 options), resolution (8 options), DPR
- WebRTC randomly disabled or proxied, WebGL 50% chance disabled, virtual media devices always enabled
- Custom JSON configuration and file loading supported

#### ESR Channel Support

- `default-channel` configuration supports `esr` as a valid value
- Full support for Firefox ESR version querying, downloading, and installation
- Version number recognition supports `esr` suffix (e.g., `115.6.0esr`)

### Other Features

- **Portable mode**: data stored in `bws-data/` next to the executable, fully portable
- **Logging system**: leveled logging, file log at DEBUG level, console log at INFO level
- **System integration**: auto-detects system-installed browser versions
- **Architecture compatibility**: auto-detects architecture compatibility, x64 can run x86 versions
- **Disk check**: checks available disk space before downloading
- **Repository management**: local binary repository scanning and management
- **Health check**: `bws doctor`/`dt` system health check
- **Multi-format archives**: supports zip, 7z, tar.gz, tar.bz2, tar.xz, .exe, etc., with magic byte detection

### Supported Archive Formats

| Format | Description |
|--------|-------------|
| `.zip` / `.jar` / `.apk` / `.war` | Native Go support |
| `.7z` | bodgit/sevenzip library |
| `.tar.gz` / `.tar.bz2` / `.tar.xz` / `.tar.zst` | Corresponding compression library + tar |
| `.exe` | Self-extracting (zip header detection) |

> Not supported: `.rar`, `.msi`, `.deb`, `.rpm`, `.iso`, `.wim`
