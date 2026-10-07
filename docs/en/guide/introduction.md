# Introduction

Browser Workshop is a multi-version browser management tool, supporting local installation, remote download, version switching, and isolated execution. Whether you are a front-end developer needing to test compatibility across different browser versions, or a security researcher analyzing specific browser versions, Browser Workshop helps you easily manage multiple browser versions.

## Features

### Multi-version Management

Install and manage multiple browser versions simultaneously, with complete isolation between versions and no interference. Built-in support for Chrome, Firefox, and Chromium.

### Local Installation

Automatically identify and install browser versions from directories or archives. Supports zip, 7z, tar.gz, tar.bz2, tar.xz, and more. Intelligent filename recognition eliminates the need to manually specify version information.

### Remote Download

Download specified browser versions from official sources (Firefox FTP). Supports multiple release channels including Stable, Beta, Dev, and Canary. Download by full or partial version number.

### Offline Distribution

Built-in `serve` command to quickly set up a LAN browser version distribution service. Supports automatic synchronization, resumable transfers, and checksum verification, suitable for offline environments within teams.

### System Integration

Automatically detect browser versions installed on the system and manage them together with manually installed versions.

### Isolated Execution

Each version uses an independent user data directory (Profile), with no interference between versions. No need to worry about configuration conflicts or data contamination between different versions.

### Profile Management

Supports named Profiles, Profile reset, and cleanup of orphaned Profiles. The same named Profile can be shared across different versions, facilitating migration and comparison testing.

### Automation Integration

bws is not just a browser version manager — it is also the browser supply layer for test automation frameworks. One command launches a version-precise, isolated browser and exposes standard endpoints for Playwright, Puppeteer, Selenium, and WebdriverIO to connect to directly.

```bash
# Launch the browser, prepare the matching chromedriver, and print CDP / WebDriver endpoints
bws r chrome@120 --automation
```

- **CDP endpoint**: discovers the WebSocket address from the `DevToolsActivePort` file in the Profile directory, for Playwright / Puppeteer
- **WebDriver endpoint**: starts the chromedriver that strictly matches the browser major version, for Selenium / WebdriverIO
- **Browser path output**: `bws where chrome@120` prints the binary path, convenient for command substitution in frameworks such as Cypress
- Endpoint failures never block the browser launch; the affected field is emitted as `null`

### Automation Drivers

Automation drivers are tightly coupled to the browser version. bws resolves, downloads, and starts the matching driver (chromedriver) on demand. Chrome ≥ 115 queries the Chrome for Testing manifest; Chrome < 115 falls back to the legacy storage bucket. With an offline source configured, drivers can also be distributed offline via serve.

```bash
bws driver install chrome@120       # install the matching driver
bws driver start 120 --port 9515    # start the driver and listen on a port
```

### Background Instance Management

Automation tests often need to "launch once, connect many times". `run --daemon` launches and registers an instance, `ps` lists running instances, and `stop` stops them — a scriptable lifecycle.

```bash
bws r chrome@120 --automation --daemon --profile test-01
bws ps
bws stop bws-chrome-120-test-01
```

### Unified JSON Output

All automation-related commands (`run --automation/--daemon`, `where`, `endpoint`, `ps`, `stop`, `ls`) support `--json`, emitting an `{appname, version, timestamp, command, ok, data}` envelope for scripts and CI to consume.

```bash
CDP=$(bws r chrome@120 --automation --daemon --json | jq -r '.data.cdp')
```

### Multi-format Support

Supports zip, 7z, tar.gz, tar.bz2, tar.xz, .exe, and more. Whether it's an official installer or a portable archive, it can be easily imported.

### Architecture Compatibility

Automatically detect architecture compatibility. x64 systems can run x86 versions, and incompatible architectures are automatically filtered during import and listing.

### Portable Mode

Data is stored in the `bws-data/` subdirectory at the same level as the program. The entire program along with its data can be copied to a USB drive or another computer, truly enabling plug-and-play.

### Logging System

Adopts a hierarchical logging system. File logs default to DEBUG level to fully record all operations, while console logs default to INFO level to provide concise user feedback.

### Source Priority

Offline sources take priority, with built-in online sources as fallback. After configuring an offline source, installation and queries will preferentially use the offline source, automatically falling back to the online source when not found.

### Browser Short Aliases

Supports short aliases such as `gc` (chrome), `ff` (firefox), `cm` (chromium), which can be used in all commands, significantly reducing typing.

### Multi-name Recognition

Browsers support multiple name inputs. For example, Chrome can be referenced via `chrome`, `googlechrome`, `google-chrome`, and more, eliminating the need to memorize a single name.

### Smart Typo Suggestion

When an entered command name does not exist, bws will automatically detect similar commands and provide a suggestion to help you quickly correct input errors. For example, entering `bws imfo` will prompt `Did you mean "info"? (similarity: 75%)`.

## Use Cases

### Front-end Compatibility Testing

Front-end developers need to verify page compatibility across different versions of Chrome, Firefox, and other browsers. bws can quickly install and switch between multiple versions.

### Security Research and Reverse Engineering

Security researchers need specific browser versions for vulnerability analysis and reproduction. bws supports precise version download and isolated execution.

### Enterprise Intranet Deployment

When enterprise intranets cannot access the external network, an offline distribution service can be set up via `bws sv` to uniformly manage internal browser versions.

### Test Automation

Automated testing needs to run test cases across multiple browser versions. bws can act as the browser supply layer, connecting directly to test frameworks: launch the browser, expose endpoints, and let the framework connect.

```bash
# Launch a background instance and query its endpoints
bws r chrome@120 --automation --daemon
bws endpoint bws-chrome-120

# When only the browser path is needed (e.g. Cypress)
cypress run --browser "$(bws where chrome@120)"
```

Combined with unified JSON output, it is easy to integrate into CI/CD workflows.

### Multi-environment Isolation

When you need to use both work and personal Profiles simultaneously, or need a clean browser environment for testing, bws's Profile management features can meet your needs.
