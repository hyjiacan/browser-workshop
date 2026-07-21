# Version Management

bws provides comprehensive browser version management features, including listing versions, viewing details, setting default versions, uninstalling versions, and more. This chapter details how to use these features.

## Listing Versions

### List All Installed Versions

Use the `ls` or `list` command to list all installed browser versions:

```bash
bws ls
bws ls
```

Example output:

```
Google Chrome (3 installed, 1 system)
  126.0.6478.114 [stable]
  121.0.6167.85
  120.0.6099.109
  125.0.6422.112 [stable] [System]

Mozilla Firefox (2 installed, 0 system)
  121.0 [stable]
  115.0esr [esr]
```

### List Versions for a Specific Browser

You can specify a browser name to list only versions of that browser:

```bash
bws ls chrome
bws ls firefox
```

### Using Short Aliases

Short aliases are supported to simplify input:

```bash
bws ls gc      # chrome
bws ls ff      # firefox
bws ls cm      # chromium
```

### List All Versions (Including Remote)

Use the `--all` parameter to list versions from all sources (local and remote):

```bash
bws ls --all
bws ls --all chrome
```

### Hide System Browsers

Use the `--no-system` parameter to hide system-installed browser versions:

```bash
bws ls --no-system
```

### Version Listing Comparison

| Command | Description |
|---------|-------------|
| `bws ls` | List locally installed versions (including system) |
| `bws ls --all` | List all versions (local + remote) |
| `bws ls --no-system` | Do not display system browsers |
| `bws ls --remote` / `bws ls -R` | List remote available versions |

## Partial Version Number Filtering

bws supports filtering using partial version numbers. Just enter a prefix of the version number to match all versions starting with that prefix.

### Basic Usage

```bash
# List all chrome 79.x versions
bws ls chrome@79

# List all chrome 120.0 versions
bws ls chrome@120.0

# List all firefox 115 versions
bws ls ff@115
```

### Matching Rules

- Entering `chrome@79` will match `79.0.3945.79`, `79.0.3945.130`, and all other 79.x versions
- Entering `chrome@120.0` will match `120.0.6099.109`, `120.0.6099.112`, and similar versions
- Matching results are sorted by version number from high to low
- When a specific version needs to be selected (e.g., with the `run` command), the latest matching version is selected by default

### Partial Version Number Matching Output Example

Output when running `bws r chrome@85`:

```
Matching versions for chrome@85:
> 85.0.4183.121
  85.0.4183.83
  85.0.4183.10
```

The `>` marker indicates the currently selected version (the latest one).

### Supported Commands

Partial version number filtering can be used in the following commands:

- `bws ls` - List versions
- `bws show` - View version details
- `bws r` - Run browser
- `bws i` - Install version
- `bws u` - Set default version
- `bws rm` - Uninstall version
- `bws dl` - Download only
- `bws pf` - Profile management

## Viewing Version Details

Use the `info` command to view detailed information about a specified version:

```bash
bws show chrome@120
bws show chrome@120.0.6099.109
bws show gc@system
```

The output typically includes:

- Browser name and version number
- Release channel (stable, beta, dev, canary, esr, etc.)
- Installation path
- Architecture information
- Profile path
- Executable file path
- Installation source (local import, remote download, system installation)

## Setting the Default Version

Use the `use` command to set the default browser version. After setting, you can run the browser directly by name without specifying a version.

### Set Default Version

```bash
bws u chrome@120
bws u ff@121
```

### Using the Default Version

After setting the default version, you can run it directly:

```bash
bws r chrome
bws r firefox
```

### View Current Default Version

```bash
bws cfg get default-browser
```

### Support for Partial Version Numbers

The `use` command also supports partial version numbers and will automatically select the latest matching version:

```bash
bws u chrome@120     # Automatically selects the latest 120.x version
```

## Uninstalling Versions

Use the `uninstall` command to uninstall a specified browser version.

### Basic Usage

```bash
bws rm chrome@120
bws rm chrome@120.0.6099.109
```

### Support for Partial Version Numbers

```bash
bws rm chrome@85     # Uninstalls the latest 85.x version
```

### Notes

1. Uninstalling will delete browser program files, but will not delete the corresponding Profile data
2. If you need to delete Profile data as well, use the `bws pf reset` command
3. System-installed browsers cannot be uninstalled via bws
4. A confirmation prompt will be displayed before uninstalling; deletion will only proceed after confirmation
5. After uninstalling, residual Profile data can be cleaned up via `bws pf clean`

### Uninstall Confirmation

When executing the uninstall command, a confirmation prompt will be displayed:

```
Are you sure you want to uninstall chrome 120.0.6099.109? (y/N)
```

Enter `y` to confirm the uninstallation, or enter `n` or press Enter directly to cancel.

## Architecture Compatibility

bws automatically detects the system architecture and filters out incompatible browser versions:

- **x64 systems**: Can run x64 and x86 versions
- **x86 systems**: Can only run x86 versions
- **arm64 systems**: Can only run arm64 versions

When listing versions and importing versions, incompatible architecture versions are automatically hidden to prevent misuse.
