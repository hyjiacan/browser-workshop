# 命令参考

本文档列出 bws 的所有命令及其详细说明，包括用途、用法、示例和参数。

> 版本信息通过全局标志 `--version` / `-v` 获取，例如 `bws --version` 或 `bws -v`。

## 命令总览

| 命令 | 说明 |
|------|------|
| `bws list` / `bws ls` | 列出已安装的浏览器版本 |
| `bws info` / `bws show` | 显示版本详细信息 |
| `bws run` / `bws r` / `bws open` | 运行指定版本的浏览器 |
| `bws install` / `bws i` | 安装浏览器版本 |
| `bws shortcut` / `bws sc` | 管理桌面快捷方式 |
| `bws import` / `bws imp` | 从目录批量导入（自动识别） |
| `bws uninstall` / `bws rm` / `bws remove` | 卸载浏览器版本 |
| `bws use` / `bws u` | 设置默认浏览器版本 |
| `bws download` / `bws dl` | 仅下载不安装 |
| `bws profile` / `bws pf` | 管理浏览器 Profile |
| `bws alias` | 管理版本别名 |
| `bws serve` / `bws sv` / `bws server` | 启动 HTTP 分发服务 |
| `bws config` / `bws cfg` | 管理配置 |
| `bws repo` | 管理本地二进制仓库 |
| `bws cache` / `bws cc` | 管理下载缓存 |
| `bws doctor` / `bws dt` | 系统健康检查 |
| `bws help` / `bws h` | 显示帮助信息 |

---

## bws list (别名: ls)

列出已安装的浏览器版本。

### 用法

```bash
bws ls [浏览器[@版本]] [选项]
```

### 参数

| 参数 | 说明 |
|------|------|
| `浏览器[@版本]` | 可选，按浏览器和版本前缀筛选 |

### 选项

| 选项 | 简写 | 说明 |
|------|------|------|
| `--remote` | `-R` | 列出远程可用版本 |
| `--all` | `-a` | 显示所有浏览器 |
| `--no-system` | - | 不显示系统浏览器 |
| `--channel <渠道>` | `-c` | 指定渠道（仅远程列表有效） |
| `--limit <数量>` | `-n` | 限制结果数量（默认 20，仅远程列表有效） |
| `--json` | - | 以 JSON 格式输出 |

### 示例

```bash
# 列出所有已安装版本
bws ls

# 只列出 Chrome
bws ls chrome

# 使用短别名
bws ls gc

# 按版本前缀筛选
bws ls chrome@79

# 列出远程可用版本
bws ls -R chrome

# 显示所有浏览器
bws ls -a

# 不显示系统浏览器
bws ls --no-system

# 列出指定渠道的远程版本
bws ls -R chrome -c beta

# 限制远程结果数量
bws ls -R chrome -n 5

# JSON 格式输出
bws ls --json
```

---

## bws info (别名: show)

显示指定版本的详细信息。

### 用法

```bash
bws show <浏览器@版本>
```

### 参数

| 参数 | 说明 |
|------|------|
| `浏览器@版本` | 要查看的浏览器版本（支持部分版本号） |

### 示例

```bash
# 查看指定版本详情
bws show chrome@120

# 查看完整版本
bws show chrome@120.0.6099.109

# 查看系统浏览器信息
bws show chrome@system

# 使用短别名
bws show ff@121
```

### 输出内容

- 浏览器名称和版本号
- 发布渠道
- 安装路径
- 架构信息
- Profile 路径
- 可执行文件路径
- 安装来源

---

## bws run (别名: r, open)

运行指定版本的浏览器。

### 用法

```bash
bws r [浏览器[@版本]] [URL] [选项] [-- 原生参数]
```

### 参数

| 参数 | 说明 |
|------|------|
| `浏览器[@版本]` | 要运行的浏览器版本，省略时使用默认浏览器和默认版本 |
| `URL` | 可选，启动时打开的网址 |

### 选项

| 选项 | 简写 | 说明 |
|------|------|------|
| `--headless` | `-H` | 无头模式 |
| `--incognito` | `-i` | 隐身/无痕模式 |
| `--new-window` | `-w` | 新窗口打开 |
| `--profile <name>` | `-p` | 指定命名 Profile |
| `--native` | - | 原生模式（使用系统 Profile） |
| `--detach` | `-d` | 后台运行（不等待进程） |
| `--dry-run` | - | 试运行（不实际启动） |
| `--proxy <url>` | - | 代理地址（如 `socks5://127.0.0.1:1080`），留空使用全局配置 |
| `--no-proxy` | - | 禁用代理（覆盖全局配置） |
| `--fingerprint <preset>` | `-fp` | 指纹隔离预设（`standard`/`random`/`none`），或 JSON 配置/@文件路径 |
| `--` | - | 之后的参数原样传递给浏览器 |

### 示例

```bash
# 运行指定版本
bws r chrome@120

# 运行默认版本
bws r chrome

# 运行系统版本
bws r chrome@system

# 打开指定 URL
bws r chrome@120 https://example.com

# 无头模式
bws r chrome@120 -H

# 隐身模式
bws r chrome@120 -i

# 指定命名 Profile
bws r chrome@120 -p work

# 后台运行
bws r chrome@120 -d

# 传递原生参数
bws r chrome@120 -- --disable-gpu --no-sandbox

# 试运行
bws r chrome@120 --dry-run

# 使用代理
bws r chrome@120 --proxy socks5://127.0.0.1:1080

# 禁用代理（覆盖全局配置）
bws r chrome@120 --no-proxy

# 指纹隔离：随机生成指纹
bws r chrome@120 --fingerprint random

# 指纹隔离：标准防护
bws r chrome@120 --fingerprint standard

# 指纹隔离：自定义 JSON
bws r chrome@120 --fingerprint '{"userAgent":"...","language":"en-US","webrtc":"disabled"}'

# 使用 open 别名
bws open chrome@120
```
### 指纹隔离

`--fingerprint`（简写 `-fp`）选项为浏览器启动时添加指纹伪装，降低网站指纹识别的准确性。

**预设模式：**

| 预设 | 说明 |
|------|------|
| `standard` | 标准防护：禁用 WebRTC、使用虚拟媒体设备 |
| `random` | 随机指纹：每次生成随机的 User-Agent、语言、分辨率等组合 |
| `none` | 无指纹隔离（默认） |

**自定义配置：**

```bash
# 直接传入 JSON
bws r chrome@120 --fingerprint '{"userAgent":"...","language":"en-US","webrtc":"disabled","disableWebGL":true,"fakeMediaDevices":true,"windowWidth":1280,"windowHeight":720,"devicePixelRatio":1}'

# 从文件读取
bws r chrome@120 --fingerprint @./fingerprint.json
```

**JSON 配置字段：**

| 字段 | 类型 | 说明 |
|------|------|------|
| `preset` | string | 预设标识（`custom`） |
| `userAgent` | string | HTTP User-Agent 头 |
| `language` | string | 浏览器语言 |
| `windowWidth` | int | 窗口宽度 |
| `windowHeight` | int | 窗口高度 |
| `devicePixelRatio` | float | 设备像素比 |
| `webrtc` | string | WebRTC 策略：`disabled`/`proxied`/`default` |
| `disableWebGL` | bool | 禁用 WebGL |
| `disableCanvasRead` | bool | 禁用 Canvas 读取 |
| `fakeMediaDevices` | bool | 使用虚拟媒体设备 |

**浏览器实现差异：**

| 维度 | Chrome/Chromium | Firefox |
|------|:---:|:---:|
| User-Agent | `--user-agent` 命令行参数 | `general.useragent.override` 配置 |
| 语言 | `--lang` 命令行参数 | `intl.accept_languages` 配置 |
| 窗口大小 | `--window-size` 命令行参数 | RFP 自动管理 |
| DPR | `--force-device-scale-factor` | RFP 自动管理 |
| WebRTC | `--force-webrtc-ip-handling-policy` | `media.peerconnection.*` 配置 |
| 综合防护 | 命令行参数逐个控制 | `privacy.resistFingerprinting` 一键开启 |

> **注意**：Chrome 的命令行参数只能控制 HTTP 层和部分浏览器行为，**无法覆盖 JS 侧的 `navigator.userAgent`、`screen` 对象、Canvas/WebGL 渲染结果**。这些需要 Chrome DevTools Protocol 或浏览器扩展来注入 JS 脚本。Firefox 的 `resistFingerprinting` 则提供更全面的内置保护。

---

## bws install (别名: i)

安装浏览器版本。

### 用法

```bash
bws i <浏览器@版本> [选项]
bws i -d <目录> [浏览器@版本]
bws i --from-file <文件> [浏览器@版本]
```

### 参数

| 参数 | 说明 |
|------|------|
| `浏览器@版本` | 要安装的浏览器版本（支持 latest、beta、部分版本号等） |

### 选项

| 选项 | 简写 | 说明 |
|------|------|------|
| `--dir <path>` | `-d` | 从本地目录安装 |
| `--from-file <path>` | - | 从本地压缩包安装 |
| `--channel <渠道>` | - | 指定发布渠道 |
| `--force` | `-f` | 强制重新安装 |

### 示例

```bash
# 安装最新稳定版
bws i chrome@latest

# 安装指定渠道
bws i chrome@beta

# 安装指定完整版本
bws i chrome@120.0.6478.114

# 安装部分版本号
bws i chrome@85

# 从目录安装
bws i -d /path/to/browser-dir

# 从目录安装并指定版本
bws i -d /path/to/browser-dir chrome@120

# 从文件安装
bws i --from-file /path/to/chrome-setup.exe chrome@120

# 强制重新安装
bws i chrome@120 --force
```

---

## bws shortcut (别名: sc)

为已安装的浏览器创建、移除或列出桌面快捷方式。快捷方式直接指向浏览器可执行文件，双击即可启动浏览器。

### 用法

```bash
bws sc <子命令> [浏览器[@版本]] [选项]
```

### 子命令

| 子命令 | 别名 | 说明 |
|--------|------|------|
| `create` | `c`, `add` | 创建桌面快捷方式 |
| `remove` | `rm`, `del` | 移除桌面快捷方式 |
| `list` | `ls` | 列出已创建的快捷方式 |

### 参数

| 参数 | 说明 |
|------|------|
| `浏览器[@版本]` | 可选，指定浏览器和版本（支持 latest、stable 等别名） |

### 选项

| 选项 | 简写 | 说明 |
|------|------|------|
| `--profile <名称>` | `-p` | 指定 Profile 名称 |
| `--native` | `-n` | 原生模式（不使用 Profile） |
| `--all` | `-a` | 为所有已安装版本创建/移除 |
| `--name <名称>` | - | 自定义快捷方式名称 |

### 示例

```bash
# 为指定版本创建快捷方式
bws sc create chrome@120

# 使用特定 Profile 创建快捷方式
bws sc create firefox@latest --profile dev

# 为所有已安装版本创建快捷方式
bws sc create --all

# 移除快捷方式
bws sc remove chrome@120

# 移除所有快捷方式
bws sc remove --all

# 列出已创建的快捷方式
bws sc list
```

### 跨平台说明

| 平台 | 快捷方式类型 | 位置 |
|------|-------------|------|
| Windows | `.lnk` | 桌面 |
| Linux | `.desktop` | 桌面 + `~/.local/share/applications/` |
| macOS | `.app` bundle | 桌面 |

---

## bws import (别名: imp)

从目录批量导入浏览器版本（自动识别）。

### 用法

```bash
bws imp <目录> [选项]
```

### 参数

| 参数 | 说明 |
|------|------|
| `目录` | 包含浏览器安装包的目录路径 |

### 选项

| 选项 | 简写 | 说明 |
|------|------|------|
| `--force` | `-f` | 强制重新导入已安装的版本 |

### 示例

```bash
# 批量导入
bws imp /path/to/browsers

# 强制重新导入
bws imp /path/to/browsers -f
```

---

## bws uninstall (别名: rm, remove)

卸载指定的浏览器版本。

### 用法

```bash
bws rm <浏览器@版本>
```

### 参数

| 参数 | 说明 |
|------|------|
| `浏览器@版本` | 要卸载的浏览器版本（支持部分版本号） |

### 示例

```bash
# 卸载指定版本
bws rm chrome@120

# 卸载部分版本号匹配的最新版本
bws rm chrome@85
```

### 注意事项

- 卸载只删除程序文件，不删除 Profile 数据
- 系统安装的浏览器无法通过 bws 卸载

---

## bws use (别名: u)

设置默认浏览器版本。

### 用法

```bash
bws u <浏览器@版本>
```

### 参数

| 参数 | 说明 |
|------|------|
| `浏览器@版本` | 要设为默认的浏览器版本（支持部分版本号） |

### 示例

```bash
# 设置 Chrome 120 为默认版本
bws u chrome@120

# 使用短别名
bws u gc@120

# 设置后直接运行
bws r chrome
```

---

## bws download (别名: dl)

仅下载安装包，不安装。

### 用法

```bash
bws dl <浏览器@版本> [选项]
```

### 参数

| 参数 | 说明 |
|------|------|
| `浏览器@版本` | 要下载的浏览器版本 |

### 选项

| 选项 | 简写 | 说明 |
|------|------|------|
| `--output <目录>` | `-o` | 指定输出目录 |
| `--channel <渠道>` | `-c` | 指定发布渠道 |

### 示例

```bash
# 下载最新稳定版
bws dl chrome@latest

# 下载指定版本
bws dl chrome@120.0.6478.114

# 下载部分版本号
bws dl chrome@85

# 指定输出目录
bws dl chrome@latest -o ~/downloads

# 下载指定渠道
bws dl chrome@beta -c beta
```

---

## bws profile (别名: pf)

管理浏览器 Profile。

### 用法

```bash
bws pf <子命令> [参数] [选项]
```

### 子命令

| 子命令 | 说明 |
|--------|------|
| `list` | 列出所有 Profile |
| `path` | 查看 Profile 路径 |
| `reset` | 重置 Profile |
| `clean` | 清理孤立 Profile |

### profile list

```bash
# 列出所有 Profile
bws pf list

# 列出指定浏览器的 Profile
bws pf list chrome
```

### profile path

```bash
# 查看默认 Profile 路径
bws pf path chrome@120

# 查看命名 Profile 路径
bws pf path chrome myprofile
```

### profile reset

```bash
# 重置默认 Profile
bws pf reset chrome@120

# 重置命名 Profile
bws pf reset chrome@120 myprofile

# 跳过确认
bws pf reset chrome@120 -f
```

### profile clean

```bash
# 清理所有孤立 Profile
bws pf clean

# 清理指定浏览器的孤立 Profile
bws pf clean chrome

# 跳过确认
bws pf clean -f
```

---

## bws alias

管理版本别名。

### 用法

```bash
bws alias <子命令> [参数]
```

### 子命令

| 子命令 | 说明 |
|--------|------|
| `list` | 列出所有别名 |
| `add` | 添加别名 |
| `remove` | 删除别名 |

### 示例

```bash
# 列出所有别名
bws alias list

# 添加别名
bws alias add mychrome chrome@120.0.6099.109

# 删除别名
bws alias remove mychrome
```

---

## bws serve (别名: sv, server)

启动 HTTP 分发服务。配置通过 `bws-serve.ini` 文件管理，首次运行时会自动创建默认配置文件。

### 用法

```bash
bws sv [-d <目录>]
```

### 选项

| 选项 | 说明 |
|------|------|
| `-d, --dir` | 基础目录（包含 packages/ 和 bin/），默认程序所在目录 |

### 配置文件 (bws-serve.ini)

首次运行 `bws sv` 会自动创建配置文件，编辑后重新运行即可启动服务。

| 配置项 | 默认值 | 说明 |
|--------|--------|------|
| `host` | `0.0.0.0` | 监听主机地址 |
| `port` | `8080` | 监听端口 |
| `packages-dir` | 程序目录/packages | 浏览器安装包存放目录 |
| `bin-dir` | 程序目录/bin | 客户端二进制存放目录 |
| `sync` | `false` | 是否启用自动同步 |
| `sync-interval` | `24h` | 同步间隔（支持 30d、24h、30m 格式） |
| `sync-browsers` | 全部 | 同步的浏览器列表，逗号分隔 |
| `sync-channels` | `stable` | 同步的渠道列表，逗号分隔 |

### 示例

```bash
# 首次运行（自动创建配置文件）
bws sv
# 输出: 配置文件已创建: D:\bws\bws-serve.ini
# 编辑配置文件后重新运行

# 编辑配置后启动服务
bws sv

# 指定基础目录
bws sv -d D:\bws-data

# 使用 server 别名
bws server
```

### 后台运行

参见 [Serve 服务文档](/guide/serve#后台运行)，了解如何使用 systemd 或 nssm 配置为系统服务。

---

## bws config (别名: cfg)

管理配置。

### 用法

```bash
bws cfg <子命令> [参数]
```

### 子命令

| 子命令 | 说明 |
|--------|------|
| `show` | 查看所有配置 |
| `get <key>` | 获取指定配置项的值 |
| `set <key> <value>` | 设置指定配置项的值 |
| `path` | 显示配置文件路径 |

### 配置项

| 配置项 | 说明 | 默认值 |
|--------|------|--------|
| `data-dir` | 数据存储目录 | 空（便携模式） |
| `default-browser` | 默认浏览器 | `chrome` |
| `default-channel` | 默认渠道 | `stable` |
| `log-level` | 控制台日志级别 | `info` |
| `repo-path` | 本地仓库路径 | 空 |
| `source` | 离线源地址 | 空 |
| `source-serve` | Serve 源开关 | `true` |
| `source-omaha` | Omaha 源开关 | `true` |
| `source-firefox-ftp` | Firefox FTP 源开关 | `true` |
| `disk-threshold` | 磁盘空间告警阈值（GB） | `5` |
| `proxy` | 代理地址（用于下载和浏览器启动） | 空 |

### 示例

```bash
# 查看所有配置
bws cfg show

# 获取配置项
bws cfg get default-browser

# 设置配置项
bws cfg set default-browser firefox
bws cfg set log-level debug
bws cfg set source http://server:8080

# 设置代理
bws cfg set proxy socks5://127.0.0.1:1080
bws cfg set proxy http://proxy.example.com:8080

# 清除代理
bws cfg set proxy none

# 显示配置文件路径
bws cfg path
```

---

## bws repo

管理本地二进制仓库。

### 用法

```bash
bws repo <子命令> [参数]
```

### 子命令

| 子命令 | 说明 |
|--------|------|
| `path` | 显示当前仓库路径 |
| `set <路径>` | 设置仓库路径 |
| `scan` | 扫描仓库中的浏览器版本 |
| `import` | 从仓库导入浏览器版本（支持 `--force` / `-f` 强制重新安装） |

### 示例

```bash
# 查看当前仓库路径
bws repo path

# 设置仓库路径
bws repo set /path/to/repo

# 扫描仓库
bws repo scan

# 从仓库导入
bws repo import

# 强制重新导入
bws repo import -f
```

---

## bws cache (别名: cc)

管理下载缓存。下载文件存储在临时目录中，安装后会自动清理。

### 用法

```bash
bws cc <子命令>
```

### 子命令

| 子命令 | 说明 |
|--------|------|
| `clear` | 清除缓存的下载文件（提示：文件存储在临时目录中，会自动清理） |
| `info` | 显示缓存信息（类型：临时，自动清理） |

### 示例

```bash
# 查看缓存信息
bws cc info

# 清除缓存
bws cc clear
```

---

## bws plugin (别名: pl)

管理 bws 插件。插件是 Lua 脚本，可以在浏览器启动时自动修改参数或执行操作。

### 子命令

| 子命令 | 别名 | 说明 |
|--------|------|------|
| `list` | `ls`, `l` | 列出已安装的插件 |
| `install` | `i`, `add` | 安装插件（本地文件或远程 registry） |
| `uninstall` | `rm`, `remove` | 卸载插件 |
| `search` | `s`, `find` | 搜索远程插件 |

### 示例

```bash
# 列出已安装插件
bws plugin list

# 从本地文件安装
bws plugin install ./my-plugin.lua

# 从 registry 安装
bws plugin install fingerprint-enhanced

# 卸载
bws plugin uninstall fingerprint-enhanced

# 搜索
bws plugin search fingerprint
```

### 使用插件运行浏览器

```bash
# 启动时激活插件
bws r chrome@120 --plugin auto-arg

# 同时激活多个插件（逗号分隔）
bws r chrome@120 --plugin auto-arg,fingerprint-enhanced
```

### 编写插件

插件是 `.lua` 文件，放在 `bws-data/plugins/`（便携模式）或 `~/.bws/plugins/` 目录下。

**可用的 ctx API：**

| 函数/字段 | 说明 |
|-----------|------|
| `ctx.browser` | 浏览器名称（如 "chrome"、"firefox"） |
| `ctx.version` | 版本号 |
| `ctx.profile` | Profile 名称 |
| `ctx.profile_dir` | Profile 目录绝对路径 |
| `ctx.config(key)` | 读取 bws 配置项 |
| `ctx.add_arg(arg)` | 添加浏览器启动参数 |
| `ctx.set_env(key, value)` | 设置环境变量 |
| `ctx.write_file(path, content)` | 写入文件（返回 nil 成功，或错误字符串） |
| `ctx.read_file(path)` | 读取文件（返回 content, error） |
| `ctx.log(message)` | 输出日志到 stderr |

**插件可以定义 `pre_run()` 函数，在浏览器启动前被调用。**

---

## bws doctor (别名: dt)

系统健康检查。

### 用法

```bash
bws dt
```

### 检查内容

- 数据目录完整性
- 配置文件有效性
- 已安装版本完整性
- 磁盘空间检查
- 网络连通性（可选）

### 示例

```bash
bws dt
```

---

## bws help (别名: h)

显示帮助信息。

### 用法

```bash
bws help [命令]
bws h [命令]
```

### 示例

```bash
# 显示总帮助
bws help

# 显示指定命令的帮助
bws help r
bws help i

# 使用 h 别名
bws h ls
```