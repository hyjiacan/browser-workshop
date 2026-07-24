# 版本变更记录

本页面记录 Browser Workshop 各版本的功能变更，按版本号倒序排列。

## v1.0.0-beta（定版中）

> 当前版本，正在逐渐定版。所有核心功能已实现，正在进行稳定性打磨和文档完善。

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

- Chrome：通过 Chrome Omaha 协议查询和下载
- Firefox：通过 Mozilla Product Details API 获取版本信息
- 支持稳定版、Beta、Dev、Canary、ESR 等多个发布渠道
- 支持完整版本号或部分版本号匹配
- 命令：`download`/`dl`

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
- 数据源开关：`serve-source`、`omaha-source`、`firefox-ftp`，可独立启用/禁用

#### 配置管理

- 统一通过 `bws cfg` 命令管理所有配置
- 配置文件自动在数据目录创建（`config.json`）
- 首次运行引导设置数据存储目录
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
