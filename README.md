# Browser Workshop

<p align="center">
  <img src="https://gitee.com/hyjiacan/browser-workshop/raw/master/logo.png" alt="Browser Workshop logo" width="128" />
</p>

<p align="center">
  多版本浏览器管理工具，支持本地导入、远程下载、版本切换、隔离运行。
</p>

## 功能特性

- **多版本管理**：同时安装和管理多个浏览器版本，支持版本前缀快速筛选
- **本地导入**：从目录或压缩包自动识别并导入，支持 zip、7z、tar.gz、tar.bz2、tar.xz、exe 等多种格式
- **远程下载**：从官方源下载指定版本（Firefox 通过 Mozilla FTP）
- **离线分发**：内置 `serve` 命令，支持自动同步、在线回退、并行扫描，搭建局域网分发服务
- **隔离运行**：每个版本独立 Profile，支持命名 Profile
- **插件系统**：Lua 脚本插件和独立进程插件，可在启动时自动修改参数、注入配置、执行自定义逻辑
- **代理支持**：全局代理配置，覆盖下载和浏览器启动，支持 HTTP/SOCKS5/SOCKS5H 协议
- **指纹隔离**：`--fingerprint` 预设（standard/random），降低浏览器指纹被追踪的风险
- **详细日志**：分级日志系统，文件与控制独立配置级别，支持日志轮转
- **国际化**：内置中文和英文，支持外部翻译文件覆盖
- **便携模式**：数据存储在 `bws-data/` 子目录，U 盘随身携带
- **浏览器短别名**：`gc` (chrome)、`ff` (firefox)、`cm` (chromium)
- **HTTPS 兼容**：默认跳过证书验证，适配内网/自签名证书环境

## 快速开始

```bash
# 查看已安装版本
bws ls
bws ls gc@79           # 使用短别名 + 版本前缀筛选

# 从本地目录批量导入
bws import /path/to/browsers

# 远程下载安装
bws i chrome@120

# Chrome 历史版本需手动下载后导入
# 下载地址: https://chromedownloads.net/
bws i --from-file chrome-120-win64.zip chrome@120

# 运行浏览器
bws r chrome@120
bws r gc@120 -i        # 隐身模式
```

## 文档

完整文档请访问：**[Browser Workshop 文档站](https://hyjiacan.github.io/browser-workshop)**

- [快速上手](https://hyjiacan.github.io/browser-workshop/guide/getting-started)
- [命令参考](https://hyjiacan.github.io/browser-workshop/guide/commands)
- [Serve 服务](https://hyjiacan.github.io/browser-workshop/guide/serve)
- [浏览器短别名](https://hyjiacan.github.io/browser-workshop/guide/short-aliases)

## 安装

```bash
go install github.com/hyjiacan/browser-workshop/cmd/bws@latest
```

或从 [Releases](https://github.com/hyjiacan/browser-workshop/releases) 下载预编译二进制。

国内用户也可以通过 Gitee 安装：

```bash
go install gitee.com/hyjiacan/browser-workshop/cmd/bws@latest
```

## 命令一览

| 命令 | 别名 | 说明 |
|------|------|------|
| `bws list` | `ls` | 列出已安装的浏览器版本 |
| `bws info` | `show` | 显示版本详细信息 |
| `bws run` | `r`, `open` | 运行指定版本的浏览器 |
| `bws install` | `i` | 安装浏览器版本 |
| `bws shortcut` | `sc` | 管理桌面快捷方式 |
| `bws uninstall` | `rm`, `remove` | 卸载浏览器版本 |
| `bws use` | `u` | 设置默认浏览器版本 |
| `bws download` | `dl` | 仅下载不安装 |
| `bws profile` | `pf` | 管理浏览器 Profile |
| `bws alias` | — | 管理版本别名 |
| `bws serve` | `sv`, `server` | 启动 HTTP 分发服务 |
| `bws config` | `cfg` | 管理配置 |
| `bws repo` | — | 管理本地二进制仓库 |
| `bws cache` | `cc` | 管理下载缓存 |
| `bws plugin` | `pl` | 插件管理 |
| `bws doctor` | `dt` | 系统健康检查 |
| `bws help` | `h` | 显示帮助信息 |

完整命令说明请查看 [命令参考](https://hyjiacan.github.io/browser-workshop/guide/commands)。

## 浏览器短别名

| 短别名 | 完整名称 |
|--------|----------|
| `gc` | chrome / googlechrome |
| `ff` | firefox |
| `cm` | chromium |

所有命令都支持短别名。详见 [浏览器短别名](https://hyjiacan.github.io/browser-workshop/guide/short-aliases)。

## 代理与指纹隔离

```bash
# 设置全局代理（下载和浏览器启动均生效）
bws cfg set proxy socks5://127.0.0.1:1080

# 运行时使用指纹隔离
bws r chrome@120 --fingerprint random
bws r chrome@120 --fingerprint standard
```

## 插件系统

bws 支持 Lua 脚本插件，在浏览器启动时自动执行自定义逻辑。

```bash
# 安装插件
bws plugin install ./my-plugin.lua

# 使用插件运行浏览器
bws r chrome@120 --plugin my-plugin

# 同时激活多个插件
bws r chrome@120 --plugin plugin-a,plugin-b

# 搜索远程插件
bws plugin search fingerprint
```

编写插件非常简单，创建一个 `.lua` 文件即可：

```lua
-- ~/.bws/plugins/my-plugin.lua
function pre_run()
    if ctx.browser == "chrome" then
        ctx.add_arg("--disable-background-timer-throttling")
    end
end
```

更多示例见 [plugins/examples](plugins/examples/)。

## Serve 服务

```bash
# 首次运行（自动创建配置文件）
bws sv
# 编辑 bws-serve.ini 配置文件

# 启动服务
bws sv
```

Serve 服务特性：
- **并行扫描**：启动时多线程并行计算文件校验和
- **扩展名过滤**：仅处理支持的安装包格式，跳过无关文件
- **在线回退**：本地缺失时自动从在线源下载（需启用 `online-fallback`）
- **详细日志**：启动时显示加载配置、扫描包等详细进度
- **双日志输出**：控制台 + 文件（`logs/serve.log`），支持日志轮转

详见 [Serve 服务文档](https://hyjiacan.github.io/browser-workshop/guide/serve)。

## 配置格式

客户端配置使用 INI 格式，存储在数据目录下的 `config.ini` 中：

```ini
[client]
default-browser = chrome
default-channel = stable
language = zh

[log]
console-level = info
file-level = debug
max-size-mb = 10
max-backups = 5

[source-switches]
enable-serve-source = true
enable-firefox-ftp = true
```

使用 `bws cfg` 命令管理配置，通常无需手动编辑文件。

## 许可证

MIT
