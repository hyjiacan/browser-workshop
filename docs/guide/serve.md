# Serve 服务

`bws serve` 命令可以将本地的浏览器安装包通过 HTTP 提供给其他客户端下载，适用于团队内部搭建离线分发服务。

## 概述

bws serve 是一个轻量级的 HTTP 服务，主要功能包括：

- **浏览器版本分发**：将本地存储的浏览器安装包提供给局域网内的客户端下载
- **文件清单管理**：自动扫描目录、识别文件、生成清单和校验和
- **断点续传**：支持 HTTP Range 请求，大文件下载可断点续传
- **自动同步**：可配置自动从在线源同步最新版本到本地
- **Web 管理界面**：内置 HTML 页面，方便查看和操作
- **REST API**：提供完整的 API 接口，便于集成

### 适用场景

- 企业内网无法访问外网，需要统一分发浏览器版本
- 团队内部共享浏览器版本，加速下载
- 测试环境中多台机器需要相同的浏览器版本
- 离线环境下管理浏览器版本

## 快速开始

serve 采用**配置 + 启动**分离的工作流：先通过 `bws serve set` 配置参数，再通过 `bws serve run` 启动服务。配置保存在 `bws-serve.ini` 文件中，重启后依然生效。

### 基本用法

```bash
# 1. 配置监听地址和端口
bws serve set host 0.0.0.0
bws serve set port 8080

# 2. 启动服务
bws serve run
```

启动后，可以在浏览器中访问 `http://localhost:8080` 查看 Web 界面。

### 查看配置

```bash
# 查看所有配置
bws serve show

# 查看单个配置项
bws serve get host
bws serve get port
```

### 配置项说明

| 配置项 | 默认值 | 说明 |
|--------|--------|------|
| `host` | `0.0.0.0` | 监听主机地址 |
| `port` | `8080` | 监听端口 |
| `sync` | `false` | 是否启用自动同步 |
| `schedule` / `sync-interval` | `24h` | 同步间隔（支持 30d、24h、30m 格式） |
| `sync-browsers` | 全部 | 同步的浏览器列表，逗号分隔（如 chrome,firefox） |
| `sync-channels` | `stable` | 同步的渠道列表，逗号分隔（如 stable,beta） |
| `base-dir` / `dir` | 程序目录 | 基础目录（包含 packages/ 和 bin/） |

### 配置文件

配置保存在程序所在目录的 `bws-serve.ini` 文件中：

```ini
[serve]
host = 0.0.0.0
port = 8080
sync = true
sync-interval = 30d
sync-channels = stable
```

### 启用自动同步

```bash
# 启用同步
bws serve set sync true

# 设置同步间隔为 30 天
bws serve set schedule 30d

# 指定同步的浏览器
bws serve set sync-browsers chrome,firefox

# 指定同步的渠道
bws serve set sync-channels stable,beta

# 启动服务
bws serve run
```

自动同步的详细说明请参考 [Serve 自动同步](./serve-sync.md) 章节。

## 目录结构

在基础目录下需要创建 `packages/` 目录，放入浏览器安装包。

### 完整目录结构

```
serve-root/
├── packages/               # 安装包存放目录（必需）
│   ├── Chrome_120.0.6099.109_Windows_x64.exe
│   ├── Chrome_121.0.6167.85_Windows_x64.zip
│   ├── Firefox_121.0_Windows_x64.exe
│   └── ...
├── bin/                    # 客户端二进制（可选）
│   ├── bws-windows-amd64.exe
│   └── bws-linux-amd64
└── .serve-cache.json       # 校验和缓存（自动生成）
```

### packages 目录

`packages/` 目录是必需的，存放所有浏览器安装包文件。文件名会被自动识别，支持的格式和识别规则与本地导入一致。

### bin 目录

`bin/` 目录是可选的，用于存放 bws 客户端二进制文件。客户端可以通过 Web 页面直接下载 bws 程序。

### .serve-cache.json

`.serve-cache.json` 是服务自动生成的缓存文件，存储文件的校验和（XXH3）和元数据，用于加速清单生成。不需要手动编辑。

## 文件名识别规则

serve 服务会自动识别 `packages/` 目录下的文件名，提取浏览器名称、版本号、平台、架构和渠道信息。

### 识别要素

| 要素 | 识别关键词 | 示例 |
|------|-----------|------|
| **浏览器名** | chrome, chromium, firefox, edge, brave, opera 等 | `chrome`, `firefox` |
| **版本号** | 形如 `x.y.z.w` 的数字组合 | `120.0.6099.109` |
| **架构** | `win64`/`x64`/`amd64` 或 `win32`/`x86`/`386` | `x64`, `win64` |
| **平台** | `windows`/`win`、`macos`/`mac`、`linux` | `windows`, `win` |
| **渠道** | `stable`, `beta`, `dev`, `canary`, `esr` | `stable`, `beta` |

### 可识别的文件名示例

```
Chrome_120.0.6099.109_Windows_x64.exe
GoogleChrome_148.0.7778.167_Windows_x64_Offline.exe
firefox-115.0esr-win64.zip
Chrome_121.0.6167.85_Windows_x64.zip
chromium-85.0.4183.121-linux-x64.tar.gz
MicrosoftEdge_120.0.2210.91_x64.msi
44.0.2403.107_chrome64_stable_windows_installer.exe
```

### 无法识别的文件

文件名无法识别的文件不会出现在清单中，但仍然可以通过直接下载 URL 访问。

如果文件名无法被自动识别，建议重命名文件使其包含足够的关键词，或者使用 `bws install -f` 手动指定版本安装。

## 客户端配置

客户端需要配置离线源地址才能从 serve 服务获取版本。

### 设置离线源

```bash
bws config set source http://server:8080
```

将 `server:8080` 替换为实际的 serve 服务地址。

### 验证配置

```bash
# 查看远程版本列表，应显示 serve 服务中的版本
bws ls --remote chrome

# 从离线源安装浏览器
bws install chrome@120
```

### 工作原理

配置离线源后，客户端的工作流程：

1. 执行 `bws install` 或 `bws ls --remote`
2. 优先从配置的离线源（serve 服务）获取版本清单
3. 从离线源下载安装包
4. 如果离线源没有所需版本，自动回退到内置在线源

详细的源优先级说明请参考 [配置管理](./config.md#数据源与优先级) 章节。

## Web 界面

启动 serve 服务后，在浏览器中访问服务地址即可看到 Web 界面。

Web 界面提供以下功能：

- 查看所有可用的浏览器版本
- 按浏览器、版本、平台、架构筛选
- 直接下载安装包
- 查看服务状态
- 手动触发同步（如果启用了同步功能）
- 下载 bws 客户端二进制（如果 bin 目录存在）
