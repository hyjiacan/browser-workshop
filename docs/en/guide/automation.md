# Automation Framework Integration

bws is not just a browser version manager — it is also a **browser supply layer for automation testing frameworks**: a single command launches a version-exact, environment-isolated browser that exposes standard endpoints, ready to be attached by Playwright, Puppeteer, Selenium, WebdriverIO, Cypress and other frameworks.

> bws does not implement automated testing; bws only supplies "the browser under test".

## Core Concepts

| Concept | Description |
|---------|-------------|
| **CDP endpoint** | The Chrome DevTools Protocol WebSocket address for Playwright / Puppeteer |
| **WebDriver endpoint** | A chromedriver HTTP address strictly matching the browser major, for Selenium / WebdriverIO |
| **Instance** | A background browser process started with `run --daemon` and registered, manageable via `ps` / `stop` |
| **Unified output contract** | Every automation-related command supports `--json` and emits an `{ok, command, data}` envelope for scripting |

## One Switch: `--automation`

`--automation` is the superset switch for automation. It enables both the CDP and WebDriver endpoints and injects automation-friendly launch parameters:

```bash
bws r chrome@120 --automation
```

Flow:

1. Resolve the Chrome version for this launch (e.g. `120.0.6099.109`)
2. Inject `--remote-debugging-port=0`, `--remote-debugging-address=127.0.0.1`, `--disable-blink-features=AutomationControlled`
3. Launch the browser and discover the CDP endpoint from the `DevToolsActivePort` file in the profile directory (falling back to `GET /json/version`)
4. Resolve and start the chromedriver matching this version to obtain the WebDriver endpoint
5. Print the launch info with all endpoints

**Sub-switches** (usable on their own; combining them with `--automation` is not an error):

| Option | Description |
|--------|-------------|
| `--cdp` | Enable the CDP endpoint only |
| `--webdriver` | Enable the WebDriver endpoint only |
| `--daemon` | Run in the background and register a manageable instance |
| `--driver-port <port>` | chromedriver listen port (`0` picks a free port) |
| `--driver-no-download` | Do not auto-download the driver; warn only if missing |
| `--endpoint-timeout <sec>` | Endpoint discovery timeout in seconds (default 10) |
| `--json` | Emit JSON (requires one of the switches above) |

### User Parameters Win

bws never overrides parameters you pass explicitly. If you set `--remote-debugging-port=9222` after `--`, bws keeps that value instead of injecting `=0`:

```bash
bws r chrome@120 --automation -- --remote-debugging-port=9222
```

## Output Contract

### Non-JSON

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
  "ok": true,
  "command": "run",
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

> **Clean `--json` output**: when `--json` is set, no banner or setup hints are emitted — stdout carries only the JSON envelope, so it can be piped straight into `jq` and friends.

### Default Values for Missing Fields

Endpoint fields use **pointer types**, so they render as `null` when unavailable (the key always exists, no null-checking needed in scripts):

```json
{
  "ok": true,
  "command": "run",
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

> **Endpoint failures never block the launch**: a CDP discovery timeout or driver start failure only warns on stderr; the browser still launches and the corresponding field is `null`. Only a failure of the browser itself returns a non-zero exit code.

## Browser Support Matrix

| Browser | CDP endpoint | WebDriver endpoint |
|---------|:---:|:---:|
| Chrome / Chromium | ✅ | ✅ (managed chromedriver) |
| Edge | ✅ | ⛔ (driver not managed yet) |
| Firefox | ⛔ (incomplete CDP) | ⛔ (driver not managed yet) |

Firefox's CDP support is incomplete, so it goes down the WebDriver path; currently bws only manages chromedriver for Chrome/Chromium.

## Background Instance Lifecycle

Automation tests often need "start once, attach many times", so `run --daemon` / `ps` / `stop` form a scriptable loop.

### Start a Background Instance

```bash
bws r chrome@120 --automation --daemon --profile test-01
```

`--daemon` means "run in the background + register in the registry". The command returns immediately with the endpoint info. The instance name is `bws-<browser>-<major>[-<profile>]`, e.g. `bws-chrome-120-test-01`.

### List Running Instances

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

> `ps` lists only **running instances** (processes in the registry) and does not overlap with `ls` (versions installed on disk). It checks PID liveness on query; stale entries of exited processes are pruned automatically with a warning on stderr.

### Query Instance Endpoints

```bash
bws endpoint bws-chrome-120-test-01
```

```
CDP:       ws://127.0.0.1:54321/devtools/browser/xxxxx
WebDriver: http://127.0.0.1:9515
```

### Stop an Instance

```bash
# Stop a specific instance
bws stop bws-chrome-120-test-01

# Stop all instances
bws stop --all
```

```
Stopped: bws-chrome-120-test-01
```

Stop behavior: graceful termination first, forced kill after timeout; the associated chromedriver is stopped too; the entry is removed from the registry. **Profile data is not deleted** and can be reused or cleaned up manually.

## Framework Integration

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
options.debugger_address = "127.0.0.1:9222"   # attach to an existing browser
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

Cypress only needs the browser binary path:

```bash
cypress run --browser "$(bws where chrome@120)"
```

`bws where` prints only the binary path by default, which makes command substitution easy:

| Option | Output |
|--------|--------|
| (default) | Browser binary path |
| `--dir` | Installation directory |
| `--profile` | Profile directory (combine with `--profile-name`) |
| `--json` | `{browser, version, binary, dir, profile}` |

## Troubleshooting

| Symptom | Cause and fix |
|---------|---------------|
| `cdp` is `null` | Endpoint discovery timed out. Increase `--endpoint-timeout`; make sure no security software blocks the local loopback port |
| `webdriver` is `null` | chromedriver is not installed or failed to start. Check the stderr warning, or install it first with `bws driver install chrome@120` |
| `stop` reports a missing instance | The instance was already stopped or the process exited; confirm the current instances with `bws ps` |
| Duplicate instance name conflict | A running instance already uses the same profile. Change `--profile` or run `bws stop <name>` first |

## Related Docs

- [Automation mode of the run command](./commands.md#automation-mode)
- [driver command](./commands.md#bws-driver-alias-drv)
- [Data storage](./data-storage.md) (instance registry `instances.json`)