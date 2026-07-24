# Local Installation

bws supports installing browser versions from local directories or files, which is suitable for scenarios where you already have browser installation packages or portable versions. This chapter details the various methods and rules for local installation.

## Install from Directory

Use the `install -d` command to install from a local directory, automatically identifying the browser version in the directory.

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

Use the `install --from-file` command to install from a single file, supporting both archive and installer formats.

### Basic Usage

```bash
# Install from archive (auto-recognition)
bws i --from-file /path/to/chrome-setup.exe
bws i --from-file /path/to/chrome.zip

# Specify version number for installation (when filename cannot be recognized)
bws i --from-file /path/to/file.exe chrome@120
```

## Supported File Formats

bws supports zip, 7z, tar.gz, tar.bz2, tar.xz, tar.zst, gz, bz2, xz, zst, exe, dmg, cab, apk, jar, war, and more file formats, mainly including the following categories:

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
| Gzip | `.gz` | Gzip single-file compression |
| Bzip2 | `.bz2` | Bzip2 single-file compression |
| XZ | `.xz` | XZ single-file compression |
| Zstd | `.zst` | Zstd single-file compression |

### Executable and Special Formats

| Format | Extension | Description |
|--------|-----------|-------------|
| Windows Executable | `.exe` | Self-extracting installer (extracted as zip), does not execute installation |
| macOS Disk Image | `.dmg` | macOS disk image (zip-based) |
| Windows Cabinet | `.cab` | Windows Cabinet compressed file |

### Other Archive Formats

| Format | Extension | Description |
|--------|-----------|-------------|
| Android APK | `.apk` | Android application package (zip-based) |
| Java JAR | `.jar` | Java archive file (zip-based) |
| Web WAR | `.war` | Web application archive (zip-based) |

> **Magic Byte Detection**: In addition to file extensions, bws can also detect archive formats by their file header magic bytes. For example, the PK signature for ZIP files, the `7z` signature for 7z files, and so on. Therefore, even if a file lacks the proper extension, bws can still correctly identify and process it based on its file header characteristics.

### Directory Format

Directories directly containing browser executable files can also be recognized and installed.

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

When using the `install -d` or `install --from-file` command, append the version identifier at the end:

```bash
# Install from directory, manually specify version
bws i -d /path/to/dir chrome@120

# Install from file, manually specify version
bws i --from-file /path/to/file.exe chrome@120
```

### Version Identifier Format

The version identifier format is `browser@version`:

- `browser`: Browser name (e.g., chrome, firefox, chromium) or short alias (gc, ff, cm)
- `version`: Version number, can be a full or partial version number

Examples:

```bash
bws i --from-file ./mystery.exe gc@120.0.6099.109
bws i -d ./my-browser ff@115.0esr
```

### Specify Channel and Architecture

If more precise specification is needed, you can also use other parameters:

```bash
bws i --from-file ./file.exe chrome@120 --channel beta
```

### Common Reasons for Unrecognition

1. No obvious browser name keyword in the filename
2. Non-standard version number format
3. Missing architecture or platform identifier (system defaults will be used)
4. Custom-named files cannot match the rules

When encountering unrecognizable files, it is recommended to install them using the manual version specification method.
