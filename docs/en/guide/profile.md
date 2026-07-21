# Profile Management

Each browser version has an independent user data directory (Profile) for storing bookmarks, history, cookies, extensions, and other data. bws provides comprehensive Profile management features, including default Profiles, named Profiles, reset, and cleanup.

## Default Profile

### How It Works

Each browser version uses an independent Profile directory by default, and data does not interfere with each other. This is one of the core features of bws, ensuring that configuration conflicts do not occur between different browser versions.

Default Profile path:

```
bws-data/runtime/{browser}/{version}/profile/
```

For example, the default Profile path for Chrome 120.0.6099.109 is:

```
bws-data/runtime/chrome/120.0.6099.109/profile/
```

### Characteristics

- One independent Profile per version
- Data is completely isolated between versions
- Automatically created without manual specification
- Not automatically deleted when uninstalling the browser (to prevent accidental deletion)

### Running the Default Profile

If no Profile is specified when running the browser, the default Profile for that version is used:

```bash
bws r chrome@120
```

## Named Profile

Named Profiles are user-defined Profiles that can be shared between different versions of the same browser.

### How It Works

Named Profile path:

```
bws-data/runtime/{browser}/profiles/{name}/
```

For example, the path for a Chrome named Profile called `work` is:

```
bws-data/runtime/chrome/profiles/work/
```

### Using Named Profiles

Use the `-p` or `--profile` parameter to specify a named Profile:

```bash
bws r chrome@120 -p work
bws r chrome@121 -p work
```

### Characteristics of Named Profiles

- The same named Profile can be shared across different versions
- Suitable for distinguishing different scenarios such as work, personal, and testing
- Data is persistent and not affected by version uninstallation
- Isolated by browser type (Chrome and Firefox profiles with the same name do not affect each other)

### Usage Scenario Examples

```bash
# Work Profile - saves work-related bookmarks and login states
bws r chrome@120 -p work

# Personal Profile - saves personal browsing data
bws r chrome@120 -p personal

# Test Profile - clean test environment
bws r chrome@120 -p test

# Development Profile - installs development extensions
bws r chrome@120 -p dev
```

## Listing Profiles

Use the `bws pf list` command to list all Profiles.

### List All Profiles

```bash
bws pf list
```

### List Profiles for a Specific Browser

```bash
bws pf list chrome
bws pf list gc
```

### Output Content

Listed information includes:

- Default Profiles: default Profiles for each installed version
- Named Profiles: all named Profiles created by the user
- Orphaned Profiles: Profiles left over from uninstalled versions

## Viewing Profile Paths

Use the `bws pf path` command to view the actual path of a Profile.

### View Default Profile Path

```bash
bws pf path chrome@120
```

### View Named Profile Path

```bash
bws pf path chrome myprofile
```

### Usage Scenarios

- Manually back up Profile data
- View specific files in a Profile
- Debug browser issues
- Manually clean up certain data

## Resetting a Profile

Use the `bws pf reset` command to reset a Profile, clearing all data and restoring the initial state.

### Reset Default Profile

```bash
bws pf reset chrome@120
```

### Reset Named Profile

```bash
bws pf reset chrome@120 myprofile
```

### Skip Confirmation

By default, a confirmation prompt is displayed before resetting. Use the `-f` parameter to skip confirmation:

```bash
bws pf reset chrome@120 -f
bws pf reset chrome@120 myprofile -f
```

### Reset Effects

Resetting a Profile will:

1. Delete all files in the Profile directory
2. Recreate an empty Profile directory
3. The browser will generate a fresh configuration on the next startup

### Usage Scenarios

- The browser is experiencing abnormalities and needs to be restored to the initial state
- Testing requires a clean environment
- Clear all browsing data
- Troubleshoot extension conflicts

> **Note**: The reset operation is irreversible; all data in the Profile (bookmarks, passwords, extensions, etc.) will be permanently deleted. Please proceed with caution.

## Cleaning Up Orphaned Profiles

After uninstalling a browser version, the corresponding Profile data will not be automatically deleted and will become an "orphaned Profile". Use the `bws pf clean` command to clean up these residual Profile data.

### Clean Up All Orphaned Profiles

```bash
bws pf clean
```

### Clean Up Orphaned Profiles for a Specific Browser

```bash
bws pf clean chrome
bws pf clean gc
```

### Skip Confirmation

Use the `-f` parameter to skip the confirmation prompt:

```bash
bws pf clean -f
bws pf clean chrome -f
```

### What Is an Orphaned Profile

An orphaned Profile refers to:

- Default Profiles of uninstalled versions
- Named Profiles not used by any version (usually does not occur)

### Why Clean Up Is Needed

- Free up disk space
- Keep the data directory tidy
- Delete old data that is no longer needed

### Cleanup Process

1. Scan all Profile directories
2. Check whether the corresponding version is still installed
3. List all unassociated orphaned Profiles
4. Delete after confirmation

## Profile Command Summary

| Command | Description |
|------|------|
| `bws pf list` | List all Profiles |
| `bws pf list <browser>` | List Profiles for a specified browser |
| `bws pf path <browser@version>` | View default Profile path |
| `bws pf path <browser> <name>` | View named Profile path |
| `bws pf reset <browser@version>` | Reset default Profile |
| `bws pf reset <browser@version> <name>` | Reset named Profile |
| `bws pf reset ... -f` | Reset directly without confirmation |
| `bws pf clean` | Clean up all orphaned Profiles |
| `bws pf clean <browser>` | Clean up orphaned Profiles for a specified browser |
| `bws pf clean ... -f` | Clean up directly without confirmation |

## Profile Directory Structure

```
bws-data/runtime/
└── chrome/
    ├── 120.0.6099.109/
    │   └── profile/           # Default Profile for version 120
    ├── 121.0.6167.85/
    │   └── profile/           # Default Profile for version 121
    └── profiles/
        ├── work/              # Named Profile "work"
        ├── test/              # Named Profile "test"
        └── personal/          # Named Profile "personal"
```

## Notes

1. **Data Security**: Reset and cleanup operations are irreversible; please confirm that important data has been backed up before operating
2. **Version Isolation**: Default Profiles for different versions are completely independent and do not affect each other
3. **Named Sharing**: Named Profiles can be shared between different versions of the same browser
4. **Cross-Browser Isolation**: Profiles for different browsers are completely isolated, even if they have the same name they are not shared
5. **Uninstallation Retention**: When uninstalling a browser version, Profile data is retained and needs to be manually cleaned up
6. **Disk Space**: Profile data may occupy a large amount of space (especially the cache); regular cleanup can free up space
