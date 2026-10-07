# Browser Workshop

<p align="center">
  <img src="https://gitee.com/hyjiacan/browser-workshop/raw/master/logo.png" alt="Browser Workshop logo" width="128" />
</p>

<p align="center">
  Isolated browser multi-version manager for fast, clean test environment switching — and a browser supply layer for test automation.
</p>

## Features

- **Multi-version Management**: Install and manage multiple browser versions side by side, fully isolated from each other
- **Flexible Version Sources**: Auto-detect and import from local directories or archives, download from official sources, and detect browsers already installed on the system
- **Isolated Execution**: Each version runs with its own profile, with named profiles shareable across versions
- **Offline Distribution**: A built-in `serve` command sets up a LAN distribution service, so intranets can fetch browsers and drivers too
- **Automation Integration**: One command launches a browser and exposes CDP / WebDriver endpoints, preparing a version-matched chromedriver automatically
- **Background Instance Management**: Start with `run --daemon`, inspect with `ps`, stop with `stop` — a scriptable lifecycle

## Quick Start

```bash
# View installed versions
bws ls

# Install a specific version
bws i chrome@120

# Import from a local directory or archive
bws i -d /path/to/browsers

# Run the browser
bws r chrome@120

# Automation: launch and expose CDP / WebDriver endpoints
bws r chrome@120 --automation
```

> Historical Chrome versions have no official remote source. Download one manually and import it with `bws i --from-file chrome-120-win64.zip chrome@120`.

## Documentation

For full documentation, please visit: **[Browser Workshop Documentation](https://hyjiacan.github.io/browser-workshop)**

- [Getting Started](https://hyjiacan.github.io/browser-workshop/en/guide/getting-started)
- [Commands Reference](https://hyjiacan.github.io/browser-workshop/en/guide/commands)
- [Automation Framework Integration](https://hyjiacan.github.io/browser-workshop/en/guide/automation)
- [Serve Service](https://hyjiacan.github.io/browser-workshop/en/guide/serve)
- [Browser Short Aliases](https://hyjiacan.github.io/browser-workshop/en/guide/short-aliases)

## Installation

```bash
go install github.com/hyjiacan/browser-workshop/cmd/bws@latest
```

Or download precompiled binaries from [Releases](https://github.com/hyjiacan/browser-workshop/releases).

Users in China can also install via Gitee:

```bash
go install gitee.com/hyjiacan/browser-workshop/cmd/bws@latest
```

## Command Overview

| Command | Aliases | Description |
|---------|---------|-------------|
| `bws list` | `ls` | List installed browser versions |
| `bws info` | `show` | Show detailed version information |
| `bws run` | `r`, `open` | Run a specific browser version |
| `bws install` | `i` | Install a browser version |
| `bws shortcut` | `sc` | Manage desktop shortcuts |
| `bws uninstall` | `rm`, `remove` | Uninstall a browser version |
| `bws use` | `u` | Set the default browser version |
| `bws download` | `dl` | Download only, without installing |
| `bws profile` | `pf` | Manage browser profiles |
| `bws alias` | — | Manage version aliases |
| `bws update` | `upgrade`, `up` | Update bws from the configured offline source |
| `bws serve` | `sv`, `server` | Start the HTTP distribution service |
| `bws config` | `cfg` | Manage configuration |
| `bws repo` | — | Manage the local binary repository |
| `bws cache` | `cc` | Manage the download cache |
| `bws plugin` | `pl` | Manage plugins |
| `bws driver` | `drv` | Manage automation drivers (chromedriver) |
| `bws where` | `path` | Print the local path of a browser |
| `bws endpoint` | — | Print the CDP / WebDriver endpoints of an instance |
| `bws ps` | — | List running background instances |
| `bws stop` | `kill` | Stop running background instances |
| `bws doctor` | `dt` | System health check |
| `bws help` | `h` | Show help information |

For full command descriptions, please see [Commands Reference](https://hyjiacan.github.io/browser-workshop/en/guide/commands).

## Browser Short Aliases

| Short Alias | Full Name |
|-------------|-----------|
| `gc` | chrome / googlechrome |
| `ff` | firefox |
| `cm` | chromium |

All commands support short aliases. For details, see [Browser Short Aliases](https://hyjiacan.github.io/browser-workshop/en/guide/short-aliases).

## Automation Integration

bws acts as the browser supply layer for test automation frameworks: one command launches a version-precise, isolated browser and exposes standard endpoints.

```bash
# Launch the browser, prepare the matching chromedriver, and print CDP / WebDriver endpoints
bws r chrome@120 --automation

# Run in the background and register a manageable instance
bws r chrome@120 --automation --daemon --profile test-01

# Inspect and stop background instances
bws ps
bws stop bws-chrome-120-test-01

# When only the browser path is needed (e.g. Cypress)
bws where chrome@120
```

`--automation` is a superset switch, equivalent to enabling both `--cdp` and `--webdriver`. Endpoint failures never block the browser launch; the affected field is emitted as `null`. See [Automation Framework Integration](https://hyjiacan.github.io/browser-workshop/en/guide/automation).

## Serve Service

Set up a browser distribution service for intranets or teams, so clients can fetch browser versions and drivers without external network access.

```bash
# The first run generates a configuration file; edit it and start again
bws sv
```

Supports parallel scanning, online fallback (fetching from online sources when a local package is missing) and driver hosting. See [Serve Service Documentation](https://hyjiacan.github.io/browser-workshop/en/guide/serve).

## License

MIT