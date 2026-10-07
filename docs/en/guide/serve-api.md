# Serve API Reference

`bws sv` provides complete REST API interfaces for querying file manifests, downloading files, managing sync, etc. This document details the usage of all API endpoints.

## API Endpoint Overview

| Path | Method | Description |
|------|--------|-------------|
| `/` | GET | HTML help page / Web interface |
| `/api/v1/manifest` | GET | File manifest (local + online cache merged, with XXH3 checksum) |
| `/api/v1/download/{filename}` | GET | File download (supports resume) |
| `/api/v1/status` | GET | Service status |
| `/api/v1/sync/status` | GET | Sync status |
| `/api/v1/sync/trigger` | POST | Manually trigger sync |
| `/api/v1/bin` | GET | Client binary file listing (JSON) |
| `/api/v1/bin/{filename}` | GET | Client binary download |
| `/api/v1/driver/manifest` | GET | Resolve an automation driver (chromedriver) build for a Chrome version |
| `/api/v1/driver/download/{filename}` | GET | Driver archive download (hosted locally first; otherwise proxied from upstream and cached) |

> **Manifest merge mechanism**: When `online-fallback` is enabled, `manifest` returns a merged result of local packages directory files and online source cached versions (deduplicated). Local files carry real XXH3 checksums; online cached versions have empty checksums (computed after download). Filtering is done client-side.

## Basic Information

### Base URL

The base URL for all APIs is the serve service address, for example:

```
http://localhost:8080
http://192.168.1.100:8080
```

### Data Format

- Response format: JSON (unless otherwise specified)
- Character encoding: UTF-8
- Time format: ISO 8601 (e.g. `2024-01-15T10:30:00Z`)

### Error Handling

When an API error occurs, a standard HTTP status code is returned, and the response body is a **plain text** error description (not JSON).

For example:

```
file not found
```

```
method not allowed
```

Common HTTP status codes:

| Status Code | Description |
|-------------|-------------|
| 200 OK | Request successful |
| 400 Bad Request | Request parameter error |
| 404 Not Found | Resource does not exist |
| 405 Method Not Allowed | Method not allowed |
| 500 Internal Server Error | Internal server error |
| 503 Service Unavailable | Service unavailable (e.g. sync feature not enabled) |

---

## GET /

HTML help page, i.e. the web management interface.

### Request

```
GET /
```

### Response

Returns an HTML page containing:

- Available browser version list
- File download links
- Service status information
- Sync control (if sync is enabled)
- Client binary download (if bin directory exists)

---

## GET /api/v1/manifest

Get the file manifest, containing local packages directory files and online source cached versions (when `online-fallback` is enabled). Returns entries for all platforms, architectures, versions, and channels; filtering is done client-side.

### Query Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| `refresh` | string | Set to `true` to force the serve instance to refresh its online cache before returning results |

### Request

```
GET /api/v1/manifest
GET /api/v1/manifest?refresh=true
```

### Response Example

```json
{
  "status": "ok",
  "data": [
    {
      "filename": "Chrome_120.0.6099.109_Windows_x64.exe",
      "version": "120.0.6099.109",
      "browser": "chrome",
      "channel": "stable",
      "major_version": "120",
      "platform": "windows",
      "architecture": "x64",
      "size": 104857600,
      "checksum": "xxh3:abcdef1234567890"
    },
    {
      "filename": "Firefox Setup 141.0.exe",
      "version": "141.0",
      "browser": "firefox",
      "channel": "stable",
      "major_version": "141",
      "platform": "windows",
      "architecture": "amd64",
      "size": 57671680,
      "checksum": ""
    }
  ],
  "server": {
    "name": "bws-serve",
    "version": "2026.10.07",
    "file_count": 2
  }
}
```

> Local files have real XXH3 checksums; online cached versions have empty checksum strings, computed only after download.

### Response Field Descriptions

| Field | Type | Description |
|-------|------|-------------|
| `status` | string | Always `"ok"` |
| `data` | array | File list |
| `data[].filename` | string | Filename (relative path) |
| `data[].version` | string | Version number |
| `data[].browser` | string | Browser name (chrome / firefox / chromium) |
| `data[].channel` | string | Release channel (stable / beta / dev / canary / esr) |
| `data[].major_version` | string | Major version number |
| `data[].platform` | string | Platform (windows / linux / macos) |
| `data[].architecture` | string | Architecture (x64 / x86 / arm64) |
| `data[].size` | number | File size (bytes) |
| `data[].checksum` | string | XXH3 checksum, format is `xxh3:` + 16-digit hex; empty for online cached versions |
| `server` | object | Server information |
| `server.name` | string | Service name |
| `server.version` | string | Server version |
| `server.file_count` | number | Total number of files |

### Usage Example

```bash
# Get full manifest
curl http://localhost:8080/api/v1/manifest
```

---

## GET /api/v1/download/{filename}

Download a specified file, supporting resume.

### Request

```
GET /api/v1/download/{filename}
```

### Path Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| filename | string | Name of the file to download |

### Response Headers

| Header | Description |
|--------|-------------|
| Content-Type | application/octet-stream |
| Content-Length | File size (bytes) |
| Content-Disposition | Attachment download |
| Accept-Ranges | bytes (supports resume) |
| ETag | File checksum |

### Resume Support

Supports HTTP Range requests, allowing download to continue from a specified position:

```bash
# Start downloading from the 1,000,000th byte
curl -H "Range: bytes=1000000-" http://localhost:8080/api/v1/download/file.exe
```

### Error Responses

| Status Code | Description |
|-------------|-------------|
| 404 | File does not exist |

### Usage Example

```bash
# Download file
curl -O http://localhost:8080/api/v1/download/Chrome_120.0.6099.109_Windows_x64.exe

# Download using wget (supports resume)
wget -c http://localhost:8080/api/v1/download/Chrome_120.0.6099.109_Windows_x64.exe
```

---

## GET /api/v1/status

Get service status information.

### Request

```
GET /api/v1/status
```

### Response Example

```json
{
  "status": "ok",
  "server": {
    "name": "bws-serve",
    "version": "2026.10.07",
    "uptime": 88215,
    "file_count": 15,
    "total_size": 1610612736
  }
}
```

### Response Field Descriptions

| Field | Type | Description |
|-------|------|-------------|
| `status` | string | Always `"ok"` |
| `server` | object | Server information |
| `server.name` | string | Service name |
| `server.version` | string | Server version |
| `server.uptime` | number | Service uptime in seconds |
| `server.file_count` | number | Total number of files |
| `server.total_size` | number | Total file size (bytes) |

### Usage Example

```bash
curl http://localhost:8080/api/v1/status
```

---

## GET /api/v1/sync/status

Get sync status. When no sync source is configured, it still returns 200, with a prompt message in `data.progress`.

### Request

```
GET /api/v1/sync/status
```

### Response Example (Sync Enabled)

```json
{
  "status": "ok",
  "data": {
    "running": false,
    "last_sync": "2024-01-15T10:30:00Z",
    "next_sync": "2024-01-16T10:30:00Z",
    "last_error": "",
    "progress": "Sync complete",
    "total_files": 15,
    "synced_files": 15
  }
}
```

### Response Example (Sync Not Enabled)

```json
{
  "status": "ok",
  "data": {
    "running": false,
    "last_sync": "0001-01-01T00:00:00Z",
    "next_sync": "0001-01-01T00:00:00Z",
    "last_error": "",
    "progress": "Sync not enabled (no sync source configured)",
    "total_files": 0,
    "synced_files": 0
  }
}
```

### Response Field Descriptions

| Field | Type | Description |
|-------|------|-------------|
| `status` | string | Always `"ok"` |
| `data` | object | Sync status information |
| `data.running` | boolean | Whether sync is currently in progress |
| `data.last_sync` | string | Last sync completion time (ISO 8601, zero value means never synced) |
| `data.next_sync` | string | Next scheduled sync time (ISO 8601, zero value means no schedule) |
| `data.last_error` | string | Last sync error message (empty means no error; may be omitted) |
| `data.progress` | string | Current sync progress description (may be omitted) |
| `data.total_files` | number | Total number of files in sync task |
| `data.synced_files` | number | Number of files already synced |

---

## POST /api/v1/sync/trigger

Manually trigger sync. If sync is already running, it is silently ignored (no-op) and still returns 200.

### Request

```
POST /api/v1/sync/trigger
```

### Success Response

```json
{
  "status": "ok",
  "message": "Sync triggered"
}
```

### Error Response

When the sync feature is not enabled, returns 503 status code with a plain text response body:

```
sync not enabled
```

### Usage Example

```bash
curl -X POST http://localhost:8080/api/v1/sync/trigger
```

---

## GET /api/v1/bin

Get the client binary file directory listing, used for auto-update scenarios.

### Request

```
GET /api/v1/bin
```

### Response

```json
{
  "status": "ok",
  "data": [
    {
      "filename": "bws_windows_amd64.zip",
      "platform": "windows",
      "arch": "amd64",
      "version": "2026.10.07",
      "size": 5242880
    }
  ]
}
```

### Response Fields

| Field | Type | Description |
|-------|------|-------------|
| status | string | Always `ok` |
| data | array | Binary file list |
| data[].filename | string | File name |
| data[].platform | string | Target platform (windows/darwin/linux) |
| data[].arch | string | Target architecture (amd64/arm64) |
| data[].version | string | bws version |
| data[].size | int64 | File size in bytes |

### Usage Example

```bash
# List all available binary files
curl http://localhost:8080/api/v1/bin
```

---

## GET /api/v1/bin/{filename}

Download client binary files (only available when bin directory exists).

### Request

```
GET /api/v1/bin/{filename}
```

### Path Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| filename | string | Binary filename |

### Response

Returns file content, supports resume, behavior is consistent with the download interface.

### Usage Example

```bash
# Download Windows version of bws
curl -O http://localhost:8080/api/v1/bin/bws-windows-amd64.exe
```

---

## GET /api/v1/driver/manifest

Resolve the automation driver (chromedriver) build matching a Chrome version, so clients can obtain drivers in offline/intranet environments.

The server queries the upstream Chrome for Testing manifest (Chrome >= 115) or the legacy chromedriver storage (Chrome < 115); when upstream is unreachable it falls back to the locally cached driver index. The resolved build is recorded in the index, and the download URL is rewritten to a relative path on this instance so clients fetch the archive through this service.

### Query Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `chrome` | string | Yes | Chrome version to match (e.g. `120` or `120.0.6099.109`) |
| `platform` | string | No | Platform (`windows`/`darwin`/`linux`), defaults to the server platform |
| `arch` | string | No | Architecture (`amd64`/`386`/`arm64`), defaults to the server arch |

### Request

```
GET /api/v1/driver/manifest?chrome=120
GET /api/v1/driver/manifest?chrome=120.0.6099.109&platform=windows&arch=amd64
```

### Response Example

```json
{
  "status": "ok",
  "data": {
    "name": "chromedriver",
    "version": "120.0.6099.109",
    "major_version": "120",
    "platform": "windows",
    "arch": "amd64",
    "download_url": "/api/v1/driver/download/chromedriver-win64.zip",
    "filename": "chromedriver-win64.zip",
    "source": "serve"
  }
}
```

### Response Fields

| Field | Type | Description |
|-------|------|-------------|
| `status` | string | `"ok"` or `"error"` |
| `data.name` | string | Driver name (always `chromedriver`) |
| `data.version` | string | Exact driver version |
| `data.major_version` | string | Major version (install directory key) |
| `data.platform` | string | Platform (windows / darwin / linux) |
| `data.arch` | string | Architecture (amd64 / 386 / arm64) |
| `data.download_url` | string | Archive download URL (relative to this instance; fetched via proxy) |
| `data.filename` | string | Archive filename |
| `data.source` | string | Resolution source (`serve` / `serve-cache`) |
| `error` | string | Error description when `status` is `error` |

### Error Responses

Returns `503` when the driver service is disabled, `400` when the `chrome` parameter is missing, and `404` when resolution fails and the local index has no match. The body is JSON:

```json
{
  "status": "error",
  "error": "Chrome for Testing manifest has no chromedriver for Chrome 999"
}
```

> **Note**: unlike other endpoints (plain-text errors), `driver/manifest` returns JSON errors so clients can parse the failure reason.

### Usage Example

```bash
# Resolve the driver for Chrome 120
curl "http://localhost:8080/api/v1/driver/manifest?chrome=120"
```

---

## GET /api/v1/driver/download/{filename}

Download a driver archive. If the archive is already hosted locally it is served directly; otherwise it is downloaded from the upstream URL recorded in the driver index, cached, and then served.

### Request

```
GET /api/v1/driver/download/{filename}
```

### Path Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| filename | string | Driver archive filename (e.g. `chromedriver-win64.zip`) |

### Response

Returns the file content, supports resume, and behaves like `/api/v1/download/{filename}`.

### Error Responses

| Status | Description |
|--------|-------------|
| 400 | Filename is empty or contains invalid characters |
| 404 | Not hosted locally and no matching entry in the index |
| 502 | Failed to proxy the download from upstream |

### Usage Example

```bash
# Download a driver archive
curl -O http://localhost:8080/api/v1/driver/download/chromedriver-win64.zip
```

---

## Error Handling Examples

### File Not Found

```
file not found
```

HTTP status code: `404 Not Found`

### Sync Not Enabled

```
sync not enabled
```

HTTP status code: `503 Service Unavailable`

### Method Not Allowed

```
method not allowed
```

HTTP status code: `405 Method Not Allowed`
