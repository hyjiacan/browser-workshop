# Proxy Support

bws supports proxies at two levels:

1. **bws network proxy**: bws's own network requests (downloading browser packages, querying version info) go through the proxy
2. **Browser launch proxy**: Proxy configuration is passed to the browser at launch

## Configure Global Proxy

Set a global proxy with `bws cfg set proxy`, affecting both bws downloads and browser launches:

```bash
# Set global proxy
bws cfg set proxy socks5://127.0.0.1:1080

# Clear proxy (direct connection)
bws cfg set proxy none

# View current proxy
bws cfg get proxy
```

After setting a global proxy, both `bws i` (download/install) and `bws r` (launch browser) will use it.

## Override Proxy at Launch

When using the `run` or `use` command, temporarily override the global proxy with `--proxy`, or force-disable with `--no-proxy`:

```bash
# Use a specific proxy (overrides global config)
bws r chrome@120 --proxy http://127.0.0.1:7890

# Force disable proxy (ignores global config)
bws r chrome@120 --no-proxy

# Without specifying, uses global config
bws r chrome@120
```

### Priority

```
--no-proxy > --proxy <url> > Global config > Direct connection
```

## Supported Protocols

| Protocol | Example | Description |
|----------|---------|-------------|
| HTTP | `http://127.0.0.1:7890` | Standard HTTP proxy |
| HTTPS | `https://127.0.0.1:7890` | HTTPS proxy |
| SOCKS5 | `socks5://127.0.0.1:1080` | SOCKS5 proxy (local DNS resolution) |
| SOCKS5h | `socks5h://127.0.0.1:1080` | SOCKS5 proxy (DNS resolved through proxy) |

### Per-Browser Implementation

| Browser | Method | Description |
|---------|--------|-------------|
| Chrome / Chromium | `--proxy-server=<url>` CLI argument | Proxy address passed directly |
| Firefox | `user.js` config written to Profile directory | Via `network.proxy.*` preference settings |

Firefox proxy config is written to the Profile directory's `user.js` file, including:
- HTTP/HTTPS proxy: `network.proxy.http` + `network.proxy.ssl`
- SOCKS5 proxy: `network.proxy.socks` + `network.proxy.socks_version=5` + `network.proxy.socks_remote_dns=true`
- `network.proxy.type = 1` (manual proxy)

## Common Usage

### Using a Local Proxy Tool

```bash
# Set global proxy (Clash default port)
bws cfg set proxy http://127.0.0.1:7890

# Download Chrome (through proxy)
bws i chrome@120

# Launch Chrome (through proxy)
bws r chrome@120
```

### Proxy for Browser Only, Direct Download

```bash
# Don't set global proxy
bws cfg set proxy none

# Download directly
bws i chrome@120

# Specify proxy at launch
bws r chrome@120 --proxy socks5://127.0.0.1:1080
```

### SOCKS5 with DNS through Proxy

```bash
bws cfg set proxy socks5h://127.0.0.1:1080
```

The `socks5h` protocol routes DNS queries through the proxy server, preventing DNS leaks.

## Notes

1. **TLS certificates**: bws download requests skip TLS certificate verification by default (`InsecureSkipVerify: true`) for compatibility with self-signed bws serve instances. The browser's own certificate verification is unaffected.
2. **Proxy authentication**: Supports including credentials in the URL, e.g. `http://user:pass@proxy:8080`.
3. **Persistent config**: Global proxy is set via `bws cfg set proxy`, saved in `config.json`, and persists across restarts.
