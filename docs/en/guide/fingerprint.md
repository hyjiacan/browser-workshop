# Browser Fingerprint Isolation

bws has built-in browser fingerprint isolation, modifying various browser characteristics to reduce the risk of being tracked and identified by websites.

## Quick Usage

```bash
# Standard privacy protection (disable WebRTC, fake media devices)
bws r chrome@120 --fingerprint standard

# Random fingerprint (different feature combination each launch)
bws r chrome@120 --fingerprint random

# Disable fingerprint isolation
bws r chrome@120 --fingerprint none

# Custom JSON config
bws r chrome@120 --fingerprint '{"userAgent":"...","language":"en-US","windowWidth":1366,"windowHeight":768}'

# Load config from file
bws r chrome@120 --fingerprint @/path/to/fingerprint.json

# Combine with proxy and plugins
bws r chrome@120 --fingerprint random --proxy socks5://127.0.0.1:1080 --plugin fingerprint-enhanced
```

## Preset Modes

| Preset | Description | Use Case |
|--------|-------------|----------|
| `standard` | Basic privacy: disable WebRTC, enable fake media devices | Daily browsing, prevent IP leaks and device fingerprinting |
| `random` | Random full fingerprint: UA, language, resolution, DPR, WebRTC, WebGL, etc. | Maximum anonymity needed |
| `none` | Disable fingerprint isolation (default) | When privacy protection is not needed |
| Custom | Specify parameters via JSON or file | Fine-grained control over each fingerprint parameter |

## Standard Preset Details

`--fingerprint standard` enables the following protections:

- **WebRTC disabled**: Completely turns off WebRTC to prevent local IP leaks
- **Fake media devices**: Uses fake camera and microphone devices to prevent real device identification
- **Firefox extras**: Enables `privacy.resistFingerprinting` (RFP)

## Random Preset Details

`--fingerprint random` randomly generates the following parameters on each launch:

| Parameter | Random Range | Description |
|-----------|------------|-------------|
| User-Agent | 3 options (Windows/Mac/Linux Chrome UAs) | Random platform and UA |
| Language | 7 options (zh-CN, en-US, en-GB, ja-JP, ko-KR, de-DE, fr-FR) | Random browser language |
| Window resolution | 8 options (1920x1080, 1366x768, 2560x1440, etc.) | Common resolutions |
| Device pixel ratio | 1.0 / 2.0 | Mac auto-uses 2.0 (Retina) |
| WebRTC | disabled / proxied | Random WebRTC policy |
| WebGL | 50% chance disabled | Random decision |
| Fake media devices | Always enabled | Fake camera/mic |

**Secure randomness**: Generated using `crypto/rand`, not predictable.

**Internal consistency**: Random values maintain logical consistency — e.g., Mac UA automatically pairs with 2.0 DPR.

## Per-Browser Implementation

### Chrome / Chromium

Passed via command-line arguments:

| Fingerprint Parameter | Chrome Argument |
|---------------------|----------------|
| User-Agent | `--user-agent=<ua>` |
| Language | `--lang=<lang>` |
| Window size | `--window-size=<w>,<h>` |
| Device pixel ratio | `--force-device-scale-factor=<dpr>` |
| WebRTC disabled | `--force-webrtc-ip-handling-policy=disable_non_proxied_udp` |
| WebRTC proxied | `--force-webrtc-ip-handling-policy=default_public_interface_only` |
| WebGL disabled | `--disable-webgl` |
| Canvas read disabled | `--disable-reading-from-canvas` |
| Fake media devices | `--use-fake-device-for-media-stream` + `--use-fake-ui-for-media-stream` |

### Firefox

Written to `user.js` config file in the Profile directory:

| Fingerprint Parameter | Firefox Preference |
|---------------------|-------------------|
| User-Agent | `user_pref("general.useragent.override", "<ua>")` |
| Language | `user_pref("intl.accept_languages", "<lang>")` |
| RFP | `user_pref("privacy.resistFingerprinting", true)` |
| WebRTC disabled | `user_pref("media.peerconnection.enabled", false)` |
| WebGL disabled | `user_pref("webgl.disabled", true)` |
| Geolocation | `user_pref("geo.enabled", false)` |
| Battery API | `user_pref("dom.battery.enabled", false)` |

## Custom Configuration

### JSON Format

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

### Parameter Reference

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `preset` | string | `"none"` | Preset mode (standard/random/none/custom) |
| `userAgent` | string | empty | Custom User-Agent |
| `language` | string | empty | Browser language (e.g. `en-US`) |
| `windowWidth` | int | 0 | Window width (min 320) |
| `windowHeight` | int | 0 | Window height (min 240) |
| `devicePixelRatio` | float | 0 | Device pixel ratio (range 0.5-4.0) |
| `webrtc` | string | empty | WebRTC policy: disabled / proxied / default |
| `disableWebGL` | bool | false | Disable WebGL |
| `disableCanvasRead` | bool | false | Block canvas readback (Chrome only) |
| `fakeMediaDevices` | bool | false | Use fake camera/microphone |

## Combining with Plugins

The core fingerprint isolation covers the most common fingerprint parameters. Advanced needs can be enhanced with plugins:

```bash
# Core + enhanced plugin
bws r chrome@120 --fingerprint random --plugin fingerprint-enhanced

# Core + workspace switching
bws r chrome@120 --fingerprint standard --plugin workspace
```

The `fingerprint-enhanced` plugin adds extra WebRTC protection and Canvas protection on top of the core. See the [Plugin System docs](./plugin.md) for details.

## Notes

1. **User-Agent limitation**: Chrome's `--user-agent` only modifies HTTP request headers; it does not affect the JavaScript `navigator.userAgent` return value. For full JS UA spoofing, use a plugin.
2. **Firefox RFP**: Firefox's `privacy.resistFingerprinting` is very powerful — when enabled, it automatically unifies multiple fingerprint parameters. This is a different strategy from Chrome's per-flag approach.
3. **user.js dedup**: Firefox `user.js` writes include dedup markers to prevent repeated appending on each launch. Proxy config and fingerprint config can coexist.
4. **Fingerprint uniqueness**: The `random` preset has a limited random pool (3 UAs x 7 languages x 8 resolutions), so repeated fingerprints may occur in high-frequency usage scenarios.
