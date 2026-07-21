# Browser Short Aliases

For ease of input, bws provides short aliases for common browsers, which can be used in all commands. Using short aliases can significantly reduce the amount of command input and improve operational efficiency.

## Supported Short Aliases List

| Short Alias | Full Name | Description |
|--------|----------|------|
| `gc` | chrome / googlechrome / google-chrome | Google Chrome browser |
| `ff` | firefox | Mozilla Firefox browser |
| `cm` | chromium | Chromium open-source browser |

## Usage Examples

### List Versions

```bash
# Full form
bws ls chrome

# Short alias form
bws ls gc
```

### Install Browser

```bash
# Full form
bws i chrome@latest

# Short alias form
bws i gc@latest
```

### Run Browser

```bash
# Full form
bws r firefox@120

# Short alias form
bws r ff@120
```

### Version Filtering

```bash
# Full form
bws ls chromium@79

# Short alias form
bws ls cm@79
```

### Remote List

```bash
# Full form
bws ls --remote chrome

# Short alias form
bws ls -R gc
```

### View Version Information

```bash
# Full form
bws show chrome@120

# Short alias form
bws show gc@120
```

### Set Default Version

```bash
# Full form
bws u chrome@120

# Short alias form
bws u gc@120
```

### Uninstall Version

```bash
# Full form
bws rm chrome@120

# Short alias form
bws rm gc@120
```

### Download Only (Do Not Install)

```bash
# Full form
bws dl chrome@120

# Short alias form
bws dl gc@120
```

### Profile Management

```bash
# Full form
bws pf list chrome

# Short alias form
bws pf list gc
```

## Supported Command Scope

Short aliases can be used in all bws commands, including but not limited to:

| Command | Supports Short Aliases | Example |
|------|-----------|------|
| `ls` / `list` | Yes | `bws ls gc` |
| `ls --remote` / `ls -R` | Yes | `bws ls -R ff` |
| `info` | Yes | `bws show cm@120` |
| `run` | Yes | `bws r gc@120` |
| `install` | Yes | `bws i ff@latest` |
| `import` | No | Batch import, no browser needs to be specified |
| `uninstall` | Yes | `bws rm gc@120` |
| `use` | Yes | `bws u cm@120` |
| `download` | Yes | `bws dl ff@beta` |
| `profile` | Yes | `bws pf list gc` |
| `config` | No | Configuration management command |
| `serve` | No | Server command |
| `repo` | No | Repository management command |
| `cache` | No | Cache management command |
| `doctor` | No | System check command |

## Usage Tips

### Combine with Version Numbers

Short aliases can be flexibly combined with version numbers:

```bash
bws ls gc@120          # List chrome 120.x versions
bws i ff@beta    # Install firefox beta version
bws r cm@latest      # Run the latest chromium version
```

### Combine with Channels

```bash
bws ls -R gc --channel beta     # View chrome beta channel versions
bws i ff --channel dev    # Install firefox dev version
```

### Combine with System Versions

```bash
bws r gc@system       # Run system-installed Chrome
bws show ff@system      # View system Firefox information
```

## Notes

1. Short aliases are case-insensitive; `gc` and `GC` have the same effect
2. Short aliases are only used to simplify command-line input; internal storage and display still use full names
3. If the entered name is neither a short alias nor a full browser name, an error will be reported
4. More short aliases may be added in future versions; please refer to the description in `bws help`
