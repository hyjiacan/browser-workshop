# 数据存储

bws 将所有运行时数据（版本、缓存、日志、Profile 等）统一存储在数据目录中，配置文件（`bws-client.ini` 和 `bws-serve.ini`）位于与 bws 可执行文件同目录下。本章介绍数据存储的目录结构和各目录的用途。

## 便携模式（默认）

默认情况下，bws 采用便携模式，数据存储在程序同级的 `bws-data/` 目录中。

### 目录结构

```
bws/
├── bws.exe                    # 程序主文件
├── bws-client.ini             # 客户端配置文件（INI 格式）
├── bws-serve.ini              # serve 服务配置文件（首次运行 serve 时创建）
└── bws-data/                  # 数据根目录
    ├── .serve-cache.json      # serve 校验和缓存
    ├── instances.json         # 后台实例注册表（run --daemon / ps / stop）
    ├── logs/                  # 日志目录
    │   ├── bws.log            # bws 日志文件（客户端与 serve 共用）
    │   └── <实例名>.log       # 后台实例的浏览器输出日志（如 bws-chrome-120-test-01.log）
    ├── cache/                 # 下载缓存
    │   ├── manifests/         # 版本清单缓存
    │   │   └── firefox-ftp-cache.json  # Firefox FTP 源缓存
    │   └── downloads/         # 下载文件缓存
    ├── versions/              # 安装的浏览器版本
    │   ├── chrome/
    │   │   ├── 126.0.6478.114/
    │   │   ├── 121.0.6167.85/
    │   │   └── 79.0.3945.79/
    │   ├── firefox/
    │   │   └── 121.0/
    │   └── ...
    ├── drivers/               # 自动化驱动（chromedriver）
    │   └── chromedriver/
    │       ├── 120/           # 主版本 120 对应的驱动
    │       └── 121/
    └── runtime/               # 运行时数据
        └── chrome/
            ├── 126.0.6478.114/
            │   └── profile/   # 版本默认 Profile
            ├── 121.0.6167.85/
            │   └── profile/   # 版本默认 Profile
            └── profiles/
                ├── work/      # 命名 Profile "work"
                └── test/      # 命名 Profile "test"
```

### 便携模式的优势

- **即插即用**：整个目录拷贝到其他机器即可使用
- **数据集中**：所有数据都在一个目录下，便于管理和备份
- **不污染系统**：不向系统目录写入任何数据
- **适合 U 盘**：可以放在 U 盘随身携带

## 各目录说明

### bws-client.ini

配置文件，存储所有用户配置项。INI 格式。

```ini
[client]
default-browser = chrome
default-channel = stable

[log]
console-level = info
file-level = debug
max-size-mb = 10
max-backups = 5

[source-switches]
enable-serve-source = true
enable-firefox-ftp = true
```

通常不需要手动编辑，使用 `bws cfg` 命令管理。

如果需要将数据存储到其他位置，可以通过配置命令设置自定义数据目录：

```bash
bws cfg set data-dir D:\browser-data
```

### bws-serve.ini

Serve 服务的配置文件，首次运行 `bws sv` 时自动在与 bws 可执行文件同目录下创建。

```ini
[serve]
host = 0.0.0.0
port = 8080
packages-dir =
bin-dir =
sync = false
sync-interval = 24h
sync-browsers =
sync-channels = stable
online-fallback = false
scan-workers = 0
```

### logs/

日志目录，存储 bws 的运行日志。

- `bws.log`：bws 日志文件（与客户端共用），记录所有操作和 HTTP 请求等信息
- `<实例名>.log`：以 `run --daemon` 启动的后台实例的浏览器标准输出/错误日志
- 文件日志默认 DEBUG 级别，详细记录所有操作
- 日志会自动轮转，防止单个文件过大

更多日志相关信息请参考 [日志系统](./logging.md) 章节。

### instances.json

后台实例注册表，记录通过 [`bws r --daemon`](./commands.md#bws-run-别名-r-open) 启动的实例，是 [`bws ps`](./commands.md#bws-ps) 与 [`bws stop`](./commands.md#bws-stop-别名-kill) 的唯一数据来源。

- 每个实例记录名称、浏览器、版本、PID、Profile 路径、CDP / WebDriver 端点等信息
- 写入采用文件锁 + 原子替换，避免并发写入损坏
- 查询时校验 PID 存活，已退出的僵尸条目会自动清理并告警到 stderr
- 停止实例会从注册表中移除对应条目；**删除该文件等价于清空实例登记**（不会终止已运行的进程）
- 该文件由 bws 自动维护，不建议手动编辑

### cache/

缓存目录，存储下载的安装包和清单缓存。

#### cache/manifests/

版本清单缓存，存储从远程源获取的版本列表，避免每次都重新请求。

- 加速 `ls --remote` 等命令的响应
- 有过期时间，过期后自动重新获取
- Firefox FTP 源缓存存储为 `firefox-ftp-cache.json`，默认 24 小时有效期
- 可以通过 `bws ls -R --refresh` 强制刷新缓存
- 可以通过 `bws cc clear` 清理

#### cache/downloads/

下载文件缓存，永久存储通过 `install` 命令从 serve 远程下载的安装包。

- 安装完成后文件保留在缓存中，下次安装相同版本时直接复用
- 当 serve 端文件更新（大小变化）时自动重新下载
- 使用 `--refresh` 参数可强制从 serve 重新下载（忽略本地缓存）
- 占用空间可能较大，可定期通过 `bws cc clear` 清理
- 通过 `bws cc info` 查看缓存文件列表和占用空间

### versions/

已安装的浏览器版本目录，按浏览器名称和版本号分层存储。

```
versions/
├── chrome/
│   ├── 126.0.6478.114/     # Chrome 126 版本文件
│   ├── 121.0.6167.85/      # Chrome 121 版本文件
│   └── 79.0.3945.79/       # Chrome 79 版本文件
├── firefox/
│   └── 121.0/              # Firefox 121 版本文件
└── chromium/
    └── ...
```

每个版本目录包含完整的浏览器程序文件。卸载时会删除对应的版本目录。

### drivers/

自动化驱动目录，存储 bws 按需下载的自动化驱动（目前为 chromedriver）。

```
drivers/
└── chromedriver/
    ├── 120/                # 主版本 120 对应的驱动
    │   ├── chromedriver.exe
    │   └── .bws-driver.json
    └── 121/
```

- 按驱动的**主版本**分层存储，一个 chromedriver 只能驱动主版本相同的 Chrome/Chromium
- 每个主版本目录下的 `.bws-driver.json` 记录精确版本号、平台、架构、安装时间等元数据
- 通过 [`bws driver`](./commands.md#bws-driver-别名-drv) 命令或 `bws r --automation` 自动管理
- 卸载驱动会删除对应主版本目录

### runtime/

运行时数据目录，存储浏览器运行时产生的数据，主要是 Profile。

#### 版本默认 Profile

每个版本有独立的默认 Profile：

```
runtime/chrome/126.0.6478.114/profile/
runtime/chrome/121.0.6167.85/profile/
```

- 运行浏览器时如果不指定 Profile，使用该目录
- 每个版本的 Profile 完全独立
- 卸载版本时不会自动删除，需要手动清理

#### 命名 Profile

用户创建的命名 Profile：

```
runtime/chrome/profiles/work/
runtime/chrome/profiles/test/
runtime/chrome/profiles/personal/
```

- 同一个命名 Profile 可以在不同版本间共享
- 按浏览器类型隔离
- 持久化存储，不受版本卸载影响

## 数据迁移

### 移动数据目录

如果需要将数据移动到其他位置：

1. 停止所有正在运行的浏览器实例
2. 复制或移动整个 `bws-data/` 目录到新位置
3. 验证数据完整性（`bws ls` 检查版本是否正常）

## 磁盘空间管理

### 查看占用空间

```bash
# 查看缓存状态
bws cc info

# 查看所有数据占用空间（需要手动计算）
du -sh bws-data/
```

### 释放空间

```bash
# 清理下载缓存
bws cc clear

# 卸载不需要的版本
bws rm chrome@79

# 卸载不需要的驱动
bws driver uninstall 79

# 清理孤立 Profile
bws pf clean
```

### 各部分空间占用估算

| 目录 | 空间占用 | 说明 |
|------|----------|------|
| `versions/` | 最大 | 每个浏览器版本约 200-500MB |
| `drivers/` | 较小 | 每个主版本驱动约 10-20MB |
| `runtime/` | 中等 | 每个 Profile 约几十到几百 MB |
| `cache/downloads/` | 中等 | 每个安装包约 50-100MB |
| `logs/` | 很小 | 通常几十 MB |
| `bws-client.ini` | 极小 | 几 KB |

## 注意事项

1. **备份建议**：定期备份 `bws-client.ini` 和重要的 Profile 数据
2. **手动编辑**：不建议手动编辑 `bws-client.ini`，使用 `bws cfg` 命令
3. **删除安全**：卸载版本不会删除 Profile，防止误删重要数据
4. **权限**：确保 bws 对数据目录有读写权限
5. **防病毒**：某些杀毒软件可能会误报浏览器文件，建议将 `versions/` 目录加入白名单
6. **磁盘格式**：`versions/` 目录下文件较多，建议使用 NTFS 等支持大量文件的文件系统
