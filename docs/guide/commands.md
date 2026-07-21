# 命令参考

本文档列出 bws 的所有命令及其详细说明，包括用途、用法、示例和参数。

> 版本信息通过全局标志 `--version` / `-v` 获取，例如 `bws --version` 或 `bws -v`。

## 命令总览

| 命令 | 说明 |
|------|------|
| `bws ls` | 列出已安装的浏览器版本 |
| `bws show` | 显示版本详细信息 |
| `bws r` / `bws open` | 运行指定版本的浏览器 |
| `bws i` | 安装浏览器版本 |
| `bws imp` | 从目录批量导入（自动识别） |
| `bws rm` | 卸载浏览器版本 |
| `bws u` | 设置默认浏览器版本 |
| `bws dl` | 仅下载不安装 |
| `bws pf` | 管理浏览器 Profile |
| `bws alias` | 管理版本别名 |
| `bws sv` / `bws server` | 启动 HTTP 分发服务 |
| `bws cfg` | 管理配置 |
| `bws repo` | 管理本地二进制仓库 |
| `bws cc` | 管理下载缓存 |
| `bws dt` | 系统健康检查 |
| `bws help` / `bws h` | 显示帮助信息 |

---

## bws ls

列出已安装的浏览器版本。

### 用法

```bash
bws ls [浏览器[@版本]] [选项]
```

### 参数

| 参数 | 说明 |
|------|------|
| `浏览器[@版本]` | 可选，按浏览器和版本前缀筛选 |

### 选项

| 选项 | 简写 | 说明 |
|------|------|------|
| `--remote` | `-R` | 列出远程可用版本 |
| `--all` | `-a` | 显示所有浏览器 |
| `--no-system` | - | 不显示系统浏览器 |
| `--channel <渠道>` | `-c` | 指定渠道（仅远程列表有效） |
| `--limit <数量>` | `-n` | 限制结果数量（默认 20，仅远程列表有效） |
| `--json` | - | 以 JSON 格式输出 |

### 示例

```bash
# 列出所有已安装版本
bws ls

# 只列出 Chrome
bws ls chrome

# 使用短别名
bws ls gc

# 按版本前缀筛选
bws ls chrome@79

# 列出远程可用版本
bws ls -R chrome

# 显示所有浏览器
bws ls -a

# 不显示系统浏览器
bws ls --no-system

# 列出指定渠道的远程版本
bws ls -R chrome -c beta

# 限制远程结果数量
bws ls -R chrome -n 5

# JSON 格式输出
bws ls --json
```

---

## bws show

显示指定版本的详细信息。

### 用法

```bash
bws show <浏览器@版本>
```

### 参数

| 参数 | 说明 |
|------|------|
| `浏览器@版本` | 要查看的浏览器版本（支持部分版本号） |

### 示例

```bash
# 查看指定版本详情
bws show chrome@120

# 查看完整版本
bws show chrome@120.0.6099.109

# 查看系统浏览器信息
bws show chrome@system

# 使用短别名
bws show ff@121
```

### 输出内容

- 浏览器名称和版本号
- 发布渠道
- 安装路径
- 架构信息
- Profile 路径
- 可执行文件路径
- 安装来源

---

## bws r / bws open

运行指定版本的浏览器。

### 用法

```bash
bws r [浏览器[@版本]] [URL] [选项] [-- 原生参数]
```

### 参数

| 参数 | 说明 |
|------|------|
| `浏览器[@版本]` | 要运行的浏览器版本，省略时使用默认浏览器和默认版本 |
| `URL` | 可选，启动时打开的网址 |

### 选项

| 选项 | 简写 | 说明 |
|------|------|------|
| `--headless` | `-H` | 无头模式 |
| `--incognito` | `-i` | 隐身/无痕模式 |
| `--new-window` | `-w` | 新窗口打开 |
| `--profile <name>` | `-p` | 指定命名 Profile |
| `--native` | - | 原生模式（使用系统 Profile） |
| `--detach` | `-d` | 后台运行（不等待进程） |
| `--dry-run` | - | 试运行（不实际启动） |
| `--` | - | 之后的参数原样传递给浏览器 |

### 示例

```bash
# 运行指定版本
bws r chrome@120

# 运行默认版本
bws r chrome

# 运行系统版本
bws r chrome@system

# 打开指定 URL
bws r chrome@120 https://example.com

# 无头模式
bws r chrome@120 -H

# 隐身模式
bws r chrome@120 -i

# 指定命名 Profile
bws r chrome@120 -p work

# 后台运行
bws r chrome@120 -d

# 传递原生参数
bws r chrome@120 -- --disable-gpu --no-sandbox

# 试运行
bws r chrome@120 --dry-run

# 使用 open 别名
bws open chrome@120
```

---

## bws i

安装浏览器版本。

### 用法

```bash
bws i <浏览器@版本> [选项]
bws i -d <目录> [浏览器@版本]
bws i --from-file <文件> [浏览器@版本]
```

### 参数

| 参数 | 说明 |
|------|------|
| `浏览器@版本` | 要安装的浏览器版本（支持 latest、beta、部分版本号等） |

### 选项

| 选项 | 简写 | 说明 |
|------|------|------|
| `--dir <path>` | `-d` | 从本地目录安装 |
| `--from-file <path>` | - | 从本地压缩包安装 |
| `--channel <渠道>` | - | 指定发布渠道 |
| `--force` | `-f` | 强制重新安装 |

### 示例

```bash
# 安装最新稳定版
bws i chrome@latest

# 安装指定渠道
bws i chrome@beta

# 安装指定完整版本
bws i chrome@120.0.6478.114

# 安装部分版本号
bws i chrome@85

# 从目录安装
bws i -d /path/to/browser-dir

# 从目录安装并指定版本
bws i -d /path/to/browser-dir chrome@120

# 从文件安装
bws i --from-file /path/to/chrome-setup.exe chrome@120

# 强制重新安装
bws i chrome@120 --force
```

---

## bws imp

从目录批量导入浏览器版本（自动识别）。

### 用法

```bash
bws imp <目录> [选项]
```

### 参数

| 参数 | 说明 |
|------|------|
| `目录` | 包含浏览器安装包的目录路径 |

### 选项

| 选项 | 简写 | 说明 |
|------|------|------|
| `--force` | `-f` | 强制重新导入已安装的版本 |

### 示例

```bash
# 批量导入
bws imp /path/to/browsers

# 强制重新导入
bws imp /path/to/browsers -f
```

---

## bws rm

卸载指定的浏览器版本。

### 用法

```bash
bws rm <浏览器@版本>
```

### 参数

| 参数 | 说明 |
|------|------|
| `浏览器@版本` | 要卸载的浏览器版本（支持部分版本号） |

### 示例

```bash
# 卸载指定版本
bws rm chrome@120

# 卸载部分版本号匹配的最新版本
bws rm chrome@85
```

### 注意事项

- 卸载只删除程序文件，不删除 Profile 数据
- 系统安装的浏览器无法通过 bws 卸载

---

## bws u

设置默认浏览器版本。

### 用法

```bash
bws u <浏览器@版本>
```

### 参数

| 参数 | 说明 |
|------|------|
| `浏览器@版本` | 要设为默认的浏览器版本（支持部分版本号） |

### 示例

```bash
# 设置 Chrome 120 为默认版本
bws u chrome@120

# 使用短别名
bws u gc@120

# 设置后直接运行
bws r chrome
```

---

## bws dl

仅下载安装包，不安装。

### 用法

```bash
bws dl <浏览器@版本> [选项]
```

### 参数

| 参数 | 说明 |
|------|------|
| `浏览器@版本` | 要下载的浏览器版本 |

### 选项

| 选项 | 简写 | 说明 |
|------|------|------|
| `--output <目录>` | `-o` | 指定输出目录 |
| `--channel <渠道>` | - | 指定发布渠道 |

### 示例

```bash
# 下载最新稳定版
bws dl chrome@latest

# 下载指定版本
bws dl chrome@120.0.6478.114

# 下载部分版本号
bws dl chrome@85

# 指定输出目录
bws dl chrome@latest -o ~/downloads

# 下载指定渠道
bws dl chrome@beta -c beta
```

---

## bws pf

管理浏览器 Profile。

### 用法

```bash
bws pf <子命令> [参数] [选项]
```

### 子命令

| 子命令 | 说明 |
|--------|------|
| `list` | 列出所有 Profile |
| `path` | 查看 Profile 路径 |
| `reset` | 重置 Profile |
| `clean` | 清理孤立 Profile |

### profile list

```bash
# 列出所有 Profile
bws pf list

# 列出指定浏览器的 Profile
bws pf list chrome
```

### profile path

```bash
# 查看默认 Profile 路径
bws pf path chrome@120

# 查看命名 Profile 路径
bws pf path chrome myprofile
```

### profile reset

```bash
# 重置默认 Profile
bws pf reset chrome@120

# 重置命名 Profile
bws pf reset chrome@120 myprofile

# 跳过确认
bws pf reset chrome@120 -f
```

### profile clean

```bash
# 清理所有孤立 Profile
bws pf clean

# 清理指定浏览器的孤立 Profile
bws pf clean chrome

# 跳过确认
bws pf clean -f
```

---

## bws alias

管理版本别名。

### 用法

```bash
bws alias <子命令> [参数]
```

### 子命令

| 子命令 | 说明 |
|--------|------|
| `list` | 列出所有别名 |
| `add` | 添加别名 |
| `remove` | 删除别名 |

### 示例

```bash
# 列出所有别名
bws alias list

# 添加别名
bws alias add mychrome chrome@120.0.6099.109

# 删除别名
bws alias remove mychrome
```

---

## bws sv / bws server

启动 HTTP 分发服务。配置通过 `bws-serve.ini` 文件管理，首次运行时会自动创建默认配置文件。

### 用法

```bash
bws sv [-d <目录>]
```

### 选项

| 选项 | 说明 |
|------|------|
| `-d, --dir` | 基础目录（包含 packages/ 和 bin/），默认程序所在目录 |

### 配置文件 (bws-serve.ini)

首次运行 `bws sv` 会自动创建配置文件，编辑后重新运行即可启动服务。

| 配置项 | 默认值 | 说明 |
|--------|--------|------|
| `host` | `0.0.0.0` | 监听主机地址 |
| `port` | `8080` | 监听端口 |
| `packages-dir` | 程序目录/packages | 浏览器安装包存放目录 |
| `bin-dir` | 程序目录/bin | 客户端二进制存放目录 |
| `sync` | `false` | 是否启用自动同步 |
| `sync-interval` | `24h` | 同步间隔（支持 30d、24h、30m 格式） |
| `sync-browsers` | 全部 | 同步的浏览器列表，逗号分隔 |
| `sync-channels` | `stable` | 同步的渠道列表，逗号分隔 |

### 示例

```bash
# 首次运行（自动创建配置文件）
bws sv
# 输出: 配置文件已创建: D:\bws\bws-serve.ini
# 编辑配置文件后重新运行

# 编辑配置后启动服务
bws sv

# 指定基础目录
bws sv -d D:\bws-data

# 使用 server 别名
bws server
```

### 后台运行

参见 [Serve 服务文档](/guide/serve#后台运行)，了解如何使用 systemd 或 nssm 配置为系统服务。

---

## bws cfg

管理配置。

### 用法

```bash
bws cfg <子命令> [参数]
```

### 子命令

| 子命令 | 说明 |
|--------|------|
| `show` | 查看所有配置 |
| `get <key>` | 获取指定配置项的值 |
| `set <key> <value>` | 设置指定配置项的值 |
| `path` | 显示配置文件路径 |

### 配置项

| 配置项 | 说明 | 默认值 |
|--------|------|--------|
| `default-browser` | 默认浏览器 | `chrome` |
| `default-channel` | 默认渠道 | `stable` |
| `log-level` | 控制台日志级别 | `info` |
| `data-dir` | 数据存储目录 | `bws-data/` |
| `repo-path` | 本地仓库路径 | 空 |
| `source` | 离线源地址 | 空 |

### 示例

```bash
# 查看所有配置
bws cfg show

# 获取配置项
bws cfg get default-browser

# 设置配置项
bws cfg set default-browser firefox
bws cfg set log-level debug
bws cfg set source http://server:8080

# 显示配置文件路径
bws cfg path
```

---

## bws repo

管理本地二进制仓库。

### 用法

```bash
bws repo <子命令> [参数]
```

### 子命令

| 子命令 | 说明 |
|--------|------|
| `path` | 显示当前仓库路径 |
| `set <路径>` | 设置仓库路径 |
| `scan` | 扫描仓库中的浏览器版本 |
| `import` | 从仓库导入浏览器版本（支持 `--force` / `-f` 强制重新安装） |

### 示例

```bash
# 查看当前仓库路径
bws repo path

# 设置仓库路径
bws repo set /path/to/repo

# 扫描仓库
bws repo scan

# 从仓库导入
bws repo import

# 强制重新导入
bws repo import -f
```

---

## bws cc

管理下载缓存。下载文件存储在临时目录中，安装后会自动清理。

### 用法

```bash
bws cc <子命令>
```

### 子命令

| 子命令 | 说明 |
|--------|------|
| `clear` | 清除缓存的下载文件（提示：文件存储在临时目录中，会自动清理） |
| `info` | 显示缓存信息（类型：临时，自动清理） |

### 示例

```bash
# 查看缓存信息
bws cc info

# 清除缓存
bws cc clear
```

---

## bws dt

系统健康检查。

### 用法

```bash
bws dt
```

### 检查内容

- 数据目录完整性
- 配置文件有效性
- 已安装版本完整性
- 磁盘空间检查
- 网络连通性（可选）

### 示例

```bash
bws dt
```

---

## bws help / bws h

显示帮助信息。

### 用法

```bash
bws help [命令]
bws h [命令]
```

### 示例

```bash
# 显示总帮助
bws help

# 显示指定命令的帮助
bws help r
bws help i

# 使用 h 别名
bws h ls
```