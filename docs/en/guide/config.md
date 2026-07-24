# Configuration Management

All bws configurations are managed uniformly through the `bws cfg` command. This chapter introduces how to view and set configurations, as well as detailed descriptions of each configuration item.

## View All Configurations

Use the `bws cfg show` command to view all current configuration items and their values.

```bash
bws cfg show
```

Example output:

```
Configuration information:

  Config file:       D:\bws-data\config.json
  Data directory:       D:\bws-data
  Default browser:     chrome
  Default channel:       stable
  Log level:       info
  Repository path:       (empty)

  Data source switches:
    Serve source:     true
    Omaha source:     true
    Firefox FTP:  true

  Disk space threshold:   5 GB (prompts when below this value)

  Proxy:           (not set, direct)
  Language:       zh

  Aliases:
    stable -> chrome@latest
    beta -> chrome@beta
```

## Get a Configuration Item

Use the `bws cfg get` command to get the value of a single configuration item.

```bash
bws cfg get default-browser
bws cfg get log-level
bws cfg get source
```

Example output:

```
chrome
```

Without specifying a key name, all readable configuration items and their aliases will be listed:

```bash
bws cfg get
```

## Set a Configuration Item

Use the `bws cfg set` command to set the value of a configuration item.

```bash
bws cfg set default-browser firefox
bws cfg set log-level debug
bws cfg set data-dir D:\browser-data
bws cfg set source http://server:8080
```

A confirmation message will be displayed after successful setting.

Without specifying a key name or value, all writable configuration items, aliases, and example values will be listed:

```bash
bws cfg set
```

## Configuration Item Descriptions

Each configuration item supports multiple equivalent key names that can be used interchangeably:

| Primary key | Aliases |
|-------------|---------|
| `default-browser` | `default`, `browser` |
| `default-channel` | `channel` |
| `log-level` | `log` |
| `data-dir` | `datadir`, `data` |
| `repo-path` | `repo` |
| `source` | `remote-source`, `remote` |
| `source-serve` | `serve-source` |
| `source-omaha` | `omaha-source` |
| `source-firefox-ftp` | `firefox-ftp` |
| `disk-threshold` | `disk-space-threshold`, `space-threshold` |
| `proxy` | — |
| `language` | `lang` |
| `path` | `config-path`, `config` |

For example, `bws cfg get log` is equivalent to `bws cfg get log-level`, and `bws cfg set browser firefox` is equivalent to `bws cfg set default-browser firefox`.

### default-browser

Default browser name.

| Attribute | Value |
|------|-----|
| Default value | `chrome` |
| Optional values | `chrome`, `firefox`, `chromium` |
| Description | The default value used when no browser is specified in the command |

Example:

```bash
# Set the default browser to Firefox
bws cfg set default-browser firefox

# After setting, the following command runs the default version of Firefox
bws r
```

### default-channel

Default release channel.

| Attribute | Value |
|------|-----|
| Default value | `stable` |
| Optional values | `stable`, `beta`, `dev`, `canary`, `esr` |
| Description | The default channel used when no channel is specified in the command |

Example:

```bash
# Set the default channel to beta
bws cfg set default-channel beta

# After setting, installing the latest version defaults to the beta channel
bws i chrome@latest
```

### log-level

Console log level.

| Attribute | Value |
|------|-----|
| Default value | `info` |
| Optional values | `debug`, `info`, `warn`, `warning`, `error` |
| Description | Controls the verbosity of console output |

Example:

```bash
# Set to debug level for more debug information
bws cfg set log-level debug

# Set to warn level to only display warnings and errors
bws cfg set log-level warn
```

> **Note**: This configuration only affects console output. File logs always use the `debug` level and are not affected by this configuration. For more information, please refer to the [Logging System](./logging.md) chapter.

### data-dir

Data storage directory.

| Attribute | Value |
|------|-----|
| Default value | Empty (in portable mode, uses the `bws-data/` directory at the same level as the program) |
| Optional values | Any directory path |
| Description | Sets the data storage directory for bws (configuration, versions, cache, logs, etc.); after setting, all data will be stored in the specified directory |

Example:

```bash
# Set a custom data directory
bws cfg set data-dir D:\browser-data

# View the current data directory
bws cfg get data-dir

# Restore defaults (clear to return to portable mode)
bws cfg set data-dir ""
```

> **Note**: If this configuration item is set, all data (including configuration, installed versions, cache, and logs) will be stored in the specified directory. When not set, the `bws-data/` directory at the same level as the program is used by default.

### repo-path

Local repository path.

| Attribute | Value |
|------|-----|
| Default value | Empty |
| Optional values | Local directory path |
| Description | Path to the local binary repository, used for additional version sources |

Example:

```bash
bws cfg set repo-path D:\browser-repo
```

### source / remote-source

Offline source address (bws sv service address).

| Attribute | Value |
|------|-----|
| Default value | Empty (offline source not used) |
| Optional values | HTTP URL |
| Description | Address of the offline distribution service; after configuration, versions are obtained from this source first |

Example:

```bash
# Set offline source
bws cfg set source http://192.168.1.100:8080

# View current source
bws cfg get source

# Clear offline source configuration
bws cfg set source ""
```

`source` and `remote-source` are equivalent; setting either one works.

### source-omaha

Chrome Omaha data source switch.

| Attribute | Value |
|------|-----|
| Default value | `true` |
| Optional values | `true`, `false` |
| Description | Whether to enable the Chrome Omaha protocol data source |

Example:

```bash
# Disable Omaha source
bws cfg set source-omaha false

# Re-enable
bws cfg set source-omaha true
```

### source-firefox-ftp

Firefox data source switch (reserved, not currently used).

| Attribute | Value |
|------|-----|
| Default value | `true` |
| Optional values | `true`, `false` |
| Description | Whether to enable the Firefox data source (reserved; Firefox currently uses the Mozilla Product Details API instead of FTP) |

Example:

```bash
# Disable Firefox data source
bws cfg set source-firefox-ftp false
```

### source-serve

Serve HTTP data source switch.

| Attribute | Value |
|------|-----|
| Default value | `true` |
| Optional values | `true`, `false` |
| Description | Whether to enable the HTTP distribution data source built by `bws sv` |

Example:

```bash
# Disable Serve source
bws cfg set source-serve false
```

### disk-threshold

Disk space alert threshold.

| Attribute | Value |
|------|-----|
| Default value | `5` (GB) |
| Optional values | Positive integer (unit: GB) |
| Description | Check remaining disk space before downloading; prompts the user when below this value |

Example:

```bash
# Set to 10 GB
bws cfg set disk-threshold 10
```

### language / lang

Interface language setting.

| Attribute | Value |
|------|-----|
| Default value | Auto-detected (reads `LANG`/`LANGUAGE` environment variables first, defaults to Chinese if not set) |
| Optional values | `zh` (Chinese), `en` (English) |
| Description | Sets the interface display language for bws, supporting Chinese and English |

Example:

```bash
# Set to English
bws cfg set language en

# Use short alias
bws cfg set lang en

# Set to Chinese
bws cfg set language zh

# View the current language
bws cfg get language
```

> **Note**: The language configuration takes effect after restart. External translation file overrides are supported: create a JSON file at `<dataDir>/i18n/<lang>.json` to override built-in translations, allowing custom terminology.

## Data Sources and Priority

bws supports multiple version data sources, queried in a fixed priority order:

### Data Source List

| Priority | Data Source | Description | Configuration Method |
|--------|--------|------|----------|
| 1 (Highest) | Offline Source | Distribution service built via `bws sv` | `bws cfg set source <url>` |
| 2 (Lowest) | Built-in Online Source | Browser official update channels (Firefox FTP, Chromium GCS) | Built-in, no configuration needed |

### Manually Downloading Chrome Historical Versions

Chrome does not provide public download links; the following third-party sites allow manual download of historical versions:

- **ChromeDownloads**: https://chromedownloads.net/ — Provides historical version downloads for Chrome on all platforms

After downloading, you can install via the following methods:

```bash
# Install from archive
bws i --from-file chrome-120.0.6099.109-win64.zip chrome@120

# Install from extracted directory
bws i -d D:\chrome-120-win64 chrome@120
```

### Priority Rules

Fixed priority: **Offline Source → Online Source** (offline source takes priority, online source as fallback).

### Workflow

When executing `bws i` or `bws ls --remote`:

1. If an offline source is configured, query the offline source first
2. If the offline source has a matching version, use it directly (or prompt the user to select)
3. If the offline source does not have a matching version, automatically fall back to the online source
4. If the online source finds a matching version, use the online source
5. If neither source finds it, report an error

### Advantages

- **Offline environment**: After configuring an offline source, browsers can be installed even without internet access
- **Accelerated downloads**: Intranet download speeds are much faster than the internet
- **Version control**: Administrators can control the browser versions used by the team
- **Automatic fallback**: Versions not available in the offline source are automatically obtained from the online source, without affecting usage

## Configuration File

Configurations are stored in the `config.json` file under the data directory in JSON format:

```
bws-data/
└── config.json
```

It is usually not necessary to manually edit the configuration file; it is recommended to use the `bws cfg` command for management.

## Configuration Command Summary

| Command | Description |
|------|------|
| `bws cfg show` | View all configurations |
| `bws cfg get <key>` | Get the value of a specified configuration item |
| `bws cfg set <key> <value>` | Set the value of a specified configuration item |
