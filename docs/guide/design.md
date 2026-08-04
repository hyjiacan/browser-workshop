# 架构设计

本文档描述 Browser Workshop 的整体架构设计、核心数据流、工作流程和交互时序，作为后续版本迭代的开发依据。

## 系统架构

bws 由客户端（CLI）和服务端（Serve）两部分组成，采用 Go 语言实现，所有数据以文件形式存储，无需外部数据库。

### 架构概览

```mermaid
graph TB
    subgraph Client["客户端 (bws CLI)"]
        CLI["命令行接口<br/>internal/cli"]
        CFG["配置管理<br/>internal/config"]
        SRC["数据源<br/>internal/source"]
        DL["下载器<br/>internal/download"]
        INST["安装器<br/>internal/install"]
        LAUNCH["启动器<br/>internal/launch"]
        LOG["日志系统<br/>internal/log"]
        FP["指纹隔离<br/>internal/fingerprint"]
        PLUGIN["插件系统<br/>internal/plugin"]
    end

    subgraph Serve["服务端 (bws serve)"]
        HTTP["HTTP 服务<br/>internal/serve"]
        SYNC["同步管理<br/>internal/serve/sync.go"]
        OF["在线回退<br/>internal/serve/serve.go"]
    end

    subgraph Storage["存储"]
        DATA["bws-data/"]
        PKG["packages/"]
        BIN["bin/"]
    end

    subgraph Online["在线源"]
        FFTP["Firefox FTP"]
    end

    CLI --> CFG
    CLI --> SRC
    CLI --> DL
    CLI --> INST
    CLI --> LAUNCH
    CLI --> LOG
    LAUNCH --> FP
    LAUNCH --> PLUGIN

    SRC -->|HTTP| HTTP
    SRC --> FFTP

    HTTP --> SYNC
    HTTP --> OF
    OF --> FFTP

    HTTP --> PKG
    HTTP --> BIN
    CFG --> DATA
    DL --> DATA
    INST --> DATA
    LAUNCH --> DATA
    LOG --> DATA
```

### 模块职责

| 模块 | 职责 |
|------|------|
| `internal/cli` | 命令解析、参数处理、命令分发 |
| `internal/config` | INI 配置读写、默认值管理 |
| `internal/source` | 多数据源抽象（Serve HTTP、Firefox FTP） |
| `internal/download` | 并发下载、断点续传、重试机制 |
| `internal/install` | 浏览器安装、压缩包解压、文件识别 |
| `internal/launch` | 浏览器启动、参数构建、Profile 管理 |
| `internal/fingerprint` | 指纹隔离参数生成（User-Agent、分辨率、WebRTC 等） |
| `internal/plugin` | Lua 脚本插件和 IPC 进程插件管理 |
| `internal/log` | 双输出日志（文件+控制台）、级别控制、日志轮转 |
| `internal/serve` | HTTP 服务、文件分发、清单生成、在线回退 |

---

## 客户端与服务端交互

### 清单查询时序

客户端执行 `bws ls -R <browser>` 时的完整交互流程：

```mermaid
sequenceDiagram
    actor User
    participant Client as bws 客户端
    participant Serve as bws serve
    participant Online as 在线源

    User->>Client: bws ls -R firefox@68.9.0esr
    Client->>Client: 构建 Filter（browser=firefox）
    Client->>Serve: GET /api/v1/manifest
    Serve->>Serve: 扫描本地 packages/ 目录
    Serve->>Serve: 按扩展名清单过滤文件
    alt online-fallback 启用
        Serve->>Online: 查询在线缓存（按浏览器分文件缓存）
        Online-->>Serve: 返回版本列表
        Serve->>Serve: 合并本地 + 在线缓存清单
    end
    Serve-->>Client: JSON Manifest
    Client->>Client: 本地过滤（版本前缀匹配）
    Client->>Client: CJK 宽度计算，对齐输出
    Client-->>User: 表格结果
```

### 安装流程时序

客户端执行 `bws i chrome@120` 时的完整交互流程：

```mermaid
sequenceDiagram
    actor User
    participant Client as bws 客户端
    participant Serve as bws serve
    participant Online as 在线源

    User->>Client: bws i chrome@120
    Client->>Client: 解析 browser=chrome, version=120

    alt 配置了 Serve 源
        Client->>Serve: GET /api/v1/manifest
        Serve-->>Client: 返回清单（含在线缓存）
        alt 清单中匹配到版本
            Client->>Serve: GET /api/v1/download/Chrome_120.xxx.exe
            alt Serve 本地有文件
                Serve-->>Client: 返回文件流
            else Serve 本地无文件且 online-fallback 启用
                Serve->>Online: 下载文件到本地
                Online-->>Serve: 文件数据
                Serve-->>Client: 返回文件流
            end
        end
    end

    alt Serve 未匹配或无 Serve 源
        Client->>Online: 直接查询 Firefox FTP
        Online-->>Client: 返回下载地址
        Client->>Online: 直接下载
        Online-->>Client: 文件数据
    end

    Client->>Client: 解压安装
    Client->>Client: 写入版本描述符
    Client-->>User: 安装完成
```

### 在线回退详细时序

当 Serve 启用 `online-fallback` 且客户端请求的文件本地不存在时：

```mermaid
sequenceDiagram
    participant Client as bws 客户端
    participant Serve as bws serve
    participant Cache as 在线缓存
    participant Online as 在线源

    Client->>Serve: GET /api/v1/download/Chrome_120.exe
    Serve->>Serve: 检查本地 packages/
    alt 本地存在
        Serve-->>Client: 直接返回文件
    else 本地不存在
        Serve->>Cache: 查找文件下载 URL
        alt 缓存中存在 URL
            Serve->>Online: 按缓存 URL 下载
            Online-->>Serve: 文件数据
            Serve->>Serve: 保存到 packages/
            Serve->>Serve: 重新扫描更新清单
            Serve-->>Client: 返回文件流
        else 缓存中无此文件
            Serve-->>Client: 404 Not Found
        end
    end
```

---

## 核心数据流

### 版本查询数据流

```mermaid
graph LR
    User["用户输入<br/>bws ls -R chrome@120"]
    Parser["命令解析器<br/>解析 browser/channel/version"]
    Filter["Filter 对象<br/>browser=chrome<br/>versionPrefix=120"]
    SourceMgr["源管理器<br/>按优先级遍历源"]
    ServeSrc["Serve HTTP 源"]
    FFTP["Firefox FTP 源"]
    Merger["结果合并与去重"]
    LocalFilter["本地过滤<br/>版本前缀匹配"]
    Output["表格输出<br/>CJK 宽度计算"]

    User --> Parser
    Parser --> Filter
    Filter --> SourceMgr
    SourceMgr --> ServeSrc
    SourceMgr --> FFTP
    ServeSrc --> Merger
    FFTP --> Merger
    Merger --> LocalFilter
    LocalFilter --> Output
```

### Serve 启动数据流

```mermaid
graph TB
    Start["bws sv"]
    LoadCfg["加载 bws-serve.ini"]
    EnsureDir["创建 bws-data/ 目录"]
    LoadCache["加载 .serve-cache.json"]
    ScanDir["扫描 packages/ 目录"]
    FilterExt["扩展名过滤<br/>仅处理支持格式"]
    Parallel["并行计算校验和<br/>scan-workers 线程"]
    BuildManifest["构建文件清单"]
    PreloadCache["后台预加载在线缓存<br/>按浏览器分文件刷新"]
    StartHTTP["启动 HTTP 服务"]
    PrintInfo["输出启动信息<br/>包数量/总大小/API 列表"]

    Start --> LoadCfg
    LoadCfg --> EnsureDir
    EnsureDir --> LoadCache
    LoadCache --> ScanDir
    ScanDir --> FilterExt
    FilterExt --> Parallel
    Parallel --> BuildManifest
    BuildManifest --> StartHTTP
    StartHTTP --> PreloadCache
    StartHTTP --> PrintInfo
```

---

## 工作流程

### 浏览器启动工作流程

```mermaid
flowchart TD
    A["用户: bws r chrome@120"] --> B["解析 browser + version"]
    B --> C{"版本是否已安装?"}
    C -->|否| D["报错: 版本未安装"]
    C -->|是| E["加载版本描述符"]
    E --> F["确定可执行文件路径"]
    F --> G["构建启动参数"]
    G --> H{"是否指定 Profile?"}
    H -->|是| I["使用命名 Profile"]
    H -->|否| J["使用版本默认 Profile"]
    I --> K
    J --> K["加载插件"]
    K --> L{"是否启用指纹隔离?"}
    L -->|standard| M["生成标准指纹参数"]
    L -->|random| N["生成随机指纹参数"]
    L -->|none| O["跳过指纹"]
    M --> P["合并启动参数"]
    N --> P
    O --> P
    P --> Q{"是否启用代理?"}
    Q -->|是| R["添加代理参数"]
    Q -->|否| S
    R --> S["执行 pre_run 插件钩子"]
    S --> T["启动浏览器进程"]
    T --> U["执行 post_run 插件钩子"]
    U --> V["等待进程退出<br/>或后台运行"]
```

### 配置管理工作流程

```mermaid
flowchart TD
    A["用户: bws cfg set key value"] --> B["解析 key 和 value"]
    B --> C{"key 是否有效?"}
    C -->|否| D["报错: 未知配置项"]
    C -->|是| E["校验 value 合法性"]
    E --> F{"校验是否通过?"}
    F -->|否| G["报错: 无效值"]
    F -->|是| H["更新内存配置对象"]
    H --> I["序列化为 INI 格式"]
    I --> J["写入 bws-client.ini"]
    J --> K["输出成功信息"]
```

---

## 日志系统架构

### 日志输出架构

```mermaid
graph TB
    subgraph App["应用程序"]
        CMD["命令执行"]
        SRV["Serve HTTP"]
    end

    subgraph Logger["Logger 实例"]
        L1["LevelFilter<br/>console-level"]
        L2["LevelFilter<br/>file-level"]
        FMT1["Formatter<br/>控制台格式"]
        FMT2["Formatter<br/>文件格式"]
    end

    subgraph Outputs["输出目标"]
        CON["stderr<br/>控制台"]
        ROT["RotatingFile<br/>日志轮转"]
        FILE1["bws.log"]
        FILE2["bws.log"]
    end

    CMD -->|日志事件| Logger
    SRV -->|日志事件| Logger
    Logger --> L1
    Logger --> L2
    L1 -->|通过| FMT1
    L1 -->|过滤| DROP1["丢弃"]
    L2 -->|通过| FMT2
    L2 -->|过滤| DROP2["丢弃"]
    FMT1 --> CON
    FMT2 --> ROT
    ROT --> FILE1
    ROT --> FILE2
```

### HTTP 请求日志流程

```mermaid
sequenceDiagram
    participant Client as HTTP 客户端
    participant Middleware as loggingHandler
    participant Handler as 业务 Handler
    participant Logger as bmlog.Logger

    Client->>Middleware: HTTP Request
    Middleware->>Middleware: 记录开始时间
    Middleware->>Handler: 转发请求
    Handler-->>Middleware: 返回响应
    Middleware->>Middleware: 计算耗时
    alt 路径不是 /api/v1/status
        Middleware->>Logger: 写入日志<br/>METHOD PATH STATUS DURATION IP
    else 健康检查端点
        Middleware->>Middleware: 跳过日志记录
    end
    Middleware-->>Client: 返回响应
```

---

## 插件系统架构

### 插件执行流程

```mermaid
sequenceDiagram
    participant Client as bws 客户端
    participant PluginMgr as Plugin Manager
    participant LuaVM as Gopher-Lua VM
    participant IPC as IPC 进程插件
    participant Browser as 浏览器进程

    Client->>PluginMgr: 加载 --plugin 指定的插件
    PluginMgr->>PluginMgr: 按后缀区分类型

    alt .lua 脚本插件
        PluginMgr->>LuaVM: 加载 Lua 脚本
        LuaVM->>LuaVM: 注册 ctx API
    else 可执行文件插件
        PluginMgr->>IPC: 启动子进程
        IPC->>IPC: 建立 stdin/stdout JSON-RPC
    end

    Client->>PluginMgr: 触发 pre_run 钩子
    PluginMgr->>LuaVM: 调用 pre_run()
    LuaVM-->>PluginMgr: 返回修改后的参数
    PluginMgr->>IPC: 发送 pre_run 请求
    IPC-->>PluginMgr: 返回 extraArgs/env
    PluginMgr->>Client: 合并插件返回的参数
    Client->>Browser: 启动浏览器

    Client->>PluginMgr: 触发 post_run 钩子
    PluginMgr->>LuaVM: 调用 post_run()
    PluginMgr->>IPC: 发送 post_run 请求
```

---

## 数据模型

### 版本信息（VersionInfo）

```go
type VersionInfo struct {
    Browser      string   // 浏览器名称: chrome, firefox, chromium
    Version      string   // 完整版本号: 120.0.6099.109
    Channel      Channel  // 发布渠道: stable, beta, dev, canary, esr
    Platform     Platform // 平台: windows, linux, macos
    Arch         Arch     // 架构: amd64, 386, arm64
    DownloadURL  string   // 下载地址
    Size         int64    // 文件大小（字节）
    SHA256       string   // 校验和
}
```

### 配置文件结构（INI）

```ini
[client]
default-browser = chrome
default-channel = stable
language = zh
data-dir =
repo-path =
remote-source =

[log]
console-level = info
file-level = debug
max-size-mb = 10
max-backups = 5

[download]
max-concurrency = 3
retry-count = 3
retry-delay = 2s
timeout = 30m

[cache]
manifest-ttl = 24h
download-ttl = 168h

[source-switches]
enable-serve-source = true
enable-firefox-ftp = true

[network]
proxy =
disk-space-threshold-gb = 5
```

### Serve 配置文件结构（INI）

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
online-fallback = true
scan-workers = 0
```

---

## 关键设计决策

### 1. 无数据库设计

所有数据以文件形式存储（INI 配置、JSON 缓存、文件系统作为版本仓库），无需外部依赖，便于便携和备份。

### 2. 数据源优先级

固定优先级：**Serve 离线源 > Firefox FTP**。优先级不可配置，通过开关控制各源启用/禁用。

### 3. 单配置文件管理

客户端配置和 Serve 配置完全分离，均位于与 bws 可执行文件同目录下：
- 客户端：`bws-client.ini`（INI 格式）
- Serve：`bws-serve.ini`（INI 格式）

不再支持 JSON 格式，不保留向后兼容。

### 4. 并发控制

- **下载并发**：通过 `download.max-concurrency` 控制（默认 3）
- **扫描并发**：通过 `serve.scan-workers` 控制（默认 CPU 核心数）
- **在线回退去重**：使用 `dlMu + dlInflight` map 确保同一文件并发下载只触发一次

### 5. 缓存策略

- **清单缓存**：`cache.manifest-ttl`（默认 24h），`--refresh` 强制刷新
- **下载缓存**：`cache.download-ttl`（默认 168h）
- **Firefox FTP 缓存**：独立缓存文件 `firefox-ftp-cache.json`，24h 有效期
- **Serve 在线缓存**：`onlineCacheTTL`（24 小时），按浏览器分文件存储（`online-cache-<browser>.json`），`onlineListTimeout`（30 秒）

### 6. 日志共用

客户端和 Serve 共用同一日志文件：
- 客户端和 Serve 均写入：`logs/bws.log`
- 日志级别、轮转参数可分别配置
