# Serve Auto Sync

`bws sv` supports automatically syncing browser installer packages from online sources to the local `packages/` directory, enabling automatic updates for the offline source.

## Feature Description

Auto sync is an optional feature of the serve service. Once enabled, it periodically downloads the latest browser installer packages from online sources to local storage, keeping the offline source always up to date.

### Main Features

- **Scheduled sync**: Automatically executes sync at specified intervals
- **Incremental download**: Existing files are skipped; only new files are downloaded
- **Multi-browser support**: Can be configured to sync multiple browsers
- **Multi-channel support**: Can be configured to sync multiple release channels
- **Manual trigger**: Supports manually triggering sync via the web page or API
- **Status query**: Can query current sync status and history
- **Auto refresh**: Automatically refreshes the file manifest after sync completes

### Sync Flow

1. Fetch the list of available versions from the online source
2. Compare with the local `packages/` directory to find missing versions
3. Download missing installer packages one by one
4. Save downloaded packages to the `packages/` directory
5. Automatically refresh the file manifest and checksum cache

## Enabling Sync

Edit the `bws-serve.ini` configuration file to enable the auto sync feature.

### Basic Configuration

```ini
[serve]
sync = true
sync-interval = 24h
sync-channels = stable
```

Then run `bws sv` to start the service.

Default configuration:
- Sync interval: 24 hours (once per day)
- Sync browsers: all supported browsers
- Sync channels: stable

### Sync on Startup

After enabling sync, the service will immediately execute a sync upon startup, and then perform scheduled syncs at the specified interval.

## Sync Interval Configuration

Use the `sync-interval` configuration item to customize the sync interval.

### Time Format

The following time units are supported:

| Unit | Abbreviation | Examples |
|------|--------------|----------|
| Days | `d` | `7d`, `30d` |
| Hours | `h` | `6h`, `12h`, `24h` |
| Minutes | `m` | `30m`, `60m` |
| Seconds | `s` | `3600s` |

Combinations are also supported: `1h30m`, `2h45m`, etc.

### Common Configurations

```ini
# Sync every 30 days
sync-interval = 30d

# Sync every 6 hours
sync-interval = 6h

# Sync every 12 hours
sync-interval = 12h

# Sync once per day (default)
sync-interval = 24h
```

### Interval Selection Recommendations

- **Development environment**: `6h` or `12h` - keep relatively recent versions
- **Production environment**: `24h` - update once per day is sufficient
- **Test environment**: `1h` - use when frequently fetching the latest versions

> **Note**: The sync interval should not be too short, otherwise it will put pressure on the online source. The recommended minimum interval is no less than 1 hour.

## Browsers and Channels to Sync

By default, stable channel versions of all supported browsers are synced. This can be customized via configuration items.

### Specifying Browsers to Sync

Use the `sync-browsers` configuration item to specify the list of browsers to sync, separated by commas:

```ini
# Only sync Chrome
sync-browsers = chrome

# Sync Chrome and Firefox
sync-browsers = chrome,firefox

# Sync Chrome, Firefox, and Chromium
sync-browsers = chrome,firefox,chromium
```

Supported browser names (downloadable from built-in sources):
- `chrome` - Google Chrome
- `firefox` - Mozilla Firefox
- `chromium` - Chromium

### Specifying Channels to Sync

Use the `sync-channels` configuration item to specify the list of channels to sync, separated by commas:

```ini
# Only sync stable channel (default)
sync-channels = stable

# Sync stable and beta channels
sync-channels = stable,beta

# Sync all channels
sync-channels = stable,beta,dev,canary
```

Supported channels:
- `stable` - Stable release
- `beta` - Beta test release
- `dev` - Dev development release
- `canary` - Canary release
- `esr` - Firefox Extended Support Release

### Combined Configuration Examples

```ini
# Sync Chrome stable and beta channels, every 12 hours
[serve]
sync = true
sync-interval = 12h
sync-browsers = chrome
sync-channels = stable,beta
```

```ini
# Sync Chrome and Firefox stable channels, once per day
[serve]
sync = true
sync-browsers = chrome,firefox
sync-channels = stable
```

After configuration is complete, run `bws sv` to start the service.

## Manual Trigger via Web Page

In addition to scheduled automatic sync, you can also manually trigger sync via the web page.

### Operation Steps

1. Open the serve service address in a browser (e.g. `http://server:8080`)
2. Find the "Sync" or "Sync Now" button
3. Click the button to trigger sync
4. The page will display sync progress and results

### Use Cases

- A new version has just been released and you don't want to wait for the next scheduled sync
- Testing whether the sync function is working properly
- Temporarily needing a new version

Manually triggered sync follows the exact same flow as scheduled sync and will not affect the scheduled sync timing.

## Sync Status API

You can query sync status via the API.

### Query Sync Status

**Request:**

```
GET /api/v1/sync/status
```

**Response Example:**

```json
{
  "running": false,
  "lastSync": "2024-01-15T10:30:00Z",
  "nextSync": "2024-01-16T10:30:00Z",
  "lastResult": {
    "success": 5,
    "failed": 0,
    "skipped": 10,
    "total": 15
  }
}
```

### Field Descriptions

| Field | Type | Description |
|-------|------|-------------|
| `running` | boolean | Whether sync is currently in progress |
| `lastSync` | string | Last sync time (ISO 8601 format) |
| `nextSync` | string | Next scheduled sync time (ISO 8601 format) |
| `lastResult.success` | number | Number of files successfully downloaded in last sync |
| `lastResult.failed` | number | Number of files that failed in last sync |
| `lastResult.skipped` | number | Number of files skipped in last sync (already exist) |
| `lastResult.total` | number | Total number of files processed in last sync |

### Manual Trigger Sync API

**Request:**

```
POST /api/v1/sync/trigger
```

**Response Example:**

```json
{
  "ok": true,
  "message": "Sync triggered"
}
```

If sync is already in progress, the request will be silently ignored (no-op) and still return a success response.

If sync is not enabled (no sync source configured), `/api/v1/sync/status` still returns 200, with the `progress` field in `data` indicating that sync is not enabled; `/api/v1/sync/trigger` returns 503.

For more API details, please refer to the [Serve API Reference](./serve-api.md) chapter.

## Notes

1. **Disk space**: Synced files may occupy a large amount of disk space, especially when syncing multiple browsers and channels
2. **Network bandwidth**: Sync will download a large number of files; be aware of network bandwidth usage
3. **Sync conflict**: When manually triggering sync, if a scheduled sync is already in progress, it will be silently ignored
4. **File verification**: File integrity is automatically verified after download; failed files will be re-downloaded
5. **Incremental sync**: Existing files are skipped and will not be downloaded again
6. **Service restart**: After a service restart, the sync schedule is recalculated, which may cause sync time shifts
