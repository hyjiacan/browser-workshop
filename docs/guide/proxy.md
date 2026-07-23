# 代理支持

bws 支持在两个层面使用代理：

1. **bws 网络代理**：bws 自身的网络请求（下载浏览器包、查询版本信息）走代理
2. **浏览器启动代理**：启动浏览器时通过参数传递代理配置

## 配置全局代理

通过 `bws cfg set proxy` 设置全局代理，影响 bws 下载和浏览器启动：

```bash
# 设置全局代理
bws cfg set proxy socks5://127.0.0.1:1080

# 清除代理（设为直连）
bws cfg set proxy none

# 查看当前代理
bws cfg get proxy
```

全局代理设置后，`bws i`（下载安装）和 `bws r`（启动浏览器）都会使用该代理。

## 启动时覆盖代理

使用 `run` 或 `use` 命令时，可以通过 `--proxy` 临时覆盖全局代理，或用 `--no-proxy` 强制禁用：

```bash
# 使用指定代理启动（覆盖全局配置）
bws r chrome@120 --proxy http://127.0.0.1:7890

# 强制禁用代理（忽略全局配置）
bws r chrome@120 --no-proxy

# 不指定则使用全局配置
bws r chrome@120
```

### 优先级

```
--no-proxy > --proxy <url> > 全局配置 > 直连
```

## 支持的协议

| 协议 | 格式示例 | 说明 |
|------|---------|------|
| HTTP | `http://127.0.0.1:7890` | 标准 HTTP 代理 |
| HTTPS | `https://127.0.0.1:7890` | HTTPS 代理 |
| SOCKS5 | `socks5://127.0.0.1:1080` | SOCKS5 代理（DNS 本地解析） |
| SOCKS5h | `socks5h://127.0.0.1:1080` | SOCKS5 代理（DNS 通过代理解析） |

### 各浏览器代理实现方式

| 浏览器 | 实现方式 | 说明 |
|--------|---------|------|
| Chrome / Chromium | `--proxy-server=<url>` 命令行参数 | 直接传递代理地址 |
| Firefox | `user.js` 配置文件写入 Profile 目录 | 通过 `network.proxy.*` 偏好设置 |

Firefox 的代理配置会写入 Profile 目录的 `user.js` 文件，包含以下设置：
- HTTP/HTTPS 代理：`network.proxy.http` + `network.proxy.ssl`
- SOCKS5 代理：`network.proxy.socks` + `network.proxy.socks_version=5` + `network.proxy.socks_remote_dns=true`
- `network.proxy.type = 1`（手动代理）

## 常见用法

### 使用本地代理工具

```bash
# 设置全局代理（Clash 默认端口）
bws cfg set proxy http://127.0.0.1:7890

# 下载 Chrome（通过代理）
bws i chrome@120

# 启动 Chrome（通过代理）
bws r chrome@120
```

### 仅浏览器走代理，下载不走

```bash
# 不设置全局代理
bws cfg set proxy none

# 下载直连
bws i chrome@120

# 启动时指定代理
bws r chrome@120 --proxy socks5://127.0.0.1:1080
```

### 使用 SOCKS5 并通过代理解析 DNS

```bash
bws cfg set proxy socks5h://127.0.0.1:1080
```

`socks5h` 协议会将 DNS 查询也通过代理服务器进行，避免 DNS 泄露。

## 注意事项

1. **TLS 证书**：bws 的下载请求默认跳过 TLS 证书验证（`InsecureSkipVerify: true`），以兼容自签名证书的 bws serve 实例。浏览器本身的证书验证不受此设置影响。
2. **代理认证**：支持在 URL 中包含用户名密码，如 `http://user:pass@proxy:8080`。
3. **配置持久化**：全局代理通过 `bws cfg set proxy` 设置，保存在 `config.json` 中，重启后生效。
