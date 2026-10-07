# 简介

Browser Workshop 是一款多版本浏览器管理工具，支持本地安装、远程下载、版本切换、隔离运行。无论你是前端开发者需要在不同浏览器版本中测试兼容性，还是安全研究员需要分析特定版本的浏览器，Browser Workshop 都能帮你轻松管理多个浏览器版本。

## 功能特性

### 多版本管理

同时安装和管理多个浏览器版本，版本之间完全隔离，互不干扰。内置支持 Chrome、Firefox、Chromium 三种浏览器。

### 本地安装

从目录或压缩包自动识别并安装浏览器版本。支持 zip、7z、tar.gz、tar.bz2、tar.xz 等多种压缩包格式。文件名智能识别，无需手动指定版本信息。

### 远程下载

从官方源（Firefox FTP）下载指定版本的浏览器。支持稳定版、Beta、Dev、Canary 等多个发布渠道，可按完整版本号或部分版本号下载。

### 离线分发

内置 `serve` 命令，可快速搭建局域网浏览器版本分发服务。支持自动同步、断点续传、校验和验证，适用于团队内部离线环境。

### 系统集成

自动识别系统已安装的浏览器版本，与手动安装的版本统一管理。

### 隔离运行

每个版本使用独立的用户数据目录（Profile），互不干扰。无需担心不同版本之间的配置冲突和数据污染。

### Profile 管理

支持命名 Profile、重置 Profile、清理孤立 Profile。同一个命名 Profile 可以在不同版本间共享，方便迁移和对比测试。

### 自动化测试集成

bws 不只是浏览器版本管理工具，也是自动化测试框架的浏览器供应层。一条命令即可启动版本精确、环境隔离的浏览器，并暴露标准端点，供 Playwright、Puppeteer、Selenium、WebdriverIO 等框架直接接入。

```bash
# 启动浏览器，自动准备匹配版本的 chromedriver，输出 CDP 与 WebDriver 端点
bws r chrome@120 --automation
```

- **CDP 端点**：从 Profile 目录的 `DevToolsActivePort` 发现 WebSocket 地址，供 Playwright / Puppeteer 连接
- **WebDriver 端点**：启动与浏览器主版本严格匹配的 chromedriver，供 Selenium / WebdriverIO 连接
- **浏览器路径输出**：`bws where chrome@120` 输出二进制路径，便于 Cypress 等框架做命令替换
- 端点能力失败不阻断浏览器启动，对应字段输出 `null`

### 自动化驱动管理

自动化驱动与浏览器版本强绑定，bws 按需解析、下载并启动匹配版本的驱动（chromedriver）。Chrome ≥ 115 查询 Chrome for Testing 清单，Chrome < 115 回退到旧版存储桶；配置离线源后还可经 serve 离线分发。

```bash
bws driver install chrome@120       # 安装匹配版本的驱动
bws driver start 120 --port 9515    # 启动驱动并监听端口
```

### 后台实例管理

自动化测试常需「启动一次、多次连接」。`run --daemon` 启动并登记实例，`ps` 查看运行中的实例，`stop` 停止实例，构成可脚本化的生命周期闭环。

```bash
bws r chrome@120 --automation --daemon --profile test-01
bws ps
bws stop bws-chrome-120-test-01
```

### 统一 JSON 输出

自动化相关命令（`run --automation/--daemon`、`where`、`endpoint`、`ps`、`stop`、`ls`）统一支持 `--json`，输出 `{appname, version, timestamp, command, ok, data}` 信封，便于脚本与 CI 消费。

```bash
CDP=$(bws r chrome@120 --automation --daemon --json | jq -r '.data.cdp')
```

### 多格式支持

支持 zip、7z、tar.gz、tar.bz2、tar.xz、.exe 等多种压缩包格式，无论是官方安装包还是绿色版压缩包都能轻松安装。

### 架构兼容

自动检测架构兼容性。x64 系统可运行 x86 版本，安装和列出时会自动过滤不兼容的架构。

### 便携模式

数据存储在 `bws-data/` 子目录中，与程序同级。整个程序可以连同数据一起拷贝到 U 盘或其他电脑上使用，真正做到即插即用。

### 日志系统

采用双输出分级日志系统，文件日志和控制台日志独立控制级别。支持日志轮转、`-V/--verbose` 临时调试模式。Serve 服务也有独立的 HTTP 请求日志。

### 源优先级

离线源优先，内置在线源兜底。配置离线源后，安装和查询会优先从离线源获取，找不到时自动回退到在线源。Serve 服务支持 `online-fallback`，本地缺失时自动从在线源实时下载。

### 浏览器短别名

支持 `gc`（chrome）、`ff`（firefox）、`cm`（chromium）等短别名，所有命令都可使用，大幅减少输入量。

### 多名称识别

浏览器支持多种名称输入，如 Chrome 可通过 `chrome`、`googlechrome`、`google-chrome` 等名称引用，无需记忆单一名称。

### 智能拼写建议

当输入的命令名不存在时，bws 会自动检测相似命令并进行提示，帮助你快速纠正输入错误。例如，输入 `bws imfo` 会提示 `你是不是想用 "info"? (相似度: 75%)`。

## 适用场景

### 前端兼容性测试

前端开发者需要在不同版本的 Chrome、Firefox 等浏览器中验证页面兼容性，bws 可以快速安装和切换多个版本。

### 安全研究与逆向分析

安全研究人员需要特定版本的浏览器进行漏洞分析和复现，bws 支持精确版本下载和隔离运行。

### 企业内网部署

企业内网无法访问外网时，可通过 `bws sv` 搭建离线分发服务，统一管理内部浏览器版本。

### 测试自动化

自动化测试需要在多个浏览器版本上运行测试用例，bws 可作为浏览器供应层直接接入测试框架：启动浏览器、输出端点，交给框架连接。

```bash
# 启动后台实例并查询端点
bws r chrome@120 --automation --daemon
bws endpoint bws-chrome-120

# 仅需浏览器路径时（如 Cypress）
cypress run --browser "$(bws where chrome@120)"
```

配合统一 JSON 输出，易于集成到 CI/CD 流程中。

### 多环境隔离

需要同时使用工作 Profile 和个人 Profile，或者需要干净的浏览器环境进行测试，bws 的 Profile 管理功能可以满足需求。
