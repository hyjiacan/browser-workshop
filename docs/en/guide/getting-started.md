# Getting Started

This chapter will guide you through the basic usage of bws in 5-10 minutes.

## Prerequisites

Make sure you have installed bws. If not, please refer to the [Installation Guide](./installation.md).

> **Note**: The first time you run `bws`, an initialization wizard guides you through the data directory, default browser, and offline (serve) source. Every step has an explanation; use arrow keys to select and Enter to confirm. You can also skip it and adjust later anytime with `bws config set`.

## 1. View Installed Versions

First, let's see the browser versions currently installed on the system:

```bash
bws ls
```

Example output:

```
Google Chrome (3 installed, 1 system)
  126.0.6478.114 [stable]
  121.0.6167.85
  120.0.6099.109
  125.0.6422.112 [stable] [system]
```

### Common Filtering Methods

```bash
# View only a specific browser
bws ls chrome

# Use short aliases (gc=chrome, ff=firefox, cm=chromium)
bws ls gc

# Filter by version prefix
bws ls chrome@79

# Do not show system browsers
bws ls --no-system
```

## 2. Install from Local Directory

If you already have some browser installers or portable directories, you can use the `install -d` command to install from local:

```bash
# Automatically identify and install from a directory
bws i -d /path/to/browser-dir

# Install from a directory and specify the version
bws i -d /path/to/browser-dir chrome@120
```

The installation process will display progress in real time, and unrecognizable files can be manually specified with a version identifier.

> **Tip**: Supported file formats include zip, 7z, tar.gz, tar.bz2, tar.xz, .exe, and more. Filenames are automatically recognized. For detailed rules, please refer to the [Local Installation](./import.md) chapter.

## 3. Remote Download and Install

If you don't have local installers, you can directly download and install from remote sources:

```bash
# Install the latest stable version
bws i chrome@latest

# Install a specific channel
bws i chrome@beta

# Install a specific full version
bws i chrome@120.0.6478.114

# Install partial version number (automatically matches the latest 85.x)
bws i chrome@85
```

### View Remote Available Versions

Before installing, you can first check which versions are available from the remote source:

```bash
bws ls --remote chrome
bws ls -R gc@79
bws ls -R chrome --channel beta

# Force refresh the remote source cache
bws ls -R firefox --refresh
```

The remote list will mark locally installed versions:

```
Available versions for chrome:

Version              Channel  Platform  Architecture  Status
--------------  ------  -------  ------  ------
150.0.7871.115  stable  windows  amd64
120.0.6099.109  stable  windows  x64     Installed
  79.0.3945.79  stable  windows  x64     Installed

  2 versions installed.
```

## 4. Run Browser

After installation, use the `run` command to launch the browser:

```bash
# Run a specific version
bws r chrome@120

# Run a system-installed version
bws r chrome@system

# Run the default version (set via bws u)
bws r chrome
```

### Common Run Options

```bash
# Incognito mode
bws r chrome@120 -i

# Open in new window
bws r chrome@120 -w

# Headless mode
bws r chrome@120 -H

# Specify a named Profile
bws r chrome@120 -p myprofile

# Run in background (do not wait for process)
bws r chrome@120 -d

# Open a specific URL
bws r chrome@120 https://example.com

# Pass native browser arguments
bws r chrome@120 -- --disable-gpu --no-sandbox

# Verbose (detailed) logging
bws r chrome@120 -V
```

> **Tip**: When matching partial version numbers, all matching versions will be listed and the latest version will be automatically selected. For more run options, please refer to the [Run Browser](./run.md) chapter.

## 5. Set Default Version

If you frequently use a certain version, you can set it as the default:

```bash
bws u chrome@120
```

After setting, you can run directly using the browser name without specifying the version:

```bash
bws r chrome
```

## 6. Automation Testing (Optional)

If you use frameworks such as Playwright, Puppeteer, Selenium or WebdriverIO, the `--automation` flag launches the browser, prepares a matching chromedriver, and prints the CDP and WebDriver endpoints in one command:

```bash
bws r chrome@120 --automation
```

Combined with `--daemon`, the browser runs in the background and is registered as a manageable instance:

```bash
bws r chrome@120 --automation --daemon --profile test-01

bws ps                               # list running instances
bws endpoint bws-chrome-120-test-01  # query instance endpoints
bws stop bws-chrome-120-test-01      # stop the instance
```

If a framework only needs the browser path (e.g. Cypress), use `bws where`:

```bash
cypress run --browser "$(bws where chrome@120)"
```

For more details, please refer to the [Automation Framework Integration](./automation.md) chapter.

## 7. Use the Serve Service (Team Sharing)

If you need to share browser versions within a team, you can set up the Serve service:

```bash
# First run, automatically creates the configuration file
bws sv

# Edit the bws-serve.ini configuration file
# Start the service
bws sv
```

Configure the client's offline source address:

```bash
bws cfg set source http://server-ip:8080
```

For detailed instructions, please refer to the [Serve Service](./serve.md) chapter.

## Next Steps

Congratulations on completing the bws quick start! Next, you can:

- Learn about [Browser Short Aliases](./short-aliases.md) to reduce typing
- Explore more tips for [Version Management](./version-management.md)
- Discover [Profile Management](./profile.md) features
- Integrate [Automation Frameworks](./automation.md) to drive Playwright / Selenium and more
- Configure [Offline Sources](./config.md) to speed up downloads
- Set up [Serve Service](./serve.md) for team sharing
- Learn about the [Logging System](./logging.md) for troubleshooting
