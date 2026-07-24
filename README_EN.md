# Browser Workshop

<p align="center">
  <img src="https://gitee.com/hyjiacan/browser-workshop/raw/master/logo.png" alt="Browser Workshop logo" width="128" />
</p>

<p align="center">
  Multi-version browser management tool, supporting local import, remote download, version switching, and isolated execution.
</p>

## Features

- **Multi-version Management**: Install and manage multiple browser versions simultaneously, with quick filtering by version prefix
- **Local Import**: Automatically detect and import from directories or archives, supporting zip, 7z, tar.gz, tar.bz2, tar.xz, exe, and more
- **Remote Download**: Download specified versions from official sources (Chrome via Omaha protocol, Firefox via Mozilla Product Details API)
- **Offline Distribution**: Built-in `serve` command with auto-sync support, set up a LAN distribution service
- **Isolated Execution**: Independent profile for each version, with support for named profiles
- **Plugin System**: Lua script plugins and standalone process plugins, automatically modify launch arguments, inject configs, and execute custom logic
- **Proxy Support**: Global proxy configuration covering downloads and browser launch, supporting HTTP/SOCKS5 protocols
- **Fingerprint Isolation**: `--fingerprint` presets (standard/random) to reduce browser fingerprint tracking
- **Internationalization**: Built-in Chinese and English, with external translation file override support
- **Portable Mode**: Data stored in the `bws-data/` subdirectory, carry it on a USB drive
- **Browser Short Aliases**: `gc` (chrome), `ff` (firefox), `cm` (chromium)
- **HTTPS Compatible**: Skip certificate verification by default, adapted for intranet/self-signed certificate environments

## Quick Start

```bash
# View installed versions
bws ls
bws ls gc@79           # Use short alias + version prefix filter

# Batch import from a local directory
bws import /path/to/browsers

# Remote download and install
bws install chrome@120

# Chrome historical versions need to be manually downloaded before importing
# Download address: https://chromedownloads.net/
bws install --from-file chrome-120-win64.zip chrome@120

# Run the browser
bws run chrome@120
bws run gc@120 -i      # Incognito mode
```

## Documentation

For full documentation, please visit: **[Browser Workshop Documentation](https://hyjiacan.github.io/browser-workshop)**

- [Getting Started](https://hyjiacan.github.io/browser-workshop/guide/getting-started)
- [Commands Reference](https://hyjiacan.github.io/browser-workshop/guide/commands)
- [Serve Service](https://hyjiacan.github.io/browser-workshop/guide/serve)
- [Browser Short Aliases](https://hyjiacan.github.io/browser-workshop/guide/short-aliases)

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

| Command | Description |
|---------|-------------|
| `bws ls` / `bws list` | List installed browser versions |
| `bws ls -R` | List remote available versions |
| `bws run <browser@version>` | Run a specific version |
| `bws install <browser@version>` | Install a browser version |
| `bws import <dir>` | Batch import from a directory |
| `bws serve` | Start the HTTP distribution service |
| `bws config` | Manage configuration |
| `bws profile` | Manage profiles |
| `bws plugin` | Manage plugins (install/uninstall/search) |

For full command descriptions, please see [Commands Reference](https://hyjiacan.github.io/browser-workshop/guide/commands).

## Browser Short Aliases

| Short Alias | Full Name |
|-------------|-----------|
| `gc` | chrome / googlechrome |
| `ff` | firefox |
| `cm` | chromium |

All commands support short aliases. For details, see [Browser Short Aliases](https://hyjiacan.github.io/browser-workshop/guide/short-aliases).

## Proxy and Fingerprint Isolation

```bash
# Set global proxy (applies to downloads and browser launch)
bws cfg set proxy socks5://127.0.0.1:1080

# Run with fingerprint isolation
bws r chrome@120 --fingerprint random
bws r chrome@120 --fingerprint standard
```

## Plugin System

bws supports Lua script plugins and standalone process plugins, automatically executing custom logic on browser launch.

```bash
# Install a plugin
bws plugin install ./my-plugin.lua

# Run browser with a plugin
bws r chrome@120 --plugin my-plugin

# Activate multiple plugins at once
bws r chrome@120 --plugin plugin-a,plugin-b

# Search remote plugins
bws plugin search fingerprint
```

Creating a plugin is as simple as creating a `.lua` file:

```lua
-- ~/.bws/plugins/my-plugin.lua
function pre_run()
    if ctx.browser == "chrome" then
        ctx.add_arg("--disable-background-timer-throttling")
    end
end
```

For more examples, see [plugins/examples](plugins/examples/).

## Serve Service

```bash
# First run (automatically creates configuration file)
bws serve
# Edit the bws-serve.ini configuration file

# Start the service
bws serve
```

For details, see [Serve Service Documentation](https://hyjiacan.github.io/browser-workshop/guide/serve).

## License

MIT
