# Serve API Reference

`bws sv` provides complete REST API interfaces for querying file manifests, downloading files, managing sync, etc. This document details the usage of all API endpoints.

## API Endpoint Overview

| Path | Method | Description |
|------|--------|-------------|
| `/` | GET | HTML help page / Web interface |
| `/api/v1/manifest` | GET | File manifest (with XXH3 checksum) |
| `/api/v1/download/{filename}` | GET | File download (supports resume) |
| `/api/v1/status` | GET | Service status |
| `/api/v1/sync/status` | GET | Sync status |
| `/api/v1/sync/trigger` | POST | Manually trigger sync |
| `/bin/{filename}` | GET | Client binary download |

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

Get the file manifest, containing information about all recognizable browser installer packages and their XXH3 checksums.

### Request

```
GET /api/v1/manifest
```

### Response Example

```json
{
  "status": "ok",
  "data": [
    {
      "filename": "Chrome_120.0.6099.109_Windows_x64.exe",
      "version": "120.0.6099.109",
      "major_version": "120",
      "platform": "windows",
      "architecture": "x64",
      "size": 104857600,
      "checksum": "xxh3:abcdef1234567890"
    },
    {
      "filename": "Firefox_121.0_Linux_x64.tar.bz2",
      "version": "121.0",
      "major_version": "121",
      "platform": "linux",
      "architecture": "x64",
      "size": 67108864,
      "checksum": "xxh3:0987654321fedcba"
    }
  ],
  "server": {
    "name": "Browser Workshop",
    "version": "1.0.0",
    "file_count": 2
  }
}
```

### Response Field Descriptions

| Field | Type | Description |
|-------|------|-------------|
| `status` | string | Always `"ok"` |
| `data` | array | File list |
| `data[].filename` | string | Filename (relative path) |
| `data[].version` | string | Version number |
| `data[].major_version` | string | Major version number |
| `data[].platform` | string | Platform (windows / linux / macos) |
| `data[].architecture` | string | Architecture (x64 / x86 / arm64) |
| `data[].size` | number | File size (bytes) |
| `data[].checksum` | string | XXH3 checksum, format is `xxh3:` + 16-digit hex |
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
    "name": "Browser Workshop",
    "version": "1.0.0",
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

## GET /bin/{filename}

Download client binary files (only available when bin directory exists).

### Request

```
GET /bin/{filename}
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
curl -O http://localhost:8080/bin/bws-windows-amd64.exe
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
