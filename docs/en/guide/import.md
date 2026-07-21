# Local Import

bws supports importing browser versions from local directories or files, which is suitable for scenarios where you already have browser installation packages or portable versions. This chapter details the various methods and rules for local import.

## Batch Import from Directory

Use the `import` command to batch import browser versions from a specified directory, automatically identifying all recognizable browser files in the directory.

### Basic Usage

```bash
# Automatically identify all browser versions in the directory
bws imp /path/to/browsers
```

### Force Re-import

By default, already installed versions will be skipped. Use the `-f` parameter to force re-import:

```bash
bws imp /path/to/browsers -f
```

### Import Process

The import process displays real-time progress, including:

- The file currently being processed
- The recognized browser name and version
- Import progress percentage
- Success/failure status

Unrecognizable files will be prompted immediately, but the overall import process will not be interrupted.

### Example Output

```
Scanning directory: D:\browsers
Found 5 files, starting identification...

✓ Chrome_120.0.6099.109_Windows_x64.exe → chrome 120.0.6099.109
✓ firefox-121.0-win64.zip → firefox 121.0
✗ unknown_setup.exe → Unable to recognize
✓ chrome-79.0.3945.79.zip → chrome 79.0.3945.79
✓ chrome-79.0.3945.79.tar.gz → chrome 79.0.3945.79

Import complete: 4 successful, 1 failed
```

## Install from Directory

Use the `install -d` command to install from a single directory, suitable for cases where there is only one browser version directory.

### Basic Usage

```bash
# Automatically identify and install from directory
bws i -d /path/to/browser-dir

# Specify version number for installation (when auto-recognition fails)
bws i -d /path/to/browser-dir chrome@120
```

### Applicable Scenarios

- Portable browser directories
- Extracted browser installation directories
- Manually built browser directories

## Install from File

Use the `install -f` command to install from a single file, supporting both archive and installer formats.

### Basic Usage

```bash
# Install from archive (auto-recognition)
bws i -f /path/to/chrome-setup.exe
bws i -f /path/to/chrome.zip

# Specify version number for installation (when filename cannot be recognized)
bws i -f /path/to/file.exe chrome@120
```

### Difference from import

| Feature | `import` | `install -d` / `install -f` |
|---------|----------|---------------------------|
| Processing quantity | Batch (all files in directory) | Single (one directory or file) |
| Auto-recognition | Yes | Yes (can be manually specified) |
| Force re-import | Supports `-f` | Reinstalls every time |
| Applicable scenarios | Bulk import | Single version installation |

## Supported File Formats

bws supports 25+ file formats, mainly including the following categories:

### Archive Formats

| Format | Extension | Description |
|--------|-----------|-------------|
| ZIP | `.zip` | Most common compression format |
| 7-Zip | `.7z` | High compression ratio format |
| Tar | `.tar` | Unix archive format |
| Tar+Gzip | `.tar.gz`, `.tgz` | Common Unix compression format |
| Tar+Bzip2 | `.tar.bz2`, `.tbz2` | High compression ratio format |
| Tar+XZ | `.tar.xz`, `.txz` | High compression ratio format |
| Tar+Zstd | `.tar.zst` | zstd compression format |

### Executable File Formats

| Format | Extension | Description |
|--------|-----------|-------------|
| Windows Executable | `.exe` | Self-extracting installer (extracted as zip), does not execute installation |

### Directory Format

Directories directly containing browser executable files can also be recognized and imported.

## Filename Auto-Recognition Rules

bws automatically recognizes browser information from keywords in filenames or directory names, including browser name, version number, platform, architecture, and channel.

### Recognition Elements

| Element | Recognition Keywords | Examples |
|---------|---------------------|----------|
| **Browser Name** | chrome, chromium, firefox | `chrome`, `firefox` |
| **Version Number** | Numeric combinations like `x.y.z.w` | `120.0.6099.109` |
| **Architecture** | `win64`/`x64`/`amd64` or `win32`/`x86`/`386` | `x64`, `win64` |
| **Platform** | `windows`/`win`, `macos`/`mac`, `linux` | `windows`, `win` |
| **Channel** | `stable`, `beta`, `dev`, `canary`, `esr` | `stable`, `beta` |

### Filename Examples

Here are some filename examples that can be correctly recognized:

```
44.0.2403.107_chrome64_stable_windows_installer.exe
GoogleChrome_148.0.7778.167_Windows_x64_Offline.exe
firefox-115.0esr-win64.zip
Chrome_120.0.6099.109_Windows_x64.zip
chromium-85.0.4183.121-linux-x64.tar.gz
firefox-121.0-x64.tar.bz2
```

### Recognition Priority

1. First, recognize the browser name
2. Then extract the version number
3. Next, recognize architecture, platform, and channel
4. If some information is missing, use default values (e.g., default stable channel)

## Handling Unrecognized Files

If a filename or directory name cannot be automatically recognized, you can install it by manually specifying the version.

### Specify Version for Installation

When using the `install -d` or `install -f` command, append the version identifier at the end:

```bash
# Install from directory, manually specify version
bws i -d /path/to/dir chrome@120

# Install from file, manually specify version
bws i -f /path/to/file.exe chrome@120
```

### Version Identifier Format

The version identifier format is `browser@version`:

- `browser`: Browser name (e.g., chrome, firefox, chromium) or short alias (gc, ff, cm)
- `version`: Version number, can be a full or partial version number

Examples:

```bash
bws i -f ./mystery.exe gc@120.0.6099.109
bws i -d ./my-browser ff@115.0esr
```

### Specify Channel and Architecture

If more precise specification is needed, you can also use other parameters:

```bash
bws i -f ./file.exe chrome@120 --channel beta
```

### Common Reasons for Unrecognition

1. No obvious browser name keyword in the filename
2. Non-standard version number format
3. Missing architecture or platform identifier (system defaults will be used)
4. Custom-named files cannot match the rules

When encountering unrecognizable files, it is recommended to install them using the manual version specification method.
