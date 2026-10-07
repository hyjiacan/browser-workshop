# 自动化框架集成

bws 不只是「浏览器版本管理工具」，它同时是**自动化测试框架的浏览器供应层**：一条命令即可启动一个版本精确、环境隔离、并暴露标准端点的浏览器，供 Playwright、Puppeteer、Selenium、WebdriverIO、Cypress 等框架直接接入。

> bws 不实现自动化测试，bws 只提供「被自动化测试的浏览器」。

## 核心概念

| 概念 | 说明 |
|------|------|
| **CDP 端点** | Chrome DevTools Protocol 的 WebSocket 地址，供 Playwright / Puppeteer 连接 |
| **WebDriver 端点** | 与浏览器主版本严格匹配的 chromedriver HTTP 地址，供 Selenium / WebdriverIO 连接 |
| **实例（instance）** | 以 `run --daemon` 启动并登记到注册表的后台浏览器进程，可用 `ps` / `stop` 管理 |
| **统一输出契约** | 所有自动化相关命令都支持 `--json`，输出 `{ok, command, data}` 信封，便于脚本消费 |

## 一个开关：`--automation`

`--automation` 是自动化能力的超集开关，等价于同时开启 CDP 与 WebDriver 端点，并注入对自动化友好的启动参数：

```bash
bws r chrome@120 --automation
```

执行流程：

1. 解析本次启动的 Chrome 版本（如 `120.0.6099.109`）
2. 注入 `--remote-debugging-port=0`、`--remote-debugging-address=127.0.0.1`、`--disable-blink-features=AutomationControlled`
3. 启动浏览器，并从 Profile 目录的 `DevToolsActivePort` 文件（回退 `GET /json/version`）发现 CDP 端点
4. 解析并启动与该版本匹配的 chromedriver，得到 WebDriver 端点
5. 输出包含所有端点的启动信息

**子开关**（可单独使用，与 `--automation` 组合不报错）：

| 选项 | 说明 |
|------|------|
| `--cdp` | 仅启用 CDP 端点 |
| `--webdriver` | 仅启用 WebDriver 端点 |
| `--daemon` | 后台运行并登记为可管理实例 |
| `--driver-port <port>` | chromedriver 监听端口（`0` 表示自动分配） |
| `--driver-no-download` | 不自动下载驱动，缺失时仅告警 |
| `--endpoint-timeout <sec>` | 端点发现超时（秒，默认 10） |
| `--json` | 以 JSON 输出（需配合上述任一开关） |

### 用户参数优先

bws 不会覆盖你显式传入的参数。若你在 `--` 之后自行指定了 `--remote-debugging-port=9222`，bws 将保留该值而不再注入 `=0`：

```bash
bws r chrome@120 --automation -- --remote-debugging-port=9222
```

## 输出契约

### 非 JSON

```bash
bws r chrome@120 --automation --profile test-01
```

```
Instance:  bws-chrome-120-test-01
Browser:   chrome
Version:   120.0.6099.109
Binary:    C:\bws\bws-data\versions\chrome\120.0.6099.109\chrome.exe
Profile:   C:\bws\bws-data\runtime\chrome\120.0.6099.109\test-01
PID:       12345
CDP:       ws://127.0.0.1:54321/devtools/browser/xxxxx
WebDriver: http://127.0.0.1:9515
Daemon:    false
```

### JSON

```bash
bws r chrome@120 --automation --profile test-01 --json
```

```json
{
  "appname": "bws",
  "version": "2026.10.07",
  "timestamp": "2026-10-07T14:30:00+08:00",
  "command": "run chrome@120 --automation --profile test-01 --json",
  "ok": true,
  "data": {
    "instance": "bws-chrome-120-test-01",
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

> **统一信封**：所有命令共享同一外层结构 —— `appname`（程序名）、`version`（版本）、`timestamp`（RFC3339 时间）、`command`（完整调用，含选项与参数）、`ok`（成功与否），命令载荷统一收纳在 `data` 字段下，失败时改由 `error` 字段承载。

> **`--json` 输出纯净**：指定 `--json` 时不会输出任何横幅或初始化提示，stdout 只承载 JSON 信封，可直接管道给 `jq` 等解析器。

### 字段缺省行为

端点字段使用**指针类型**，不可用时输出 `null`（键始终存在，脚本无需判空）：

```json
{
  "appname": "bws",
  "version": "2026.10.07",
  "timestamp": "2026-10-07T14:31:00+08:00",
  "command": "run firefox@115 --json",
  "ok": true,
  "data": {
    "instance": "bws-firefox-115",
    "browser": "firefox",
    "version": "115.0",
    "pid": 12346,
    "binary": "...\\firefox.exe",
    "profile": null,
    "cdp": null,
    "webdriver": null,
    "daemon": false
  }
}
```

> **端点失败不阻断启动**：CDP 发现超时或驱动启动失败只会在 stderr 输出告警，浏览器照常启动，对应字段为 `null`。只有浏览器本身启动失败才返回非 0 退出码。

## 浏览器支持矩阵

| 浏览器 | CDP 端点 | WebDriver 端点 |
|--------|:---:|:---:|
| Chrome / Chromium | ✅ | ✅（托管 chromedriver） |
| Edge | ✅ | ⛔（暂不托管驱动） |
| Firefox | ⛔（CDP 不完整） | ⛔（暂不托管驱动） |

Firefox 的 CDP 支持不完整，因此按 WebDriver 路径处理；当前 bws 仅托管 Chrome/Chromium 的 chromedriver。

## 后台实例生命周期

自动化测试常需要「启动一次、多次连接」，因此 `run --daemon` / `ps` / `stop` 构成可脚本化的闭环。

### 启动后台实例

```bash
bws r chrome@120 --automation --daemon --profile test-01
```

`--daemon` 等价于「后台运行 + 登记注册表」，命令立即返回并输出端点信息。实例名规则为 `bws-<browser>-<主版本>[-<profile>]`，例如 `bws-chrome-120-test-01`。

### 查看运行中的实例

```bash
bws ps
```

```
NAME                     BROWSER   VERSION          PROFILE   PID     CDP
bws-chrome-120-test-01   chrome    120.0.6099.109   test-01   12345   ws://127.0.0.1:54321/…
```

```bash
bws ps --json
```

> `ps` 只列出**运行中的实例**（注册表中的进程），与 `ls`（磁盘上已安装的版本）互不重叠。查询时会校验 PID 存活，已退出的僵尸条目自动清理并告警到 stderr。

### 查询实例端点

```bash
bws endpoint bws-chrome-120-test-01
```

```
CDP:       ws://127.0.0.1:54321/devtools/browser/xxxxx
WebDriver: http://127.0.0.1:9515
```

### 停止实例

```bash
# 停止指定实例
bws stop bws-chrome-120-test-01

# 停止所有实例
bws stop --all
```

```
Stopped: bws-chrome-120-test-01
```

停止行为：先优雅终止、超时后强制终止；一并停止关联的 chromedriver；从注册表移除。**Profile 数据不会被删除**，可复用或手动清理。

## 框架接入

### Playwright

```javascript
const browser = await chromium.connectOverCDP(cdpUrl);
const context = browser.contexts()[0];
const page = context.pages()[0];
```

```bash
CDP=$(bws r chrome@120 --automation --daemon --json | jq -r '.data.cdp')
```

### Puppeteer

```javascript
const browser = await puppeteer.connect({ browserURL: webSocketDebuggerUrl });
```

### Selenium

```python
from selenium import webdriver
from selenium.webdriver.chrome.options import Options

options = Options()
options.debugger_address = "127.0.0.1:9222"   # 连接已有浏览器
driver = webdriver.Remote(command_executor="http://127.0.0.1:9515", options=options)
```

### WebdriverIO

```javascript
const browser = await remote({
  hostname: '127.0.0.1',
  port: 9515,
  capabilities: { browserName: 'chrome' }
});
```

### Cypress

Cypress 只需浏览器二进制路径：

```bash
cypress run --browser "$(bws where chrome@120)"
```

`bws where` 默认只输出二进制路径，便于命令替换：

| 选项 | 输出 |
|------|------|
| （默认） | 浏览器二进制路径 |
| `--dir` | 安装目录 |
| `--profile` | Profile 目录（可配合 `--profile-name`） |
| `--json` | `{browser, version, binary, dir, profile}` |

## 故障排查

| 现象 | 原因与处理 |
|------|-----------|
| `cdp` 为 `null` | 端点发现超时。适当增大 `--endpoint-timeout`；确认未被安全软件拦截本地回环端口 |
| `webdriver` 为 `null` | chromedriver 未安装或启动失败。查看 stderr 告警，或先用 `bws driver install chrome@120` 手动安装 |
| `stop` 报实例不存在 | 实例已被停止或进程已退出；`bws ps` 确认当前实例 |
| 同名实例冲突 | 同一 Profile 已有运行实例。更换 `--profile` 或先 `bws stop <实例名>` |

## 相关文档

- [run 命令的自动化模式](./commands.md#自动化模式)
- [driver 命令](./commands.md#bws-driver-别名-drv)
- [数据存储](./data-storage.md)（实例注册表 `instances.json`）