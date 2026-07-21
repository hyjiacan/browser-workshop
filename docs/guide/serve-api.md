# Serve API 参考

bws sv 提供了完整的 REST API 接口，用于查询文件清单、下载文件、管理同步等。本文档详细说明所有 API 端点的用法。

## API 端点总览

| 路径 | 方法 | 说明 |
|------|------|------|
| `/` | GET | HTML 帮助页 / Web 界面 |
| `/api/v1/manifest` | GET | 文件清单（含 XXH3 校验和） |
| `/api/v1/download/{filename}` | GET | 文件下载（支持断点续传） |
| `/api/v1/status` | GET | 服务状态 |
| `/api/v1/sync/status` | GET | 同步状态 |
| `/api/v1/sync/trigger` | POST | 手动触发同步 |
| `/bin/{filename}` | GET | 客户端二进制下载 |

## 基础信息

### 基础 URL

所有 API 的基础 URL 为 serve 服务的地址，例如：

```
http://localhost:8080
http://192.168.1.100:8080
```

### 数据格式

- 响应格式：JSON（除非另有说明）
- 字符编码：UTF-8
- 时间格式：ISO 8601（如 `2024-01-15T10:30:00Z`）

### 错误处理

API 出错时返回标准 HTTP 状态码，响应体为**纯文本**错误描述（非 JSON）。

例如：

```
file not found
```

```
method not allowed
```

常见 HTTP 状态码：

| 状态码 | 说明 |
|--------|------|
| 200 OK | 请求成功 |
| 400 Bad Request | 请求参数错误 |
| 404 Not Found | 资源不存在 |
| 405 Method Not Allowed | 方法不允许 |
| 500 Internal Server Error | 服务器内部错误 |
| 503 Service Unavailable | 服务不可用（如同步功能未启用） |

---

## GET /

HTML 帮助页，即 Web 管理界面。

### 请求

```
GET /
```

### 响应

返回 HTML 页面，包含：

- 可用浏览器版本列表
- 文件下载链接
- 服务状态信息
- 同步控制（如果启用了同步）
- 客户端二进制下载（如果 bin 目录存在）

---

## GET /api/v1/manifest

获取文件清单，包含所有可识别的浏览器安装包信息及其 XXH3 校验和。

### 请求

```
GET /api/v1/manifest
```

### 响应示例

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

### 响应字段说明

| 字段 | 类型 | 说明 |
|------|------|------|
| `status` | string | 固定为 `"ok"` |
| `data` | array | 文件列表 |
| `data[].filename` | string | 文件名（相对路径） |
| `data[].version` | string | 版本号 |
| `data[].major_version` | string | 主版本号 |
| `data[].platform` | string | 平台（windows / linux / macos） |
| `data[].architecture` | string | 架构（x64 / x86 / arm64） |
| `data[].size` | number | 文件大小（字节） |
| `data[].checksum` | string | XXH3 校验和，格式为 `xxh3:` + 16 位十六进制 |
| `server` | object | 服务端信息 |
| `server.name` | string | 服务名称 |
| `server.version` | string | 服务端版本号 |
| `server.file_count` | number | 文件总数 |

### 使用示例

```bash
# 获取完整清单
curl http://localhost:8080/api/v1/manifest
```

---

## GET /api/v1/download/{filename}

下载指定文件，支持断点续传。

### 请求

```
GET /api/v1/download/{filename}
```

### 路径参数

| 参数 | 类型 | 说明 |
|------|------|------|
| filename | string | 要下载的文件名 |

### 响应头

| 响应头 | 说明 |
|--------|------|
| Content-Type | application/octet-stream |
| Content-Length | 文件大小（字节） |
| Content-Disposition | 附件形式下载 |
| Accept-Ranges | bytes（支持断点续传） |
| ETag | 文件校验和 |

### 断点续传

支持 HTTP Range 请求，可从指定位置继续下载：

```bash
# 从第 1000000 字节开始下载
curl -H "Range: bytes=1000000-" http://localhost:8080/api/v1/download/file.exe
```

### 错误响应

| 状态码 | 说明 |
|--------|------|
| 404 | 文件不存在 |

### 使用示例

```bash
# 下载文件
curl -O http://localhost:8080/api/v1/download/Chrome_120.0.6099.109_Windows_x64.exe

# 使用 wget 下载（支持断点续传）
wget -c http://localhost:8080/api/v1/download/Chrome_120.0.6099.109_Windows_x64.exe
```

---

## GET /api/v1/status

获取服务状态信息。

### 请求

```
GET /api/v1/status
```

### 响应示例

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

### 响应字段说明

| 字段 | 类型 | 说明 |
|------|------|------|
| `status` | string | 固定为 `"ok"` |
| `server` | object | 服务端信息 |
| `server.name` | string | 服务名称 |
| `server.version` | string | 服务端版本号 |
| `server.uptime` | number | 服务已运行秒数 |
| `server.file_count` | number | 文件总数 |
| `server.total_size` | number | 文件总大小（字节） |

### 使用示例

```bash
curl http://localhost:8080/api/v1/status
```

---

## GET /api/v1/sync/status

获取同步状态。未配置同步源时仍返回 200，`data.progress` 中包含提示信息。

### 请求

```
GET /api/v1/sync/status
```

### 响应示例（同步已启用）

```json
{
  "status": "ok",
  "data": {
    "running": false,
    "last_sync": "2024-01-15T10:30:00Z",
    "next_sync": "2024-01-16T10:30:00Z",
    "last_error": "",
    "progress": "同步完成",
    "total_files": 15,
    "synced_files": 15
  }
}
```

### 响应示例（同步未启用）

```json
{
  "status": "ok",
  "data": {
    "running": false,
    "last_sync": "0001-01-01T00:00:00Z",
    "next_sync": "0001-01-01T00:00:00Z",
    "last_error": "",
    "progress": "同步未启用（未配置同步源）",
    "total_files": 0,
    "synced_files": 0
  }
}
```

### 响应字段说明

| 字段 | 类型 | 说明 |
|------|------|------|
| `status` | string | 固定为 `"ok"` |
| `data` | object | 同步状态信息 |
| `data.running` | boolean | 当前是否正在同步 |
| `data.last_sync` | string | 上次同步完成时间（ISO 8601，零值为未同步过） |
| `data.next_sync` | string | 下次计划同步时间（ISO 8601，零值为无计划） |
| `data.last_error` | string | 上次同步错误信息（为空表示无错误，可能省略） |
| `data.progress` | string | 当前同步进度描述（可能省略） |
| `data.total_files` | number | 同步任务的总文件数 |
| `data.synced_files` | number | 已同步的文件数 |

---

## POST /api/v1/sync/trigger

手动触发同步。如果同步已在运行中，则静默忽略（no-op），仍返回 200。

### 请求

```
POST /api/v1/sync/trigger
```

### 成功响应

```json
{
  "status": "ok",
  "message": "同步已触发"
}
```

### 错误响应

同步功能未启用时返回 503 状态码，响应体为纯文本：

```
sync not enabled
```

### 使用示例

```bash
curl -X POST http://localhost:8080/api/v1/sync/trigger
```

---

## GET /bin/{filename}

下载客户端二进制文件（仅在 bin 目录存在时可用）。

### 请求

```
GET /bin/{filename}
```

### 路径参数

| 参数 | 类型 | 说明 |
|------|------|------|
| filename | string | 二进制文件名 |

### 响应

返回文件内容，支持断点续传，与下载接口行为一致。

### 使用示例

```bash
# 下载 Windows 版本的 bws
curl -O http://localhost:8080/bin/bws-windows-amd64.exe
```

---

## 错误处理示例

### 文件不存在

```
file not found
```

HTTP 状态码：`404 Not Found`

### 同步功能未启用

```
sync not enabled
```

HTTP 状态码：`503 Service Unavailable`

### 方法不允许

```
method not allowed
```

HTTP 状态码：`405 Method Not Allowed`
