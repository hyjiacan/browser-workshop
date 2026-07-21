# Getting Started

This chapter will guide you through the basic usage of bws in 5-10 minutes.

## Prerequisites

Make sure you have installed bws. If not, please refer to the [Installation Guide](./installation.md).

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

## 2. Import from Local Directory

If you already have some browser installers or portable directories, you can use the `import` command to batch import them:

```bash
# Automatically identify all browser versions in the directory
bws imp /path/to/browsers

# Force re-import of already installed versions
bws imp /path/to/browsers -f
```

The import process will display progress in real time, and unrecognizable files will be prompted immediately.

> **Tip**: Supported file formats include zip, 7z, tar.gz, tar.bz2, tar.xz, .exe, and more. Filenames are automatically recognized. For detailed rules, please refer to the [Local Import](./import.md) chapter.

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

## Next Steps

Congratulations on completing the bws quick start! Next, you can:

- Learn about [Browser Short Aliases](./short-aliases.md) to reduce typing
- Explore more tips for [Version Management](./version-management.md)
- Discover [Profile Management](./profile.md) features
- Configure [Offline Sources](./config.md) to speed up downloads
- Set up [Serve Service](./serve.md) for team sharing
