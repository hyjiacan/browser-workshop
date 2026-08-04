# Data Storage

bws stores all runtime data (versions, cache, logs, Profiles, etc.) uniformly in the data directory. Configuration files (`bws-client.ini` and `bws-serve.ini`) are located in the same directory as the bws executable. This chapter introduces the directory structure and the purpose of each directory.

## Portable Mode (Default)

By default, bws uses portable mode, storing data in the `bws-data/` directory at the same level as the program.

### Directory Structure

```
bws/
├── bws.exe                    # Main program file
├── bws-client.ini             # Client configuration file (INI format)
├── bws-serve.ini              # Serve service configuration file (created on first run of serve)
└── bws-data/                  # Data root directory
    ├── .serve-cache.json      # serve checksum cache
    ├── logs/                  # Log directory
    │   └── bws.log            # bws log file (shared with client)
    ├── cache/                 # Download cache
    │   ├── manifests/         # Version manifest cache
    │   │   └── firefox-ftp-cache.json  # Firefox FTP source cache
    │   └── downloads/         # Downloaded file cache
    ├── versions/              # Installed browser versions
    │   ├── chrome/
    │   │   ├── 126.0.6478.114/
    │   │   ├── 121.0.6167.85/
    │   │   └── 79.0.3945.79/
    │   ├── firefox/
    │   │   └── 121.0/
    │   └── ...
    └── runtime/               # Runtime data
        └── chrome/
            ├── 126.0.6478.114/
            │   └── profile/   # Version default Profile
            ├── 121.0.6167.85/
            │   └── profile/   # Version default Profile
            └── profiles/
                ├── work/      # Named Profile "work"
                └── test/      # Named Profile "test"
```

### Advantages of Portable Mode

- **Plug and play**: The entire directory can be copied to another machine and used immediately
- **Centralized data**: All data is under one directory, easy to manage and back up
- **No system pollution**: Does not write any data to system directories
- **USB drive friendly**: Can be placed on a USB drive and carried around

## Directory Descriptions

### bws-client.ini

Configuration file, storing all user configuration items. INI format.

```ini
[client]
default-browser = chrome
default-channel = stable
language = zh

[log]
console-level = info
file-level = debug
max-size-mb = 10
max-backups = 5

[source-switches]
enable-serve-source = true
enable-firefox-ftp = true
```

Usually no manual editing is needed; use the `bws cfg` command to manage.

If you need to store data in another location, you can set a custom data directory via the configuration command:

```bash
bws cfg set data-dir D:\browser-data
```

### bws-serve.ini

Serve service configuration file, automatically created in the same directory as the bws executable on first run of `bws sv`.

```ini
[serve]
host = 0.0.0.0
port = 8080
packages-dir =
bin-dir =
sync = false
sync-interval = 24h
sync-browsers =
sync-channels = stable
online-fallback = false
scan-workers = 0
```

### logs/

Log directory, storing bws runtime logs.

- `bws.log`: Main log file, records all operations
- File logs are at DEBUG level by default, recording all operations in detail
- Logs are automatically rotated to prevent a single file from becoming too large

For more log-related information, please refer to the [Logging System](./logging.md) chapter.

### cache/

Cache directory, storing downloaded installer packages and manifest caches.

#### cache/manifests/

Version manifest cache, storing version lists fetched from remote sources to avoid re-requesting every time.

- Speeds up response for commands like `ls --remote`
- Has an expiration time; automatically re-fetches after expiry
- Firefox FTP source cache stored as `firefox-ftp-cache.json`, default 24-hour validity
- Can be force-refreshed via `bws ls -R --refresh`
- Can be cleaned via `bws cc clear`

#### cache/downloads/

Download file cache, permanently storing installer packages downloaded from serve via the `install` command.

- Files are retained in the cache after installation; reused for subsequent installs of the same version
- Automatically re-downloaded when the file on the serve side is updated (size changes)
- Use `--refresh-cache` flag to force re-download from serve (ignoring local cache)
- May occupy a large amount of space; can be cleaned periodically via `bws cc clear`
- View cache file list and space usage via `bws cc info`

### versions/

Installed browser versions directory, stored hierarchically by browser name and version number.

```
versions/
├── chrome/
│   ├── 126.0.6478.114/     # Chrome 126 version files
│   ├── 121.0.6167.85/      # Chrome 121 version files
│   └── 79.0.3945.79/       # Chrome 79 version files
├── firefox/
│   └── 121.0/              # Firefox 121 version files
└── chromium/
    └── ...
```

Each version directory contains complete browser program files. Uninstalling will delete the corresponding version directory.

### runtime/

Runtime data directory, storing data generated during browser runtime, mainly Profiles.

#### Version Default Profile

Each version has an independent default Profile:

```
runtime/chrome/126.0.6478.114/profile/
runtime/chrome/121.0.6167.85/profile/
```

- Used when running the browser without specifying a Profile
- Each version's Profile is completely independent
- Not automatically deleted when uninstalling a version; manual cleanup is required

#### Named Profile

Named Profiles created by the user:

```
runtime/chrome/profiles/work/
runtime/chrome/profiles/test/
runtime/chrome/profiles/personal/
```

- The same named Profile can be shared across different versions
- Isolated by browser type
- Persistent storage, unaffected by version uninstallation

## Data Migration

### Moving the Data Directory

If you need to move data to another location:

1. Stop all running browser instances
2. Copy or move the entire `bws-data/` directory to the new location
3. Verify data integrity (`bws ls` to check if versions are normal)

## Disk Space Management

### Checking Occupied Space

```bash
# Check cache status
bws cc info

# Check total data occupied space (manual calculation required)
du -sh bws-data/
```

### Reclaiming Space

```bash
# Clean download cache
bws cc clear

# Uninstall unneeded versions
bws rm chrome@79

# Clean orphaned Profiles
bws pf clean
```

### Estimated Space Usage by Part

| Directory | Space Usage | Description |
|-----------|-------------|-------------|
| `versions/` | Largest | Each browser version is approximately 200-500MB |
| `runtime/` | Medium | Each Profile is approximately tens to hundreds of MB |
| `cache/downloads/` | Medium | Each installer package is approximately 50-100MB |
| `logs/` | Very small | Usually tens of MB |
| `bws-client.ini` | Extremely small | A few KB |

## Notes

1. **Backup recommendation**: Regularly back up `bws-client.ini` and important Profile data
2. **Manual editing**: It is not recommended to manually edit `bws-client.ini`; use the `bws cfg` command
3. **Deletion safety**: Uninstalling a version does not delete the Profile, preventing accidental loss of important data
4. **Permissions**: Ensure bws has read/write permissions for the data directory
5. **Antivirus**: Some antivirus software may falsely flag browser files; it is recommended to add the `versions/` directory to the whitelist
6. **Disk format**: The `versions/` directory contains many files; it is recommended to use a file system that supports a large number of files, such as NTFS
