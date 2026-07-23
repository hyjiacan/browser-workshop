# 浏览器指纹隔离

bws 内置浏览器指纹隔离功能，可以修改浏览器的各种特征参数，降低被网站追踪识别的风险。

## 快速使用

```bash
# 标准隐私保护（禁用 WebRTC、虚拟媒体设备）
bws r chrome@120 --fingerprint standard

# 随机指纹（每次启动生成不同特征组合）
bws r chrome@120 --fingerprint random

# 禁用指纹隔离
bws r chrome@120 --fingerprint none

# 自定义 JSON 配置
bws r chrome@120 --fingerprint '{"userAgent":"...","language":"en-US","windowWidth":1366,"windowHeight":768}'

# 从文件加载配置
bws r chrome@120 --fingerprint @/path/to/fingerprint.json

# 与代理、插件组合使用
bws r chrome@120 --fingerprint random --proxy socks5://127.0.0.1:1080 --plugin fingerprint-enhanced
```

## 预设模式

| 预设 | 说明 | 适用场景 |
|------|------|---------|
| `standard` | 基础隐私保护：禁用 WebRTC、启用虚拟媒体设备 | 日常浏览，防止 IP 泄露和设备指纹 |
| `random` | 随机生成完整指纹：UA、语言、分辨率、DPR、WebRTC、WebGL 等 | 需要最大程度匿名化 |
| `none` | 禁用指纹隔离（默认） | 不需要隐私保护时 |
| 自定义 | 通过 JSON 或文件指定各项参数 | 精确控制各项指纹参数 |

## 标准预设详情

`--fingerprint standard` 启用以下保护：

- **WebRTC 禁用**：完全关闭 WebRTC，防止本地 IP 泄露
- **虚拟媒体设备**：使用假的摄像头和麦克风设备，防止真实的媒体设备被识别
- **Firefox 额外保护**：启用 `privacy.resistFingerprinting`（RFP）

## 随机预设详情

`--fingerprint random` 每次启动时随机生成以下参数：

| 参数 | 随机范围 | 说明 |
|------|---------|------|
| User-Agent | 3 种（Windows/Mac/Linux 各一套 Chrome UA） | 随机选择平台和 UA |
| 语言 | 7 种（zh-CN, en-US, en-GB, ja-JP, ko-KR, de-DE, fr-FR） | 随机选择浏览器语言 |
| 窗口分辨率 | 8 种（1920x1080, 1366x768, 2560x1440 等） | 常见分辨率 |
| 设备像素比 | 1.0 / 2.0 | Mac 自动使用 2.0（Retina） |
| WebRTC | disabled / proxied | 随机选择 WebRTC 策略 |
| WebGL | 50% 概率禁用 | 随机决定是否禁用 WebGL |
| 虚拟媒体设备 | 始终启用 | 使用假的摄像头/麦克风 |

**随机数安全**：使用 `crypto/rand` 生成，不可预测。

**内部一致性**：随机值之间保持逻辑一致，例如选择 Mac UA 时自动搭配 2.0 DPR。

## 各浏览器实现方式

### Chrome / Chromium

通过命令行参数传递：

| 指纹参数 | Chrome 参数 |
|---------|------------|
| User-Agent | `--user-agent=<ua>` |
| 语言 | `--lang=<lang>` |
| 窗口大小 | `--window-size=<w>,<h>` |
| 设备像素比 | `--force-device-scale-factor=<dpr>` |
| WebRTC 禁用 | `--force-webrtc-ip-handling-policy=disable_non_proxied_udp` |
| WebRTC 代理 | `--force-webrtc-ip-handling-policy=default_public_interface_only` |
| WebGL 禁用 | `--disable-webgl` |
| Canvas 读取禁用 | `--disable-reading-from-canvas` |
| 虚拟媒体设备 | `--use-fake-device-for-media-stream` + `--use-fake-ui-for-media-stream` |

### Firefox

通过 `user.js` 配置文件写入 Profile 目录：

| 指纹参数 | Firefox 偏好设置 |
|---------|----------------|
| User-Agent | `user_pref("general.useragent.override", "<ua>")` |
| 语言 | `user_pref("intl.accept_languages", "<lang>")` |
| RFP | `user_pref("privacy.resistFingerprinting", true)` |
| WebRTC 禁用 | `user_pref("media.peerconnection.enabled", false)` |
| WebGL 禁用 | `user_pref("webgl.disabled", true)` |
| 地理位置 | `user_pref("geo.enabled", false)` |
| 电池 API | `user_pref("dom.battery.enabled", false)` |

## 自定义配置

### JSON 格式

```json
{
  "preset": "custom",
  "userAgent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) ...",
  "language": "en-US",
  "windowWidth": 1366,
  "windowHeight": 768,
  "devicePixelRatio": 1.0,
  "webrtc": "disabled",
  "disableWebGL": false,
  "disableCanvasRead": false,
  "fakeMediaDevices": true
}
```

### 参数说明

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `preset` | string | `"none"` | 预设模式（standard/random/none/custom） |
| `userAgent` | string | 空 | 自定义 User-Agent |
| `language` | string | 空 | 浏览器语言（如 `en-US`） |
| `windowWidth` | int | 0 | 窗口宽度（最小 320） |
| `windowHeight` | int | 0 | 窗口高度（最小 240） |
| `devicePixelRatio` | float | 0 | 设备像素比（范围 0.5-4.0） |
| `webrtc` | string | 空 | WebRTC 策略：disabled / proxied / default |
| `disableWebGL` | bool | false | 禁用 WebGL |
| `disableCanvasRead` | bool | false | 阻止 Canvas 读取（Chrome only） |
| `fakeMediaDevices` | bool | false | 使用虚拟摄像头/麦克风 |

## 与插件配合

指纹隔离的核心功能覆盖了最常用的指纹参数，高级需求可通过插件进一步增强：

```bash
# 核心 + 增强插件
bws r chrome@120 --fingerprint random --plugin fingerprint-enhanced

# 核心 + 工作空间切换
bws r chrome@120 --fingerprint standard --plugin workspace
```

`fingerprint-enhanced` 插件在核心基础上增加了额外的 WebRTC 防护和 Canvas 保护，详见 [插件系统文档](./plugin.md)。

## 注意事项

1. **User-Agent 局限性**：Chrome 的 `--user-agent` 仅修改 HTTP 请求头，不影响 JavaScript 中 `navigator.userAgent` 的返回值。如需完全伪装 JS UA，需使用插件。
2. **Firefox RFP**：Firefox 的 `privacy.resistFingerprinting` 功能非常强大，启用后会自动统一多项指纹参数，与 Chrome 的实现策略不同。
3. **user.js 重复写入保护**：Firefox 的 `user.js` 写入包含去重标记，不会在每次启动时重复追加内容。代理配置和指纹配置可以共存。
4. **指纹唯一性**：`random` 预设的随机池有限（3 UA x 7 语言 x 8 分辨率），在高频使用场景中可能出现重复指纹。
