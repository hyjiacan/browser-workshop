# 配置管理

bws 的所有配置通过 `bws cfg` 命令统一管理。本章介绍配置的查看、设置以及各配置项的详细说明。

## 查看所有配置

使用 `bws cfg show` 命令查看当前所有配置项及其值。

```bash
bws cfg show
```

输出示例：

```
配置信息：

  配置文件:       D:\bws-data\config.json
  数据目录:       D:\bws-data
  默认浏览器:     chrome
  默认渠道:       stable
  日志级别:       info
  仓库路径:       （空）

  数据源开关:
    Serve 源:     true
    Omaha 源:     true
    Firefox FTP:  true

  磁盘空间阈值:   5 GB (低于此值会提示)

  别名:
    stable -> chrome@latest
    beta -> chrome@beta
```

## 获取配置项

使用 `bws cfg get` 命令获取单个配置项的值。

```bash
bws cfg get default-browser
bws cfg get log-level
bws cfg get source
```

输出示例：

```
chrome
```

## 设置配置项

使用 `bws cfg set` 命令设置配置项的值。

```bash
bws cfg set default-browser firefox
bws cfg set log-level debug
bws cfg set data-dir D:\browser-data
bws cfg set source http://server:8080
```

设置成功后会显示确认信息。

## 配置项说明

### default-browser

默认浏览器名称。

| 属性 | 值 |
|------|-----|
| 默认值 | `chrome` |
| 可选值 | `chrome`, `firefox`, `chromium` |
| 说明 | 当命令中未指定浏览器时使用的默认值 |

示例：

```bash
# 设置默认浏览器为 Firefox
bws cfg set default-browser firefox

# 设置后，以下命令运行 Firefox 的默认版本
bws r
```

### default-channel

默认发布渠道。

| 属性 | 值 |
|------|-----|
| 默认值 | `stable` |
| 可选值 | `stable`, `beta`, `dev`, `canary`, `esr` |
| 说明 | 当命令中未指定渠道时使用的默认渠道 |

示例：

```bash
# 设置默认渠道为 beta
bws cfg set default-channel beta

# 设置后，安装最新版本时默认使用 beta 渠道
bws i chrome@latest
```

### log-level

控制台日志级别。

| 属性 | 值 |
|------|-----|
| 默认值 | `info` |
| 可选值 | `trace`, `debug`, `info`, `warn`, `error`, `fatal` |
| 说明 | 控制控制台输出的日志详细程度 |

示例：

```bash
# 设置为 debug 级别，输出更多调试信息
bws cfg set log-level debug

# 设置为 warn 级别，只显示警告和错误
bws cfg set log-level warn
```

> **注意**：此配置仅影响控制台输出。文件日志始终使用 `debug` 级别，不受此配置影响。更多信息请参考 [日志系统](./logging.md) 章节。

### data-dir

数据存储目录。

| 属性 | 值 |
|------|-----|
| 默认值 | `bws-data`（便携模式，与程序同级） |
| 可选值 | 任意有效目录路径 |
| 说明 | 指定 bws 所有数据的存储位置 |

示例：

```bash
# 设置数据目录为绝对路径
bws cfg set data-dir D:\browser-data

# 设置后，所有配置、版本、日志都存储在该目录下
```

修改数据目录后，原目录中的数据不会自动迁移，需要手动移动。

### repo-path

本地仓库路径。

| 属性 | 值 |
|------|-----|
| 默认值 | 空 |
| 可选值 | 本地目录路径 |
| 说明 | 本地二进制仓库的路径，用于额外的版本来源 |

示例：

```bash
bws cfg set repo-path D:\browser-repo
```

### source / remote-source

离线源地址（bws sv 服务地址）。

| 属性 | 值 |
|------|-----|
| 默认值 | 空（不使用离线源） |
| 可选值 | HTTP URL |
| 说明 | 离线分发服务的地址，配置后优先从该源获取版本 |

示例：

```bash
# 设置离线源
bws cfg set source http://192.168.1.100:8080

# 查看当前源
bws cfg get source

# 清除离线源配置
bws cfg set source ""
```

`source` 和 `remote-source` 是等效的，设置任意一个都可以。

### source-omaha

Chrome Omaha 数据源开关。

| 属性 | 值 |
|------|-----|
| 默认值 | `true` |
| 可选值 | `true`, `false` |
| 说明 | 是否启用 Chrome Omaha 协议数据源 |

示例：

```bash
# 禁用 Omaha 源
bws cfg set source-omaha false

# 重新启用
bws cfg set source-omaha true
```

### source-firefox-ftp

Firefox FTP 数据源开关。

| 属性 | 值 |
|------|-----|
| 默认值 | `true` |
| 可选值 | `true`, `false` |
| 说明 | 是否启用 Firefox FTP 发布数据源 |

示例：

```bash
# 禁用 Firefox FTP 源
bws cfg set source-firefox-ftp false
```

### source-serve

Serve HTTP 数据源开关。

| 属性 | 值 |
|------|-----|
| 默认值 | `true` |
| 可选值 | `true`, `false` |
| 说明 | 是否启用 `bws sv` 搭建的 HTTP 分发数据源 |

示例：

```bash
# 禁用 Serve 源
bws cfg set source-serve false
```

### download.max-concurrency

下载最大并发数。

| 属性 | 值 |
|------|-----|
| 默认值 | `3` |
| 可选值 | 正整数 |
| 说明 | 同时下载文件的最大数量，值越大下载越快，但占用带宽和系统资源越多 |

示例：

```bash
# 设置为 5 个并发
bws cfg set download.max-concurrency 5
```

### download.retry-count

下载重试次数。

| 属性 | 值 |
|------|-----|
| 默认值 | `3` |
| 可选值 | 非负整数 |
| 说明 | 下载失败后的最大重试次数 |

示例：

```bash
# 设置为 5 次重试
bws cfg set download.retry-count 5
```

### download.retry-delay

下载重试间隔。

| 属性 | 值 |
|------|-----|
| 默认值 | `2s` |
| 可选值 | Go duration 格式字符串（如 `1s`、`500ms`、`1m`） |
| 说明 | 每次下载重试之间的等待时间 |

示例：

```bash
# 设置为 5 秒间隔
bws cfg set download.retry-delay 5s
```

### download.timeout

下载超时时间。

| 属性 | 值 |
|------|-----|
| 默认值 | `30m` |
| 可选值 | Go duration 格式字符串（如 `10m`、`1h`） |
| 说明 | 单个下载任务的最大超时时间，超时后将取消下载并重试或报错 |

示例：

```bash
# 设置为 1 小时超时
bws cfg set download.timeout 1h
```

### cache.manifest-ttl

版本清单缓存有效期。

| 属性 | 值 |
|------|-----|
| 默认值 | `24h` |
| 可选值 | Go duration 格式字符串（如 `12h`、`48h`） |
| 说明 | 从远程获取的版本清单（版本列表）在本地的缓存有效时长，过期后会重新拉取 |

示例：

```bash
# 设置为 12 小时
bws cfg set cache.manifest-ttl 12h
```

### cache.download-ttl

已下载文件缓存有效期。

| 属性 | 值 |
|------|-----|
| 默认值 | `168h`（7 天） |
| 可选值 | Go duration 格式字符串（如 `72h`、`336h`） |
| 说明 | 已下载的安装包在本地缓存中的保留时长，过期后会被自动清理 |

示例：

```bash
# 设置为 3 天（72 小时）
bws cfg set cache.download-ttl 72h
```

### disk-threshold

磁盘空间告警阈值。

| 属性 | 值 |
|------|-----|
| 默认值 | `5`（GB） |
| 可选值 | 正整数（单位 GB） |
| 说明 | 下载前检查磁盘剩余空间，低于此值时会提示用户 |

示例：

```bash
# 设置为 10 GB
bws cfg set disk-threshold 10
```

## 数据源与优先级

bws 支持多个版本数据源，按固定优先级顺序查询：

### 数据源列表

| 优先级 | 数据源 | 说明 | 配置方式 |
|--------|--------|------|----------|
| 1（最高） | 离线源 | 通过 `bws sv` 搭建的分发服务 | `bws cfg set source <url>` |
| 2（最低） | 内置在线源 | 浏览器官方更新渠道（Firefox FTP、Chromium GCS） | 内置，无需配置 |

### 手动下载 Chrome 历史版本

Chrome 官方不提供公开的下载链接，以下第三方站点可手动下载历史版本：

- **ChromeDownloads**: https://chromedownloads.net/ — 提供 Chrome 全平台历史版本下载

下载后可通过以下方式安装：

```bash
# 从压缩包安装
bws i --from-file chrome-120.0.6099.109-win64.zip chrome@120

# 从解压目录安装
bws i -d D:\chrome-120-win64 chrome@120
```

### 优先级规则

固定优先级：**离线源 → 在线源**（离线源优先，在线源兜底）。

### 工作流程

当执行 `bws i` 或 `bws ls --remote` 时：

1. 如果配置了离线源，首先查询离线源
2. 离线源中有匹配的版本，直接使用（或提示用户选择）
3. 离线源中没有匹配的版本，自动回退到在线源
4. 在线源中找到匹配版本，使用在线源
5. 两个源都找不到，报错

### 优势

- **离线环境**：配置离线源后，即使无法访问外网也能安装浏览器
- **加速下载**：局域网内下载速度远快于外网
- **版本可控**：管理员可以控制团队使用的浏览器版本
- **自动降级**：离线源没有的版本自动从在线源获取，不影响使用

## 配置文件

配置以 JSON 格式存储在数据目录下的 `config.json` 文件中：

```
bws-data/
└── config.json
```

通常不需要手动编辑配置文件，建议使用 `bws cfg` 命令进行管理。

## 配置命令汇总

| 命令 | 说明 |
|------|------|
| `bws cfg show` | 查看所有配置 |
| `bws cfg get <key>` | 获取指定配置项的值 |
| `bws cfg set <key> <value>` | 设置指定配置项的值 |
