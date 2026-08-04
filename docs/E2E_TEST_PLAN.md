# E2E 测试计划：Serve 模式客户端与服务端交互

> 覆盖 `bws serve` HTTP 服务全部 API 端点及客户端 `HTTPSource` 与之的完整交互流程。

## 1. 测试范围

### 服务端 API 端点

| 端点 | 方法 | 用途 |
|------|------|------|
| `/api/v1/manifest` | GET | 文件清单（本地 + 在线缓存） |
| `/api/v1/download/{filename}` | GET | 软件包下载（支持 Range） |
| `/api/v1/status` | GET | 服务状态 |
| `/api/v1/sync/status` | GET | 同步状态 |
| `/api/v1/sync/trigger` | POST | 手动触发同步 |
| `/api/v1/bin/{filename}` | GET | 客户端二进制下载 |
| `/` | GET | HTML 帮助页面 |

### 客户端交互

| 客户端方法 | 调用端点 | 验证项 |
|-----------|---------|--------|
| `HTTPSource.List()` | manifest | 版本列表获取 + 客户端过滤 |
| `HTTPSource.Latest()` | manifest | 最新版本选取 |
| `HTTPSource.Resolve()` | manifest | 精确/前缀版本匹配 |
| 下载 URL 构造 | download | URL 格式正确性 |
| manifest 缓存 | — | 30s TTL 单次请求 |

### 跨模块交互

| 场景 | 涉及组件 | 验证项 |
|------|---------|--------|
| 在线回退下载 | Server + SyncSource | 缺失文件按需下载 |
| manifest 合并 | Server + OnlineCacheManager | 本地 + 在线条目合并去重 |
| 认证中间件 | authMiddleware | Bearer Token 保护 API |
| 包扫描 | Server + Scanner | 支持扩展名过滤 + 校验和计算 |

## 2. 测试架构

### 文件布局

```
internal/serve/e2e_test.go          — 服务端 E2E 测试
internal/source/http_integration_test.go — 客户端集成测试
```

### 测试基础设施

- **真实 HTTP 服务**：通过 `NewServerWithOptions` 启动真实 Server，监听随机端口
- **真实 HTTP 请求**：使用 `net/http.Client` 发送请求，验证完整 HTTP 语义
- **mockSyncSource**：实现 `serve.SyncSource` 接口，用于在线回退测试
- **临时目录隔离**：每个测试用例使用 `t.TempDir()` 创建独立的 packages/bin 目录

### 测试助手

```
startTestServer(t, opts) → *testServer
  - 创建临时目录
  - 创建测试文件
  - 启动 Server（随机端口）
  - 轮询 /api/v1/status 直到就绪
  - 返回 baseURL + cleanup 函数
```

## 3. 测试用例

### 3.1 Manifest API

| ID | 用例 | 预期 |
|----|------|------|
| M-01 | 空清单（无包） | status=ok, data=[], file_count=0 |
| M-02 | 本地包清单 | 返回所有支持的扩展名文件 |
| M-03 | 查询参数传递 | browser/platform/arch/channel 参数到达 |
| M-04 | 合并在线缓存 | 本地 + 在线条目去重合并 |
| M-05 | 服务器信息 | name=bws-serve, version 正确 |
| M-06 | 非 GET 方法 | 405 Method Not Allowed |

### 3.2 Download API

| ID | 用例 | 预期 |
|----|------|------|
| D-01 | 下载本地文件 | 200, Content-Disposition 正确 |
| D-02 | 文件不存在（无回退） | 404 |
| D-03 | 文件不存在（有回退） | 200, 文件被按需下载 |
| D-04 | 空文件名 | 400 |
| D-05 | 路径遍历攻击 | 400 |
| D-06 | POST 方法 | 405 |
| D-07 | Range 请求 | 206 Partial Content |

### 3.3 Status API

| ID | 用例 | 预期 |
|----|------|------|
| S-01 | 基本状态 | status=ok, uptime>0, file_count 正确 |
| S-02 | POST 方法 | 405 |

### 3.4 Sync API

| ID | 用例 | 预期 |
|----|------|------|
| SY-01 | 同步状态（未启用） | running=false |
| SY-02 | 同步状态（已启用） | running 字段存在 |
| SY-03 | 触发同步（未启用） | 503 |
| SY-04 | 触发同步（已启用） | 200, status=ok |
| SY-05 | GET 触发同步 | 405 |

### 3.5 Bin Download API

| ID | 用例 | 预期 |
|----|------|------|
| B-01 | 下载 bin 文件 | 200 |
| B-02 | bin 不存在 | 404 |
| B-03 | 路径遍历 | 400 |
| B-04 | 空文件名 | 404 |

### 3.6 Root HTML

| ID | 用例 | 预期 |
|----|------|------|
| R-01 | HTML 页面 | 200, Content-Type: text/html |
| R-02 | 文件计数显示 | HTML 包含正确 file_count |
| R-03 | 未知路径 | 404 |

### 3.7 认证中间件

| ID | 用例 | 预期 |
|----|------|------|
| A-01 | 无 Token 访问 API | 401 |
| A-02 | 正确 Token 访问 API | 200 |
| A-03 | 错误 Token | 401 |
| A-04 | 根页面无需认证 | 200 |
| A-05 | 无 Token 配置时 API 开放 | 200 |

### 3.8 在线回退

| ID | 用例 | 预期 |
|----|------|------|
| OF-01 | 按需下载缺失文件 | 200, 文件内容正确 |
| OF-02 | manifest 包含在线条目 | data 包含在线版本 |
| OF-03 | SHA256 校验失败 | 404 |
| OF-04 | 在线源无此文件 | 404 |
| OF-05 | 下载后文件出现在 manifest | 二次请求 manifest 包含新文件 |

### 3.9 包扫描

| ID | 用例 | 预期 |
|----|------|------|
| PS-01 | 支持的扩展名被扫描 | zip/exe/tar.bz2 出现在 manifest |
| PS-02 | 不支持的扩展名被跳过 | txt/json/md 不出现 |
| PS-03 | 校验和格式 | checksum 以 "xxh3:" 开头 |

### 3.10 客户端 HTTPSource 集成

| ID | 用例 | 预期 |
|----|------|------|
| C-01 | List 返回过滤后的版本 | 仅匹配 browser/platform/arch 的版本 |
| C-02 | Latest 返回最新版本 | 版本号最高 |
| C-03 | Resolve 精确匹配 | 返回指定版本 |
| C-04 | Resolve 前缀匹配 | 返回最高匹配版本 |
| C-05 | Resolve "latest" | 返回最新版本 |
| C-06 | Resolve 版本不存在 | 返回 error |
| C-07 | 空清单 | 返回空列表，无错误 |
| C-08 | manifest 缓存 | 多次 List 仅一次 HTTP 请求 |
| C-09 | 下载 URL 构造 | URL 格式为 baseURL/api/v1/download/filename |
| C-10 | SupportsBrowser | 始终返回 true |

## 4. 测试优先级

- **P0**（核心流程）：M-01, M-02, D-01, D-02, S-01, OF-01, C-01, C-03
- **P1**（重要交互）：D-03, D-05, OF-02, A-01~A-05, C-02, C-04, C-05
- **P2**（边界情况）：其余用例

## 5. 运行方式

```bash
# 运行全部 e2e 测试
cd bws
go test ./internal/serve/ -run TestE2E -v
go test ./internal/source/ -run TestHTTPSourceIntegration -v

# 运行所有测试
go test ./...
```
