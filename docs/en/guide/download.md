# Remote Download

bws supports downloading browser versions from remote sources, including both online and offline sources. This chapter details the configuration and usage of remote downloads.

## Online and Offline Sources

bws supports two types of remote sources:

### Online Sources

Online sources are built-in official download sources for bws, directly obtaining version information and installation packages from the browser vendor's official servers.

- **Chrome**: Obtains version lists and download addresses via the Google Omaha protocol (official update protocol)
- **Firefox**: Obtains version information and download URLs via the Mozilla Product Details API
- **Other browsers**: Their respective official update channels

Characteristics of online sources:
- Latest and most complete versions
- No configuration required, works out of the box
- Requires internet access
- Download speed depends on network conditions

### Offline Sources

Offline sources are local/intranet distribution services built using `bws sv`, providing internal distribution of browser versions.

- Served by the `bws sv` command
- Stored in the local `packages/` directory
- Supports automatic synchronization of versions from online sources
- Suitable for team internal or offline environments

Characteristics of offline sources:
- Fast download speed (within LAN)
- Can be used offline
- Requires manual setup and maintenance
- Versions depend on synchronization status

## Source Priority

bws adopts a fixed source priority strategy:

```
Offline Source → Online Source
```

That is, **offline source takes priority, online source as fallback**.

### Priority Rules

1. If an offline source is configured (`source` config item), query and download from the offline source first
2. If the version is not found in the offline source, automatically fall back to the online source
3. If neither source has the version, an error is reported

### Priority Example

Assuming an offline source `http://server:8080` is configured, when executing `bws i chrome@120`:

1. First query whether `http://server:8080` has the chrome 120 version
2. If yes, download from the offline source (faster)
3. If not, download from the Google Omaha online source
4. If the online source also doesn't have it, report "Version not found"

## Configuring Offline Sources

Use the `bws cfg` command to configure the offline source address.

### Set Offline Source

```bash
bws cfg set source http://server:8080
```

You can also use the `remote-source` config item (equivalent to `source`):

```bash
bws cfg set remote-source http://server:8080
```

### View Current Source

```bash
bws cfg get source
```

### Clear Offline Source Configuration

```bash
bws cfg set source ""
```

After clearing, only the online source will be used.

### Client Configuration Steps

1. Ensure the server has started `bws sv`
2. Execute the configuration command on the client:

```bash
bws cfg set source http://server-ip:8080
```

3. Verify that the configuration is effective:

```bash
bws ls -R chrome
```

If the version list can be obtained from the offline source, the configuration is successful.

## Listing Remote Versions

Use the `ls --remote` or `ls -R` command to list remote available browser versions.

### Basic Usage

```bash
# List remote versions for all browsers
bws ls --remote
bws ls -R

# List remote versions for a specific browser
bws ls -R chrome
bws ls -R gc
```

### Specify Channel

```bash
bws ls -R chrome --channel beta
bws ls -R ff --channel dev
```

### Version Prefix Filtering

```bash
bws ls -R chrome@79
bws ls -R gc@120
```

### Example Output

```
Available versions for chrome:

Version              Channel      Platform       Architecture      Status
--------------  ------  -------  ------  ------
150.0.7871.115  stable  windows  amd64
120.0.6099.109  stable  windows  x64     Installed
  79.0.3945.79  stable  windows  x64     Installed
148.0.7778.167  beta    windows  amd64

  2 versions installed.
```

Output description:
- Table lists version number, channel, platform, architecture, and status
- "Installed" marker indicates that the version is already installed locally
- Versions are sorted by version number from high to low

## Download and Install

Use the `install` command to download and install browser versions from remote sources.

### Basic Usage

```bash
# Install the latest stable version
bws i chrome@latest

# Install the latest version of a specified channel
bws i chrome@beta
bws i chrome@dev
bws i chrome@canary

# Install a specific full version
bws i chrome@120.0.6478.114

# Install a partial version number (automatically matches the latest matching version)
bws i chrome@85
```

### Installation Process

1. Query the remote source for version information
2. Download the installation package to the cache directory
3. Verify file integrity
4. Extract and install to the versions directory
5. Update the version manifest

The download process displays a progress bar and download speed.

### Using Short Aliases

```bash
bws i gc@latest     # chrome
bws i ff@beta       # firefox
bws i cm@120        # chromium
```

## Download Only (Do Not Install)

Use the `download` command to download the installation package without installing it, suitable for scenarios where you need to cache installation packages or handle them manually.

### Basic Usage

```bash
# Download the latest stable version
bws dl chrome@latest

# Download a specific version
bws dl chrome@120.0.6478.114

# Download a partial version number
bws dl chrome@85
```

### Download File Location

Downloaded files are saved in the current working directory by default. You can specify another save path via `bws dl --output`.

### Difference from install

| Feature | `download` | `install` |
|---------|-----------|-----------|
| Download file | Yes | Yes |
| Install to versions | No | Yes |
| Can run directly | No | Yes |
| Space usage | Only archive | Archive + extracted files |

### Usage Scenarios

- Pre-download multiple versions for subsequent offline installation
- Download installation packages for other uses
- Batch download for setting up offline sources

Downloaded installation packages can be installed locally via the `bws i --from-file` command.
