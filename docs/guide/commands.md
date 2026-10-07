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
| `bws uninstall` / `bws rm` / `bws remove` | 卸载浏览器版本 |
| `bws use` / `bws u` | 设置默认浏览器版本 |
| `bws download` / `bws dl` | 仅下载不安装 |
| `bws profile` / `bws pf` | 管理浏览器 Profile |
| `bws alias` | 管理版本别名 |
| `bws update` / `bws upgrade` / `bws up` | 从离线源更新 bws 到最新版本 |
| `bws serve` / `bws sv` / `bws server` | 启动 HTTP 分发服务 |
| `bws config` / `bws cfg` | 管理配置 |
| `bws repo` | 管理本地二进制仓库 |
| `bws cache` / `bws cc` | 管理下载缓存 |
| `bws plugin` / `bws pl` | 插件管理 |
| `bws driver` / `bws drv` | 管理自动化驱动（chromedriver） |
| `bws where` / `bws path` | 输出浏览器的本地路径（供 Cypress 等框架使用） |
| `bws endpoint` | 输出指定实例的 CDP / WebDriver 端点 |
| `bws ps` | 列出运行中的后台实例 |
| `bws stop` / `bws kill` | 停止运行中的后台实例 |
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
| `--refresh` | - | 强制刷新远程源缓存（仅远程列表有效） |
| `--json` | - | 以 JSON 格式输出（本地模式；输出 `installed[]`） |

### 示例

> `bws list`（别名 `bws ls`）

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

# 强制刷新远程源缓存（跳过缓存，从网络获取最新数据）
bws ls -R firefox --refresh

# 以 JSON 格式输出
bws ls --json
```

JSON 输出结构：

```json
{
  "ok": true,
  "command": "ls",
  "data": {
    "installed": [
      { "browser": "chrome", "version": "120.0.6099.109", "type": "bws", "channel": "stable" },
      { "browser": "chrome", "version": "125.0.6422.112", "type": "system", "channel": "stable" }
    ]
  }
}
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

> `bws info`（别名 `bws show`）

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
bws r <浏览器[@版本]> [URL] [选项] [-- 原生参数]
```

### 参数

| 参数 | 说明 |
|------|------|
| `浏览器[@版本]` | 要运行的浏览器版本（必填），如 `chrome@120` |
| `URL` | 可选，启动时打开的网址 |

### 选项

| 选项 | 简写 | 说明 |
|------|------|------|
| `--headless` | `-H` | 无头模式 |
| `--incognito` | `-i` | 隐身/无痕模式 |
| `--new-window` | `-w` | 新窗口打开 |
| `--profile <name>` | `-p` | 指定命名 Profile |
| `--native` | `-n` | 原生模式（使用系统 Profile） |
| `--detached` | `-d` | 后台运行（不等待进程） |
| `--dry-run` | - | 试运行（不实际启动） |
| `--proxy <url>` | - | 代理地址（如 `socks5://127.0.0.1:1080`），留空使用全局配置 |
| `--no-proxy` | - | 禁用代理（覆盖全局配置） |
| `--fingerprint <preset>` | `-fp` | 指纹隔离预设（`standard`/`random`/`none`），或 JSON 配置/@文件路径 |
| `--plugin <names>` | - | 激活的插件（逗号分隔多个） |
| `--automation` | - | 自动化模式：注入 CDP 参数、管理匹配版本的驱动并输出端点 |
| `--cdp` | - | 仅启用 CDP 端点（`--automation` 的子集） |
| `--webdriver` | - | 仅启用 WebDriver 端点（`--automation` 的子集） |
| `--daemon` | - | 后台运行并登记为可管理实例（等价 `--detached` + 注册表） |
| `--driver-port <port>` | - | chromedriver 监听端口（`0` 表示自动分配） |
| `--driver-no-download` | - | 自动化模式下不自动下载驱动（缺失时仅告警） |
| `--endpoint-timeout <sec>` | - | 端点发现超时（秒，默认 10） |
| `--json` | - | 以 JSON 格式输出（需配合 `--automation`/`--cdp`/`--webdriver`/`--daemon`） |
| `--` | - | 之后的参数原样传递给浏览器 |

### 示例

> `bws run`（别名 `bws r`、`bws open`）

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

# 自动化模式：自动管理匹配版本的 chromedriver 并输出端点
bws r chrome@120 --automation

# 自动化模式并指定驱动端口
bws r chrome@120 --automation --driver-port 9515

# 仅启用 CDP 端点
bws r chrome@120 --cdp

# 仅启用 WebDriver 端点
bws r chrome@120 --webdriver

# 后台运行并登记为可管理实例
bws r chrome@120 --automation --daemon --profile test-01

# 自动化模式 + JSON 输出
bws r chrome@120 --automation --json

# 使用 open 别名
bws open chrome@120
```
### 自动化模式

`--automation` 是自动化能力的超集开关：启动浏览器的同时注入 CDP 参数、准备与该版本匹配的自动化驱动（chromedriver），并输出 CDP 与 WebDriver 端点，供 Playwright、Puppeteer、Selenium、WebdriverIO 等框架连接。

**执行流程：**

1. 解析本次启动的 Chrome 版本（如 `120.0.6099.109`）
2. 注入 `--remote-debugging-port=0`、`--remote-debugging-address=127.0.0.1`、`--disable-blink-features=AutomationControlled`
3. 启动浏览器，并从 Profile 目录的 `DevToolsActivePort` 文件（回退 `GET /json/version`）发现 CDP 端点
4. 查询 Chrome for Testing 清单，下载并解压 chromedriver 到 `bws-data/drivers/chromedriver/120/`
5. 启动 chromedriver 进程并监听端口，输出 WebDriver 端点

```bash
# 启动浏览器并自动准备 CDP 与驱动
bws r chrome@120 --automation --profile test-01
# 输出:
# Instance:  bws-chrome-120-test-01
# CDP:       ws://127.0.0.1:54321/devtools/browser/xxxxx
# WebDriver: http://127.0.0.1:9515
```

**JSON 输出契约：**

```bash
bws r chrome@120 --automation --json
```

```json
{
  "ok": true,
  "command": "run",
  "data": {
    "instance": "bws-chrome-120",
    "browser": "chrome",
    "version": "120.0.6099.109",
    "pid": 12345,
    "binary": "C:\\bws\\bws-data\\versions\\chrome\\120.0.6099.109\\chrome.exe",
    "profile": "C:\\bws\\bws-data\\runtime\\chrome\\120.0.6099.109\\test-01",
    "cdp": "ws://127.0.0.1:54321/devtools/browser/xxxxx",
    "webdriver": "http://127.0.0.1:9515",
    "daemon": false
  }
}
```

端点字段不可用时输出 `null`（键始终存在）。端点相关能力失败**不阻断浏览器启动**，仅向 stderr 告警。

**相关选项：**

| 选项 | 说明 |
|------|------|
| `--cdp` | `--automation` 的子集，仅启用 CDP 端点 |
| `--webdriver` | `--automation` 的子集，仅启用 WebDriver 端点 |
| `--daemon` | 后台运行并登记为可管理实例（配合 `ps` / `stop`） |
| `--driver-port <port>` | 指定驱动监听端口，`0` 表示自动分配 |
| `--driver-no-download` | 不自动下载驱动，缺失时仅告警 |
| `--endpoint-timeout <sec>` | 端点发现超时（秒，默认 10） |
| `--json` | 以 JSON 输出启动契约 |

**用户参数优先：** 若在 `--` 之后自行指定了 `--remote-debugging-port=9222`，bws 将保留该值而不再注入 `=0`。

**注意事项：**

- CDP 端点仅支持 Chrome/Chromium/Edge；WebDriver 目前仅托管 Chrome/Chromium 的 chromedriver
- 驱动启动失败或端点发现超时不会中断浏览器启动，仅在标准错误输出告警，对应字段为 `null`
- 配置了 serve 离线源时，将优先通过 serve 解析并下载驱动
- 驱动的解析、安装与卸载也可通过 [`bws driver`](#bws-driver-别名-drv) 命令单独管理

> 完整的自动化框架接入指南（Playwright/Puppeteer/Selenium/Cypress）请参考 [自动化框架集成](./automation.md)。

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
| `--from-dir <path>` | `-d` | 从本地目录安装 |
| `--from-file <path>` | - | 从本地压缩包安装 |
| `--channel <渠道>` | `-c` | 指定发布渠道 |
| `--force` | `-f` | 强制重新安装 |
| `--refresh` | - | 强制从 serve 重新下载（忽略本地缓存） |

### 示例

> `bws install`（别名 `bws i`）

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

# 强制从 serve 重新下载（忽略本地缓存）
bws i chrome@120 --refresh
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

> `bws shortcut`（别名 `bws sc`）

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

> `bws uninstall`（别名 `bws rm`、`bws remove`）

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

> `bws use`（别名 `bws u`）

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

> `bws download`（别名 `bws dl`）

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

### 示例

> `bws profile`（别名 `bws pf`）

### profile list

| 选项 | 简写 | 说明 |
|------|------|------|
| `--browser <name>` | `-b` | 指定浏览器 |

```bash
# 列出所有 Profile
bws pf list

# 列出指定浏览器的 Profile
bws pf list chrome
bws pf list --browser firefox
```

### profile path

```bash
# 查看默认浏览器 Profile 路径
bws pf path

# 查看指定浏览器的 Profile 路径
bws pf path chrome

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

> `bws alias`（无缩写别名）

```bash
# 列出所有别名
bws alias list

# 添加别名
bws alias add mychrome chrome@120.0.6099.109

# 删除别名
bws alias remove mychrome
```

---

## bws update (别名: upgrade, up)

从配置的离线源（serve）下载并更新 bws 到最新版本。

### 用法

```bash
bws update
```

### 说明

- 需要先通过 `bws cfg set source <url>` 配置离线源地址
- 自动从 `/api/v1/bin` 获取当前平台/架构的最新版本
- 版本相同时不进行任何操作
- 升级过程：下载新版本 → 备份旧版本 → 替换 → 清理备份

### 示例

> `bws update`（别名 `bws upgrade`、`bws up`）

```bash
# 配置离线源
bws cfg set source http://192.168.1.1:8080

# 检查并更新到最新版本
bws update
bws up
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
| `-d, --dir` | 基础目录（包含 packages/ 和 bin/），默认为程序所在目录 |

### 配置文件 (bws-serve.ini)

首次运行 `bws sv` 会自动在与 bws 可执行文件同目录下创建配置文件，编辑后重新运行即可启动服务。

| 配置项 | 默认值 | 说明 |
|--------|--------|------|
| `host` | `0.0.0.0` | 监听主机地址 |
| `port` | `8080` | 监听端口 |
| `packages-dir` | 程序目录/packages | 浏览器安装包存放目录（支持绝对/相对路径） |
| `bin-dir` | 程序目录/bin | 客户端二进制存放目录（支持绝对/相对路径） |
| `sync` | `false` | 是否启用自动同步 |
| `sync-interval` | `24h` | 同步间隔（支持 30d、24h、30m 格式） |
| `sync-browsers` | 全部 | 同步的浏览器列表，逗号分隔 |
| `sync-channels` | `stable` | 同步的渠道列表，逗号分隔 |
| `online-fallback` | `true` | 在线回退：本地未命中的包自动从在线源实时下载 |
| `scan-workers` | `0` | 并行扫描线程数，`0` 表示自动（CPU 核心数） |

### 示例

> `bws serve`（别名 `bws sv`、`bws server`）

```bash
# 首次运行（自动创建配置文件）
bws sv
# 输出: 配置文件已创建: bws-serve.ini
# 编辑配置文件后重新运行

# 编辑配置后启动服务
bws sv

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
| `language` | 界面语言（zh/en） | 自动检测 |
| `log-level` | 日志级别（debug/info/warn/error） | `info` |
| `repo-path` | 本地仓库路径 | 空 |
| `source` | 离线源地址 | 空 |
| `source-serve` | Serve 源开关 | `true` |
| `source-firefox-ftp` | Firefox 数据源开关 | `true` |
| `disk-threshold` | 磁盘空间告警阈值（GB） | `5` |
| `proxy` | 代理地址（用于下载和浏览器启动） | 空 |

### 示例

> `bws config`（别名 `bws cfg`）

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
bws cfg set proxy ""

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

> `bws repo`（无缩写别名）

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

管理下载缓存。远程安装时下载的安装包会永久缓存在本地（`cache/downloads/` 目录），后续安装相同版本时直接复用缓存文件，无需重复下载。当 serve 端文件更新时（文件大小变化），自动重新下载。

### 用法

```bash
bws cc <子命令>
```

### 子命令

| 子命令 | 说明 |
|--------|------|
| `clear` | 清除所有缓存的下载文件（需确认） |
| `info` | 显示缓存信息（目录、文件数、总大小、文件列表） |

### 示例

> `bws cache`（别名 `bws cc`）

```bash
# 查看缓存信息
bws cc info

# 清除缓存
bws cc clear
```

---

## bws plugin (别名: pl)

管理 bws 插件。插件可以修改浏览器启动参数或执行操作，支持两种类型：

- **Lua 脚本**（`.lua`）：简单逻辑，如修改启动参数、写配置文件
- **IPC 插件**（可执行文件）：通过 stdin/stdout JSON-RPC 通信，可用任何语言编写

### 子命令

| 子命令 | 别名 | 说明 |
|--------|------|------|
| `list` | `ls`, `l` | 列出已安装的插件 |
| `install` | `i`, `add` | 安装插件（本地文件或远程 registry） |
| `uninstall` | `rm`, `remove`, `del` | 卸载插件 |
| `update` | `up`, `u` | 更新插件到最新版本 |
| `search` | `s`, `find` | 搜索远程插件 |

### 示例

> `bws plugin`（别名 `bws pl`）

```bash
# 列出已安装插件
bws plugin list

# 从本地 Lua 文件安装
bws plugin install ./my-plugin.lua

# 从本地 IPC 插件安装（任意可执行文件）
bws plugin install ./my-plugin.py

# 从 registry 安装
bws plugin install fingerprint-enhanced

# 更新插件（仅 registry 来源的插件支持）
bws plugin update fingerprint-enhanced

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

**Lua 插件**是 `.lua` 文件，放在 `bws-data/plugins/`（便携模式）或 `~/.bws/plugins/` 目录下。

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

**IPC 插件**是任意可执行文件，通过 stdin/stdout JSON-RPC 通信：

- **请求**（stdin）：`{"event":"pre_run","browser":"chrome","version":"120","profile":"default","profileDir":"..."}`
- **响应**（stdout）：`{"extraArgs":["--flag"],"env":{"KEY":"val"},"error":""}`
- 超时：10 秒后自动终止进程
- 详见 `plugins/README.md` 和 `plugins/examples/browser-alias.py`

**插件可以定义 `pre_run()` 函数，在浏览器启动前被调用。**

---

## bws driver (别名: drv)

管理自动化驱动（目前为 chromedriver）。

自动化驱动的版本与浏览器版本强绑定：一个 chromedriver 只能驱动主版本相同的 Chrome/Chromium。因此 bws 不内置固定版本，而是按需解析、下载并启动匹配版本的驱动。

### 数据源

| Chrome 版本 | 数据源 |
|-------------|--------|
| `>= 115` | Chrome for Testing 已知版本清单（`known-good-versions-with-downloads.json`） |
| `< 115` | 旧版 chromedriver storage 存储桶 |

> 配置了 serve 离线源（`bws cfg set source <url>`）时，bws 会优先通过 serve 解析并下载驱动，便于内网/离线环境分发。

### 用法

```bash
bws driver <子命令> [参数] [选项]
```

### 子命令

| 子命令 | 别名 | 说明 |
|--------|------|------|
| `list` | `ls` | 列出已安装的驱动 |
| `install` | `i` | 下载并安装匹配指定 Chrome 版本的驱动 |
| `start` | - | 启动已安装的驱动并监听端口 |
| `uninstall` | `rm`, `remove` | 卸载指定主版本的驱动 |

### 示例

> `bws driver`（别名 `bws drv`）

```bash
# 列出已安装的驱动
bws driver ls

# 安装匹配 Chrome 120 的驱动
bws driver install chrome@120

# 仅解析并显示下载信息
bws driver install 120 --dry-run

# 强制重新安装
bws driver install chrome@120 --force

# 启动驱动（自动分配端口）
bws driver start 120

# 启动驱动并指定端口
bws driver start chrome@120 --port 9515

# 后台启动驱动
bws driver start 120 --detach

# 卸载驱动
bws driver uninstall 120
```

### driver install 选项

| 选项 | 简写 | 说明 |
|------|------|------|
| `--force` | `-f` | 强制重新安装 |
| `--dry-run` | - | 仅解析并显示下载信息，不实际安装 |

### driver start 选项

| 选项 | 简写 | 说明 |
|------|------|------|
| `--port <port>` | `-p` | 监听端口（`0` 表示自动分配） |
| `--detach` | `-d` | 后台运行（不等待进程结束） |
| `--allowed-ips <ips>` | - | 允许连接的 IP（默认 `127.0.0.1`） |
| `--log <file>` | - | 驱动日志文件路径 |

### driver uninstall 选项

| 选项 | 简写 | 说明 |
|------|------|------|
| `--force` | `-f` | 跳过确认直接卸载 |

### 目录结构

驱动安装在数据目录下的 `drivers/chromedriver/<主版本>/`：

```
bws-data/drivers/chromedriver/120/
```

每个主版本目录保存该主版本对应的驱动可执行文件及元数据（`.bws-driver.json`），元数据记录精确版本号、平台、安装时间等信息。

> 与 `bws r --automation` 配合使用可一步完成“启动浏览器 + 准备驱动”，详见 [run 命令的自动化模式](#自动化模式)。

---

## bws where (别名: path)

输出浏览器的本地路径，主要用于命令替换场景，例如让 Cypress 使用 bws 管理的浏览器：

```bash
cypress run --browser "$(bws where chrome@120)"
```

### 用法

```bash
bws where <浏览器@版本> [选项]
```

### 参数

| 参数 | 说明 |
|------|------|
| `浏览器[@版本]` | 必填，如 `chrome@120` |

### 选项

| 选项 | 说明 |
|------|------|
| `--dir` | 输出安装目录而非二进制路径 |
| `--profile` | 输出 Profile 目录 |
| `--profile-name <name>` | 指定 Profile 名称（与 `--profile` 配合） |
| `--json` | 以 JSON 格式输出 |

### 示例

```bash
# 默认输出二进制路径
bws where chrome@120

# 输出安装目录
bws where chrome@120 --dir

# 输出 Profile 目录
bws where chrome@120 --profile --profile-name test-01

# JSON 输出
bws where chrome@120 --json
```

JSON 输出结构：

```json
{
  "ok": true,
  "command": "where",
  "data": {
    "browser": "chrome",
    "version": "120.0.6099.109",
    "binary": "C:\\bws\\bws-data\\versions\\chrome\\120.0.6099.109\\chrome.exe",
    "dir": "C:\\bws\\bws-data\\versions\\chrome\\120.0.6099.109",
    "profile": "C:\\bws\\bws-data\\runtime\\chrome\\120.0.6099.109\\test-01"
  }
}
```

---

## bws endpoint

输出指定后台实例的 CDP / WebDriver 端点。

### 用法

```bash
bws endpoint <实例名> [选项]
```

### 选项

| 选项 | 说明 |
|------|------|
| `--json` | 以 JSON 格式输出 |

### 示例

```bash
bws endpoint bws-chrome-120-test-01
bws endpoint bws-chrome-120 --json
```

```
CDP:       ws://127.0.0.1:54321/devtools/browser/xxxxx
WebDriver: http://127.0.0.1:9515
```

> 实例名可由 [`bws ps`](#bws-ps) 查看，通过 `bws r ... --daemon` 创建。

---

## bws ps

列出运行中的后台实例（由 `bws r --daemon` 登记）。

### 用法

```bash
bws ps [选项]
```

### 选项

| 选项 | 说明 |
|------|------|
| `--json` | 以 JSON 格式输出 |

### 示例

```bash
bws ps
bws ps --json
```

```
NAME                     BROWSER   VERSION          PROFILE   PID     CDP
bws-chrome-120-test-01   chrome    120.0.6099.109   test-01   12345   ws://127.0.0.1:54321/…
```

- 无实例时输出 `No running instances.`
- 查询时校验 PID 存活，已退出的僵尸条目自动清理并告警到 stderr
- 表格中的 CDP 会截断显示，完整值请用 `--json` 或 [`bws endpoint`](#bws-endpoint)

> `ps` 与 [`ls`](#bws-list-别名-ls) 互不重叠：`ls` 列出磁盘上已安装的版本，`ps` 列出内存/注册表中运行中的进程。

---

## bws stop (别名: kill)

停止运行中的后台实例。

### 用法

```bash
bws stop <实例名> [选项]
bws stop --all [选项]
```

### 选项

| 选项 | 简写 | 说明 |
|------|------|------|
| `--all` | `-a` | 停止所有实例 |
| `--json` | - | 以 JSON 格式输出 |

### 示例

```bash
# 停止指定实例
bws stop bws-chrome-120-test-01

# 停止所有实例
bws stop --all

# JSON 输出
bws stop --all --json
```

### 行为说明

| 行为 | 说明 |
|------|------|
| 终止进程 | 先尝试优雅退出，超时后强制终止 |
| 停止驱动 | 若实例关联了 chromedriver，一并停止 |
| 清理注册表 | 从实例注册表移除 |
| 保留 Profile | Profile 数据不删除，可复用或手动清理 |

- 实例不存在时报错，退出码非 0
- 进程已退出但注册表未清理时，输出 `Stopped: <name> (already exited)`
- JSON 输出包含 `stopped` 与 `failed` 两个数组

---

## 统一输出契约

所有自动化相关命令（`run --automation/--daemon`、`where`、`endpoint`、`ps`、`stop`、`ls`）都支持 `--json`，并遵循统一信封：

```json
{
  "ok": true,
  "command": "<命令名>",
  "data": { }
}
```

失败时：

```json
{
  "ok": false,
  "command": "<命令名>",
  "error": {
    "code": "NOT_FOUND",
    "message": "实例不存在: bws-chrome-120"
  }
}
```

约定：

- **stdout 只放内容，stderr 只放元信息**（启动横幅、提示、告警），便于管道消费
- 描述性字段使用稳定枚举值（如 `type: system|bws`、`status: running`），中文仅用于人类可读输出
- 端点等可缺省字段始终存在，不可用时为 `null`
- 错误码为语言中立的稳定标识：`NOT_FOUND`、`INVALID_ARGUMENT`、`UNSUPPORTED`、`CONFLICT`、`INTERNAL`

---

## bws doctor (别名: dt)

系统健康检查。

### 用法

```bash
bws dt
```

### 检查内容

- 目录结构完整性
- 配置文件有效性
- 浏览器描述符数量
- 已安装版本完整性
- 系统浏览器检测
- 远程版本查询可用性
- 下载管理器可用性

### 示例

> `bws doctor`（别名 `bws dt`）

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

### 智能拼写建议

当输入的命令名不存在时，bws 会自动检测相似命令并进行提示。例如，输入 `bws imfo` 会提示：

```
你是不是想用 "info"? (相似度: 75%)
```

### 示例

> `bws help`（别名 `bws h`）

```bash
# 显示总帮助
bws help

# 显示指定命令的帮助
bws help r
bws help i

# 使用 h 别名
bws h ls
```