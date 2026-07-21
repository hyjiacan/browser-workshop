# Serve 服务

`bws sv` 命令可以将本地的浏览器安装包通过 HTTP 提供给其他客户端下载，适用于团队内部搭建离线分发服务。

## 概述

bws sv 是一个轻量级的 HTTP 服务，主要功能包括：

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

serve 的配置通过 `bws-serve.ini` 文件管理。首次运行 `bws sv` 时会自动创建默认配置文件，编辑后重新运行即可启动服务。

### 基本用法

```bash
# 1. 首次运行（自动创建配置文件）
bws sv
# 输出: 配置文件已创建: D:\bws\bws-serve.ini
# 编辑配置文件后重新运行

# 2. 编辑 bws-serve.ini 后启动服务
bws sv
```

启动后，可以在浏览器中访问 `http://localhost:8080` 查看 Web 界面。

### 配置文件示例

```ini
# bws sv 配置文件
[serve]
host = 0.0.0.0
port = 8080
packages-dir =
bin-dir =
sync = false
sync-interval = 24h
sync-browsers =
sync-channels = stable
```

```bash
# 编辑配置文件后启动服务
bws sv
```

### 配置项说明

| 配置项 | 默认值 | 说明 |
|--------|--------|------|
| `host` | `0.0.0.0` | 监听主机地址 |
| `port` | `8080` | 监听端口 |
| `packages-dir` | 程序目录/packages | 浏览器安装包存放目录 |
| `bin-dir` | 程序目录/bin | 客户端二进制存放目录 |
| `sync` | `false` | 是否启用自动同步 |
| `sync-interval` | `24h` | 同步间隔（支持 30d、24h、30m 格式） |
| `sync-browsers` | 全部 | 同步的浏览器列表，逗号分隔（如 chrome,firefox） |
| `sync-channels` | `stable` | 同步的渠道列表，逗号分隔（如 stable,beta） |

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

编辑 `bws-serve.ini` 配置文件，设置以下选项：

```ini
[serve]
sync = true
sync-interval = 30d
sync-browsers = chrome,firefox
sync-channels = stable,beta
```

然后重新运行 `bws sv` 即可。

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

如果文件名无法被自动识别，建议重命名文件使其包含足够的关键词，或者使用 `bws i -f` 手动指定版本安装。

## 客户端配置

客户端需要配置离线源地址才能从 serve 服务获取版本。

### 设置离线源

```bash
bws cfg set source http://server:8080
```

将 `server:8080` 替换为实际的 serve 服务地址。

### 验证配置

```bash
# 查看远程版本列表，应显示 serve 服务中的版本
bws ls --remote chrome

# 从离线源安装浏览器
bws i chrome@120
```

### 工作原理

配置离线源后，客户端的工作流程：

1. 执行 `bws i` 或 `bws ls --remote`
2. 优先从配置的离线源（serve 服务）获取版本清单
3. 从离线源下载安装包
4. 如果离线源没有所需版本，自动回退到内置在线源

详细的源优先级说明请参考 [配置管理](./config.md#数据源与优先级) 章节。

## 后台运行

serve 命令默认在前台运行，适合调试和临时使用。生产环境建议配置为系统服务，实现开机自启和自动重启。

### Linux (systemd)

创建 systemd 服务文件：

```bash
sudo tee /etc/systemd/system/bws-serve.service << 'EOF'
[Unit]
Description=BWS Browser Version Serve
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/bws sv
WorkingDirectory=/usr/local/bin
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF
```

启用并启动服务：

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now bws-serve
sudo systemctl status bws-serve
```

### Windows (nssm)

推荐使用 [nssm](https://nssm.cc/)（Non-Sucking Service Manager）将 serve 注册为 Windows 服务。

```powershell
# 1. 下载 nssm 并解压到 PATH 目录
# 2. 安装服务
nssm install bws-serve

# 在弹出的窗口中配置：
#   Path:        D:\bws\bws.exe
#   Arguments:   serve
#   Startup dir: D:\bws

# 3. 启动服务
nssm start bws-serve

# 4. 查看状态
nssm status bws-serve
```

常用 nssm 命令：

| 命令 | 说明 |
|------|------|
| `nssm start bws-serve` | 启动服务 |
| `nssm stop bws-serve` | 停止服务 |
| `nssm restart bws-serve` | 重启服务 |
| `nssm remove bws-serve` | 卸载服务 |
| `nssm edit bws-serve` | 编辑服务配置 |

### 为什么不内置服务安装

bws 选择不内置系统服务安装功能，原因如下：

- **跨平台复杂度**：Windows 服务、Linux systemd、macOS launchd 差异巨大，维护成本高
- **权限问题**：安装系统服务通常需要管理员/root 权限，容易引起安全顾虑
- **灵活性**：使用 systemd/nssm 等成熟工具，用户可以更灵活地配置日志、资源限制、依赖关系等
- **专注核心**：bws 专注于浏览器版本管理，服务管理交给专业工具

## Web 界面

启动 serve 服务后，在浏览器中访问服务地址即可看到 Web 界面。

Web 界面提供以下功能：

- 查看所有可用的浏览器版本
- 按浏览器、版本、平台、架构筛选
- 直接下载安装包
- 查看服务状态
- 手动触发同步（如果启用了同步功能）
- 下载 bws 客户端二进制（如果 bin 目录存在）
