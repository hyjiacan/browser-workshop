# Browser Workshop

<p align="center">
  <img src="https://gitee.com/hyjiacan/browser-workshop/raw/master/logo.png" alt="Browser Workshop logo" width="128" />
</p>

<p align="center">
  浏览器多版本隔离运行工具，快速搭建独立测试环境，自由切换版本，并为自动化测试提供浏览器供应层。
</p>

## 功能特性

- **多版本管理**：同时安装和管理多个浏览器版本，版本间完全隔离，互不干扰
- **灵活的版本来源**：本地目录或压缩包自动识别导入，也可从官方源远程下载，并自动识别系统已安装的版本
- **隔离运行**：每个版本使用独立的 Profile，支持命名 Profile 在不同版本间共享
- **离线分发**：内置 `serve`，一键搭建局域网分发服务，内网也能获取浏览器与驱动
- **自动化集成**：一条命令启动浏览器并暴露 CDP / WebDriver 端点，自动准备与版本匹配的 chromedriver
- **后台实例管理**：`run --daemon` 启动、`ps` 查看、`stop` 停止，构成可脚本化的生命周期闭环

## 快速开始

```bash
# 查看已安装版本
bws ls

# 安装指定版本
bws i chrome@120

# 从本地目录或压缩包导入
bws i -d /path/to/browsers

# 运行浏览器
bws r chrome@120

# 自动化：启动浏览器并暴露 CDP / WebDriver 端点
bws r chrome@120 --automation
```

> Chrome 历史版本没有官方远程源，可手动下载后导入：`bws i --from-file chrome-120-win64.zip chrome@120`。

## 文档

完整文档请访问：**[Browser Workshop 文档站](https://hyjiacan.github.io/browser-workshop)**

- [快速上手](https://hyjiacan.github.io/browser-workshop/guide/getting-started)
- [命令参考](https://hyjiacan.github.io/browser-workshop/guide/commands)
- [自动化框架集成](https://hyjiacan.github.io/browser-workshop/guide/automation)
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
| `bws update` | `upgrade`, `up` | 从配置的离线源更新 bws |
| `bws serve` | `sv`, `server` | 启动 HTTP 分发服务 |
| `bws config` | `cfg` | 管理配置 |
| `bws repo` | — | 管理本地二进制仓库 |
| `bws cache` | `cc` | 管理下载缓存 |
| `bws plugin` | `pl` | 插件管理 |
| `bws driver` | `drv` | 管理自动化驱动（chromedriver） |
| `bws where` | `path` | 输出浏览器的本地路径 |
| `bws endpoint` | — | 输出实例的 CDP / WebDriver 端点 |
| `bws ps` | — | 列出运行中的后台实例 |
| `bws stop` | `kill` | 停止运行中的后台实例 |
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

## 自动化集成

bws 可作为自动化测试框架的浏览器供应层：一条命令启动版本精确、环境隔离的浏览器，并暴露标准端点。

```bash
# 启动浏览器，自动准备匹配版本的 chromedriver，输出 CDP / WebDriver 端点
bws r chrome@120 --automation

# 后台运行并登记为可管理实例
bws r chrome@120 --automation --daemon --profile test-01

# 查看并停止后台实例
bws ps
bws stop bws-chrome-120-test-01

# 仅需要浏览器路径时（如 Cypress）
bws where chrome@120
```

`--automation` 是超集开关，等价于同时启用 `--cdp` 与 `--webdriver`。端点相关能力失败不会阻断浏览器启动，对应字段输出 `null`。详见 [自动化框架集成](https://hyjiacan.github.io/browser-workshop/guide/automation)。

## Serve 服务

在内网或团队环境搭建浏览器分发服务，客户端无需外网即可获取浏览器版本与驱动。

```bash
# 首次运行自动生成配置文件，编辑后再次启动即可
bws sv
```

支持并行扫描、在线回退（本地缺失时从在线源实时下载）与驱动托管。详见 [Serve 服务文档](https://hyjiacan.github.io/browser-workshop/guide/serve)。

## 许可证

MIT