# 配置管理

bws 的所有配置通过 `bws cfg` 命令统一管理。本章介绍配置的查看、设置以及各配置项的详细说明。

## 查看所有配置

使用 `bws cfg show` 命令查看当前所有配置项及其值。

```bash
bws cfg show
```

输出示例：

```json
{
  "default-browser": "chrome",
  "default-channel": "stable",
  "log-level": "info",
  "data-dir": "bws-data",
  "repo-path": "",
  "source": ""
}
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
| 可选值 | `chrome`, `firefox`, `chromium`, `edge` 等 |
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

## 数据源与优先级

bws 支持多个版本数据源，按固定优先级顺序查询：

### 数据源列表

| 优先级 | 数据源 | 说明 | 配置方式 |
|--------|--------|------|----------|
| 1（最高） | 离线源 | 通过 `bws sv` 搭建的分发服务 | `bws cfg set source <url>` |
| 2（最低） | 内置在线源 | 浏览器官方更新渠道（Chrome Omaha 等） | 内置，无需配置 |

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
