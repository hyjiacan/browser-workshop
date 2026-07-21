# Logging System

bws adopts a dual-output logging system, with file and console levels controlled independently, preserving complete debugging information while keeping console output concise.

## Dual-Output Logging System

The bws logging system outputs to two places simultaneously: file and console, with independent log levels for each.

### Output Comparison

| Output | Default Level | Content Characteristics | Location |
|--------|---------------|------------------------|----------|
| File log | `DEBUG` | Timestamp + level + source location + detailed information | `logs/bws.log` |
| Console | `INFO` | Concise user-visible information | stderr (standard error output) |

### Design Philosophy

- **File log**: Completely records all operations, used for troubleshooting and post-hoc analysis
- **Console**: Only outputs information the user cares about, keeping the interface clean
- **Independent configuration**: The two levels are independent; changing the console level does not affect the completeness of the file log

## Log Level Descriptions

bws supports 6 log levels, from low to high:

| Level | Description | Example Scenarios | File Default | Console Default |
|-------|-------------|-------------------|--------------|-----------------|
| `trace` | Most detailed trace information | Function calls, variable values | - | - |
| `debug` | Debug information | HTTP requests, file operation paths | ✓ | - |
| `info` | General operation information | Install start, download progress | ✓ | ✓ |
| `warn` | Warning information | Abnormalities that do not affect operation | ✓ | ✓ |
| `error` | Error information | Operation failure, network errors | ✓ | ✓ |
| `fatal` | Fatal errors | Program crash, unrecoverable | ✓ | ✓ |

### Output Content by Level

#### trace

The most detailed log level, typically used for in-depth debugging:

- Function entry and exit
- Variable value changes
- Loop iteration information
- Low-level library calls

#### debug

Debug information, used for troubleshooting:

- HTTP request and response details
- File read/write paths
- Configuration loading process
- Version matching process
- Parsing and recognition process

#### info

General operation information, which users typically care about:

- Install start and completion
- Download progress
- Import result statistics
- Configuration changes
- Service start and stop

#### warn

Warning information, indicating an abnormality but not affecting continued operation:

- Filename cannot be recognized (but continues processing other files)
- Network retry
- Cache expiration
- Configuration item uses default value

#### error

Error information, indicating an operation failed:

- Download failure
- Install failure
- File corruption
- Invalid configuration
- Insufficient permissions

#### fatal

Fatal errors, the program cannot continue running:

- Data directory cannot be created
- Configuration file corrupted and unrecoverable
- Serious internal errors

## Log File Location

### Portable Mode

```
bws-data/
└── logs/
    └── bws.log
```

### Default Location

Log files are located in the `logs/` subdirectory of the data directory:

```
bws-data/logs/bws.log
```

If a custom data directory is set via `bws cfg set data-dir`, logs will be located at `$data-dir/logs/bws.log`.

### Log File Format

Each log line contains the following information:

```
Timestamp  Level  Source location  Message content
```

Example:

```
2024-01-15T10:30:00.123Z  INFO  install/install.go:45  Starting installation of chrome 120.0.6099.109
2024-01-15T10:30:00.456Z  DEBUG download/download.go:78  Download URL: https://dl.google.com/chrome/...
2024-01-15T10:30:05.789Z  INFO  download/download.go:120  Download progress: 50% (50MB/100MB)
2024-01-15T10:30:10.012Z  INFO  install/install.go:120  Installation complete
```

### Log Rotation

Log files are automatically rotated to prevent a single file from becoming too large:

- A new file is automatically created when a single log file reaches a certain size
- A certain number of historical log files are retained
- Old log files are automatically cleaned up

## Changing Console Log Level

Use the `bws cfg` command to modify the console log level.

### Setting Log Level

```bash
bws cfg set log-level debug
```

### Viewing Current Level

```bash
bws cfg get log-level
```

### Level Selection Recommendations

| Scenario | Recommended Level | Description |
|----------|-------------------|-------------|
| Daily use | `info` | Default level, outputs key information |
| Troubleshooting | `debug` | View detailed operation process |
| In-depth debugging | `trace` | For developers |
| Only errors | `error` | Only focus on error information |
| Silent mode | `fatal` | Almost no output |

### Examples

#### Viewing Debug Information

When encountering issues, adjusting the log level to debug can provide more information:

```bash
# Set to debug level
bws cfg set log-level debug

# Re-execute the problematic command
bws i chrome@120

# View more output information to help locate the problem
```

#### Silent Mode

When used in scripts, you may want to reduce output:

```bash
# Set to error level, only output errors
bws cfg set log-level error

# Or set to fatal, almost no output
bws cfg set log-level fatal
```

## File Log Level

File logs use `DEBUG` level by default, always recording detailed information, unaffected by console log level configuration.

### Why File Log is Fixed at DEBUG

- Facilitates troubleshooting: detailed logs are always available when problems occur
- Small space usage: text logs have a high compression ratio
- Automatic rotation: no need to worry about files growing infinitely

### Viewing File Logs

```bash
# View latest logs
tail -f bws-data/logs/bws.log

# View error logs
grep ERROR bws-data/logs/bws.log

# View logs for a specific time period
grep "2024-01-15T10:" bws-data/logs/bws.log
```

### Viewing Logs on Windows

```powershell
# View log file
Get-Content bws-data\logs\bws.log -Tail 50

# View in real time
Get-Content bws-data\logs\bws.log -Wait -Tail 50
```

## Logging and Troubleshooting

When encountering issues, logs are an important basis for troubleshooting.

### General Troubleshooting Steps

1. **Reproduce the issue**: Ensure it can be stably reproduced
2. **Increase log level**: Set console to debug level
3. **View output**: Observe error information in console output
4. **View file logs**: Search for errors and warnings in log files
5. **Locate the problem**: Determine the cause based on source locations and error information in the logs

### Common Log Information Descriptions

| Keyword | Description |
|---------|-------------|
| `Starting download` | Download operation started |
| `Download progress` | Download progress update |
| `Download complete` | Download successfully completed |
| `Starting installation` | Install operation started |
| `Installation complete` | Install successfully completed |
| `Installation failed` | Install error, check error details |
| `Cannot recognize` | Filename cannot be automatically recognized |
| `Verification failed` | File checksum mismatch, may be corrupted |
| `Network error` | Network connection issue |
| `Insufficient permissions` | File read/write permission issue |

### Submitting Bug Reports

If you need to submit a bug report, it is recommended to include the following information:

1. bws version (`bws version`)
2. Operating system and version
3. Reproduction steps
4. Error information (console output)
5. Relevant log snippets (file logs)

## Notes

1. **Log level only affects console**: The `log-level` configuration only affects console output; file logs are always at DEBUG level
2. **Logs are automatically rotated**: No need to worry about log files growing infinitely; the system will automatically rotate and clean up
3. **Logs may contain sensitive information**: Logs may contain file paths and other information; pay attention to sanitization before sharing logs
4. **Performance impact**: Increasing the log level (e.g. trace) may slightly affect performance; it is recommended to change back to info after debugging
5. **Log directory location**: Can be found via `bws cc path` or by checking the data directory to locate the logs folder
