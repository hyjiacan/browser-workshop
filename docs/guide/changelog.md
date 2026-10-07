# 版本变更记录

本页面记录 Browser Workshop 各版本的功能变更，按版本号倒序排列。

## 未发布（2026-10-07 自动化框架集成）

> 将 bws 从「浏览器版本管理工具」扩展为「自动化测试框架的浏览器供应层」：暴露 CDP / WebDriver 端点与浏览器路径，并提供后台实例的完整生命周期管理。详见 [自动化框架集成](./automation.md)。

### 新增功能

#### CDP 端点暴露

- `run` 新增 `--cdp` 选项：注入 `--remote-debugging-port=0`、`--remote-debugging-address=127.0.0.1`、`--disable-blink-features=AutomationControlled`
- 从 Profile 目录的 `DevToolsActivePort` 文件发现 CDP WebSocket 端点，回退到 `GET /json/version`
- 用户显式传入的 `--remote-debugging-port` 等参数优先，不被覆盖

#### 后台实例生命周期

- `run` 新增 `--daemon` 选项：后台运行并登记到实例注册表（`bws-data/instances.json`）
- 新增 `bws ps`：列出运行中的后台实例（校验 PID 存活，自动清理僵尸条目并告警）
- 新增 `bws stop`（别名 `kill`）：停止指定实例或全部实例，一并停止关联的 chromedriver
- 新增 `bws endpoint`：输出实例的 CDP / WebDriver 端点
- 实例注册表采用文件锁 + 原子写入，支持并发安全

#### 浏览器路径输出

- 新增 `bws where`（别名 `path`）：输出浏览器二进制 / 安装目录 / Profile 路径，便于 Cypress 等框架做命令替换

#### 统一输出契约

- 自动化相关命令（`run --automation/--daemon`、`where`、`endpoint`、`ps`、`stop`、`ls`）统一支持 `--json`
- 统一信封 `{appname, version, timestamp, command, ok, data}`，失败时为 `{appname, version, timestamp, command, ok, error}`，错误码为语言中立的稳定标识
- stdout 只放内容，stderr 只放元信息（横幅、提示、告警）

#### run 自动化模式增强

- `--automation` 现为超集开关：同时启用 CDP 与 WebDriver 端点
- 新增 `--endpoint-timeout`（端点发现超时，默认 10 秒）、`--json`（JSON 输出）
- 端点相关能力失败不阻断浏览器启动，对应字段输出 `null`

### 行为变更

- 客户端版本号改用日期规则（`yyyy.MM.dd`，如 `2026.10.07`），不再使用 `1.0.0` 形式的语义化版本
- 发布构建由构建当天日期注入版本号，本地开发构建回退到构建日期，`--version`、JSON 信封的 `version` 字段与更新命令的版本比较均遵循该规则

## 未发布（2026-09-30 自动化驱动管理）

> 新增自动化驱动（chromedriver）管理能力，让 bws 管理的浏览器可被 Selenium、WebdriverIO 等自动化框架直接接入。

### 新增功能

#### 自动化驱动管理（driver）

- 新增 `bws driver` 命令（别名 `drv`），管理自动化驱动（chromedriver）
- 子命令：`list`/`ls`、`install`/`i`、`start`、`uninstall`/`rm`/`remove`
- 驱动版本按需解析：根据 Chrome 版本动态匹配对应的 chromedriver
  - Chrome ≥ 115：查询 Chrome for Testing 已知版本清单
  - Chrome < 115：回退到旧版 chromedriver storage 存储桶
- 驱动下载并解压到 `bws-data/drivers/chromedriver/<主版本>/`，元数据记录精确版本号
- 支持启动驱动进程并监听端口，输出 WebDriver 端点
- 命令：`driver`/`drv`

#### run 自动化模式

- `run` 命令新增 `--automation` 选项：启动浏览器的同时自动准备匹配版本的驱动
- 新增 `--webdriver`（仅启用 WebDriver 端点）、`--driver-port`（指定端口）、`--driver-no-download`（禁止自动下载）
- 驱动启动失败不会中断浏览器启动，仅在标准错误输出告警

#### serve 驱动托管与代理

- serve 新增驱动清单端点 `/api/v1/driver/manifest`，按 Chrome 版本解析驱动构建
- serve 新增驱动下载端点 `/api/v1/driver/download/{filename}`，本地托管优先、未命中时从上游代理下载并缓存
- 驱动索引持久化（`drivers-index.json`），重启后仍可代理；上游不可用时回退到本地索引
- 客户端配置了 serve 离线源时，优先通过 serve 解析并下载驱动

## v1.0.0-beta（2026-07-31 配置文件重构）

> 配置文件结构调整：INI 配置文件移至可执行文件目录。

- **重命名配置文件**：客户端配置文件 `config.ini` 重命名为 `bws-client.ini`，与 `bws-serve.ini` 命名风格统一
- **调整配置文件位置**：`bws-client.ini` 和 `bws-serve.ini` 从数据目录 `bws-data/` 移至与 bws 可执行文件同目录（二进制目录）
- 数据目录 `bws-data/` 现仅存储运行时数据（版本、缓存、日志、Profile 等），不再包含配置文件

## v1.0.0（2026-09-23 正式版）

> 首个正式稳定版本。所有核心功能已实现，并完成稳定性打磨与文档完善。

### 核心功能

#### 多版本浏览器管理

- 支持 Chrome、Firefox、Chromium 三种浏览器
- 同时安装和管理多个版本，版本之间完全隔离
- 命令：`list`/`ls`、`info`/`show`、`install`/`i`、`uninstall`/`rm`、`run`/`r`、`use`/`u`、`download`/`dl`

#### 本地导入

- 从目录或压缩包自动识别并导入浏览器版本
- 文件名智能识别，无需手动指定版本信息
- 支持批量导入：`bws install -d <directory>`
- 命令：`install -d`

#### 远程下载

- Firefox：通过 Mozilla Product Details API 获取版本信息
- 支持稳定版、Beta、Dev、Canary、ESR 等多个发布渠道
- 支持完整版本号或部分版本号匹配
- 命令：`download`/`dl`

#### 自动下载（run 未安装时）

- `run` 命令在目标版本未安装时，若远程源可用则自动下载并安装后再启动
- 解析顺序：本地已安装 → 远程源自动下载 → 安装指引（三方均无匹配时）
- 多版本匹配时支持交互式选择（终端）与自动选择（管道/脚本环境）

#### 隔离运行

- 每个版本使用独立的用户数据目录（Profile），互不干扰
- 支持命名 Profile、重置 Profile、清理孤立 Profile
- 同一命名 Profile 可在不同版本间共享
- 命令：`profile`/`pf`

#### 离线分发服务

- 内置 `serve` 命令，搭建局域网浏览器版本分发服务
- 支持自动同步（从在线源下载二进制包）、断点续传、校验和验证
- 提供 HTML 页面手动触发同步
- 支持定时同步（默认每天一次）
- 配置持久化到 `bws-serve.ini`，支持 `packages-dir` 和 `bin-dir` 独立路径配置
- 命令：`serve`/`sv`

#### 源优先级机制

- 离线源（serve 服务）优先，内置在线源兜底
- 按浏览器类型过滤源：查询特定浏览器时仅从支持该浏览器的源获取
- 数据源开关：`serve-source`、`firefox-ftp`，可独立启用/禁用

#### 配置管理

- 统一通过 `bws cfg` 命令管理所有配置
- 配置文件自动在数据目录创建（`config.ini`）
- 首次运行自动进入初始化向导：逐项引导设置数据目录、默认浏览器、离线源，每一步都附说明，方向键选择、回车确认（交互终端）
- 初始化完成后提示后续修改配置的命令（`bws config show` / `bws config set source` 等）
- 非交互环境（管道/脚本）自动回退到默认配置，不会中断
- `bws cfg get`（无参数）列出所有可读配置项及其别名
- `bws cfg set`（无参数或只有 key）列出所有可写配置项及示例值
- 配置项支持多别名：`language`→`lang`、`default-browser`→`browser` 等
- 命令：`config`/`cfg`

#### 别名系统

- 浏览器短别名：`gc`（chrome）、`ff`（firefox）、`cm`（chromium）
- 命令短别名：`r`（run）、`u`（use）、`dl`（download）、`cfg`（config）、`sv`（serve）、`cc`（cache）、`pf`（profile）、`dt`（doctor）、`sc`（shortcut）
- 浏览器多名称识别：`chrome`/`googlechrome`/`google-chrome` 等

#### 桌面快捷方式

- 创建、删除、列出桌面快捷方式
- 跨平台支持：Windows（`.lnk`）、Linux（`.desktop`）、macOS（`.app`）
- 命令：`shortcut`/`sc`

### 增强功能

#### 国际化（i18n）

- 内置中文和英文两种语言，通过 `bws cfg set language` 配置
- 支持外部翻译文件覆盖：在 `<数据目录>/i18n/<lang>.json` 中创建 JSON 文件即可覆盖内置翻译
- 自动检测系统语言（读取 `LANG`/`LANGUAGE` 环境变量），未设置时默认中文
- 提供语言模板文件 `template.json`，方便贡献者添加新语言

#### 命令拼写建议

- 输入不存在的命令时，自动检测相似命令并给出提示
- 基于 Levenshtein 编辑距离算法，支持前缀匹配加权和相邻字符交换检测
- 相似度低于 35% 时不展示建议，避免无效提示
- 示例：输入 `bws insall` 会提示 `你是不是想用 "install"? (相似度: 96%)`

#### 插件系统

- 支持 Lua 脚本插件（简单逻辑）和独立进程插件（复杂逻辑）两种类型
- 插件通过 Hook 机制注入核心流程：`pre-run`、`post-run`、`pre-install`、`post-install`、`on-exit`
- 插件市场通过 GitHub/Gitee 托管的 JSON 索引文件实现，无需自建服务器
- 支持三种安装方式：Registry 索引安装、Git 仓库直装、本地文件安装
- 插件下载时校验 SHA256 哈希，确保文件完整性
- 注册表缓存 24 小时，避免频繁下载
- 命令：`bws plugin list/install/uninstall/search`

#### 代理支持

- 全局代理配置：通过 `bws cfg set proxy <url>` 设置，用于下载浏览器包和查询版本源
- 浏览器启动代理：通过 `--proxy <url>` 指定或使用全局配置，`--no-proxy` 禁用
- 支持协议：HTTP、HTTPS、SOCKS5、SOCKS5h（DNS 通过代理解析）
- Chrome/Chromium 使用 `--proxy-server` 参数，Firefox 通过 `user.js` 写入 profile 目录

#### 指纹隔离

- 命令行参数层基础指纹隔离，通过 `--fingerprint` 参数触发
- 预设模式：`standard`（基础保护）、`random`（随机指纹）、`none`（不隔离）
- 随机指纹包含：User-Agent（Windows/Mac/Linux 各一套）、语言（7 种）、分辨率（8 种）、DPR
- WebRTC 随机禁用或代理，WebGL 50% 概率禁用，虚拟媒体设备始终启用
- 支持自定义 JSON 配置和文件加载

#### ESR 渠道支持

- `default-channel` 配置项支持 `esr` 可选值
- Firefox ESR 版本的查询、下载、安装全流程支持
- 版本号识别支持 `esr` 后缀（如 `115.6.0esr`）

### 其他功能

- **便携模式**：数据存储在程序同级 `bws-data/` 目录，可整体拷贝
- **日志系统**：分级日志，文件日志 DEBUG 级别，控制台日志 INFO 级别
- **系统集成**：自动识别系统已安装的浏览器版本
- **架构兼容**：自动检测架构兼容性，x64 可运行 x86 版本
- **磁盘检查**：下载前检查磁盘剩余空间
- **仓库管理**：本地二进制仓库扫描和管理
- **健康检查**：`bws doctor`/`dt` 系统健康检查
- **多格式压缩包**：支持 zip、7z、tar.gz、tar.bz2、tar.xz、.exe 等格式，魔术字节检测

### 支持的压缩格式

| 格式 | 说明 |
|------|------|
| `.zip` / `.jar` / `.apk` / `.war` | 原生 Go 支持 |
| `.7z` | bodgit/sevenzip 库 |
| `.tar.gz` / `.tar.bz2` / `.tar.xz` / `.tar.zst` | 对应压缩库 + tar |
| `.exe` | 自解压（zip 头检测） |

> 不支持：`.rar`、`.msi`、`.deb`、`.rpm`、`.iso`、`.wim`
