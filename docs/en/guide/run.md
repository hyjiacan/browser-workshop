# Running Browsers

The `run` command in bws is used to launch a specified browser version, supporting multiple run modes and parameter options. This chapter details the various ways to run browsers.

## Basic Usage

### Run a Specified Version

```bash
# Run a specific full version
bws r chrome@120.0.6099.109

# Run a partial version number (automatically selects the latest matching version)
bws r chrome@120
```

### Run the System Version

Run a browser version already installed on the system:

```bash
bws r chrome@system
```

### Run the Default Version

Run the default version set via `bws u`:

```bash
bws r chrome
```

If no default version is set, an error will be reported.

### Using Short Aliases

```bash
bws r gc@120       # chrome
bws r ff           # firefox default version
bws r cm@latest    # chromium latest version
```

### Open a Specified URL

Directly open a specified URL when running the browser:

```bash
bws r chrome@120 https://example.com
bws r gc https://github.com
```

## Headless Mode

Use the `-H` or `--headless` parameter to run the browser in headless mode, suitable for automated testing and scripting scenarios.

```bash
bws r chrome@120 -H
bws r chrome@120 --headless
```

In headless mode, the browser will not display a graphical interface, and all operations are completed in the background. Commonly used for:

- Automated testing
- Webpage screenshots
- Performance testing
- Crawler scripts

## Incognito Mode

Use the `-i` or `--incognito` parameter to run the browser in incognito/private mode.

```bash
bws r chrome@120 -i
bws r chrome@120 --incognito
```

Characteristics of incognito mode:

- Does not save browsing history
- Does not save cookies and site data
- Does not save form data
- Automatically clears session data after closing the browser

## New Window

Use the `-w` or `--new-window` parameter to force the browser to open in a new window.

```bash
bws r chrome@120 -w
bws r chrome@120 --new-window
```

Even if a browser of that version is already running, a new window will be opened.

## Named Profile

Use the `-p` or `--profile` parameter to specify a named Profile when running the browser.

```bash
bws r chrome@120 -p myprofile
bws r chrome@120 --profile work
```

### Characteristics of Named Profiles

- The same named Profile can be shared across different versions
- Each named Profile has an independent data directory
- Suitable for distinguishing different scenarios such as work, personal, and testing

### Examples

```bash
# Work Profile
bws r chrome@120 -p work

# The same work Profile can also be used on version 121
bws r chrome@121 -p work

# Test Profile
bws r chrome@120 -p test
```

For more Profile management features, please refer to the [Profile Management](./profile.md) chapter.

## Native Mode

Use the `--native` parameter to run the browser in native mode, i.e., without using a bws-managed Profile, directly using the system's default user data directory.

```bash
bws r chrome@120 --native
```

Characteristics of native mode:

- Uses the system's default browser user data directory
- Same effect as directly running the browser
- Different versions may share the same Profile
- Suitable for scenarios requiring consistency with the system browser

> **Note**: In native mode, different versions sharing a Profile may cause configuration conflicts or data corruption; use with caution.

### Standard Isolation Parameters

In non-native mode, bws automatically adds the following standard parameters for all browsers (not added in native mode):

| Parameter | Chrome/Chromium | Firefox |
|-----------|----------------|---------|
| Multi-instance isolation | Via separate Profile directory | `-no-remote` |
| Disable update check | `--disable-update` | `user.js`: `app.update.enabled=false` |
| Disable default browser check | `--no-default-browser-check` | `user.js`: `browser.shell.checkDefaultBrowser=false` |
| Skip first-run wizard | `--no-first-run` | - |

Firefox preferences are written to the Profile directory via `user.js` file, with dedup markers to prevent duplicate writes.

## Background Run

Use the `-d` or `--detached` parameter to let the browser run in the background, the bws command returns immediately without waiting for the browser process to end.

```bash
bws r chrome@120 -d
bws r chrome@120 --detached
```

### Usage Scenarios

- Start the browser in a script and continue executing other operations
- No need to wait for the browser to close
- Background services in automated scripts

By default (without `-d`), the `bws r` command waits for the browser process to end before returning.

## Dry Run

Use the `--dry-run` parameter to perform a dry run, only displaying the command that would be executed without actually starting the browser.

```bash
bws r chrome@120 --dry-run
```

### Example Output

```
Command to be executed:
C:\bws\bws-data\versions\chrome\120.0.6099.109\chrome.exe --user-data-dir=C:\bws\bws-data\runtime\chrome\120.0\profile
```

Uses of dry run:

- Debug command parameters
- Confirm browser path and startup parameters
- View Profile directory location
- Verify whether parameter passing is correct

## Passing Native Browser Parameters

Use the `--` separator to pass native command-line parameters to the browser. All parameters after `--` are passed to the browser as-is.

```bash
bws r chrome@120 -- --disable-gpu --no-sandbox
bws r chrome@120 -i -- --window-size=1920,1080
bws r ff -- --private-window
```

### Common Native Parameter Examples

Chrome common parameters:

```bash
# Disable GPU acceleration
bws r chrome@120 -- --disable-gpu

# Disable sandbox
bws r chrome@120 -- --no-sandbox

# Specify window size
bws r chrome@120 -- --window-size=1920,1080

# Specify launch position
bws r chrome@120 -- --window-position=0,0

# Disable extensions
bws r chrome@120 -- --disable-extensions

# Maximize on startup
bws r chrome@120 -- --start-maximized
```

Firefox common parameters:

```bash
# Private window
bws r firefox -- --private-window

# Safe mode
bws r firefox -- --safe-mode
```

## Partial Version Number Matching

When using a partial version number, bws will list all matching versions and automatically select the latest version.

### Matching Output Example

```
Matching versions for chrome@85:
> 85.0.4183.121
  85.0.4183.83
  85.0.4183.10
```

The `>` marker indicates the currently selected version (the latest one).

## Automation Mode

With the `--automation` option, bws prepares an automation driver (chromedriver) matching the launched browser version and prints the WebDriver endpoint, so frameworks such as Selenium and WebdriverIO can attach.

```bash
bws r chrome@120 --automation
```

Flow:

1. Resolve the Chrome version being launched (e.g. `120.0.6099.109`)
2. Look up the matching chromedriver in the Chrome for Testing manifest
3. Download and extract it to `bws-data/drivers/chromedriver/120/`
4. Start chromedriver, listen on a port, and print the WebDriver endpoint

Example output:

```
WebDriver:  http://127.0.0.1:9515
Driver PID: 12345
```

### Related Options

| Option | Description |
|--------|-------------|
| `--webdriver` | Subset of `--automation`, only exposes the WebDriver endpoint |
| `--driver-port <port>` | Fixed driver listen port; `0` picks a free port |
| `--driver-no-download` | Do not auto-download the driver; warn only if missing |

```bash
# Fixed driver port
bws r chrome@120 --automation --driver-port 9515

# Enable the WebDriver endpoint only
bws r chrome@120 --webdriver

# Do not auto-download the driver (use an installed one only)
bws r chrome@120 --automation --driver-no-download
```

### Notes

- Automation mode only supports Chrome/Chromium (chromedriver only drives a matching Chrome/Chromium major)
- A driver start failure never blocks the browser launch; it only warns on stderr
- When a serve source is configured, drivers are resolved and downloaded through it first
- Drivers can also be managed separately with `bws driver` (see the [commands reference](./commands.md#bws-driver-alias-drv))

## Run Options Summary

| Option | Short | Description |
|--------|-------|-------------|
| `--headless` | `-H` | Headless mode |
| `--incognito` | `-i` | Incognito/private mode |
| `--new-window` | `-w` | Open in new window |
| `--profile <name>` | `-p` | Specify named Profile |
| `--native` | `-n` | Native mode (use system Profile) |
| `--detached` | `-d` | Run in background (do not wait for process) |
| `--dry-run` | - | Dry run (do not actually start) |
| `--automation` | - | Automation mode (manage chromedriver automatically) |
| `--webdriver` | - | Enable the WebDriver endpoint only |
| `--driver-port <port>` | - | Driver listen port |
| `--driver-no-download` | - | Do not auto-download the driver |
| `--` | - | Pass native browser parameters |

## Combined Usage Examples

Multiple options can be combined:

```bash
# Headless mode + named Profile + native parameters
bws r chrome@120 -H -p test -- --disable-gpu --no-sandbox

# Incognito mode + new window + open URL
bws r chrome@120 -i -w https://example.com

# Background run + native mode
bws r chrome@system -d --native
```
