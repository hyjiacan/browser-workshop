# 数据存储

bws 将所有数据（配置、版本、缓存、日志、Profile 等）统一存储在数据目录中。本章介绍数据存储的目录结构和各目录的用途。

## 便携模式（默认）

默认情况下，bws 采用便携模式，数据存储在程序同级的 `bws-data/` 目录中。

### 目录结构

```
bws/
├── bws.exe                    # 程序主文件
└── bws-data/                  # 数据根目录
    ├── config.json            # 配置文件
    ├── logs/                  # 日志目录
    │   └── bws.log            # 主日志文件
    ├── cache/                 # 下载缓存
    │   ├── manifests/         # 版本清单缓存
    │   └── downloads/         # 下载文件缓存
    ├── versions/              # 安装的浏览器版本
    │   ├── chrome/
    │   │   ├── 126.0.6478.114/
    │   │   ├── 121.0.6167.85/
    │   │   └── 79.0.3945.79/
    │   ├── firefox/
    │   │   └── 121.0/
    │   └── ...
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

### config.json

配置文件，存储所有用户配置项。JSON 格式。

```json
{
  "default-browser": "chrome",
  "default-channel": "stable",
  "log-level": "info",
  "dataDir": "",
  "repo-path": "",
  "source": ""
}
```

通常不需要手动编辑，使用 `bws cfg` 命令管理。

如果需要将数据存储到其他位置，可以通过配置命令设置自定义数据目录：

```bash
bws cfg set data-dir D:\browser-data
```

### logs/

日志目录，存储 bws 的运行日志。

- `bws.log`：主日志文件，记录所有操作
- 文件日志默认 DEBUG 级别，详细记录所有操作
- 日志会自动轮转，防止单个文件过大

更多日志相关信息请参考 [日志系统](./logging.md) 章节。

### cache/

缓存目录，存储下载的临时文件和清单缓存。

#### cache/manifests/

版本清单缓存，存储从远程源获取的版本列表，避免每次都重新请求。

- 加速 `ls --remote` 等命令的响应
- 有过期时间，过期后自动重新获取
- 可以通过 `bws cc clear` 清理

#### cache/downloads/

下载文件缓存，存储通过 `download` 命令下载的安装包，以及 `install` 命令下载的临时文件。

- 下载的文件会保留在这里，便于后续重复使用
- 占用空间可能较大，可定期清理
- 可以通过 `bws cc clear` 清理
- 可以通过 `bws cc size` 查看占用空间

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
# 查看缓存大小
bws cc size

# 查看所有数据占用空间（需要手动计算）
du -sh bws-data/
```

### 释放空间

```bash
# 清理下载缓存
bws cc clear

# 卸载不需要的版本
bws rm chrome@79

# 清理孤立 Profile
bws pf clean
```

### 各部分空间占用估算

| 目录 | 空间占用 | 说明 |
|------|----------|------|
| `versions/` | 最大 | 每个浏览器版本约 200-500MB |
| `runtime/` | 中等 | 每个 Profile 约几十到几百 MB |
| `cache/downloads/` | 中等 | 每个安装包约 50-100MB |
| `logs/` | 很小 | 通常几十 MB |
| `config.json` | 极小 | 几 KB |

## 注意事项

1. **备份建议**：定期备份 `config.json` 和重要的 Profile 数据
2. **手动编辑**：不建议手动编辑 `config.json`，使用 `bws cfg` 命令
3. **删除安全**：卸载版本不会删除 Profile，防止误删重要数据
4. **权限**：确保 bws 对数据目录有读写权限
5. **防病毒**：某些杀毒软件可能会误报浏览器文件，建议将 `versions/` 目录加入白名单
6. **磁盘格式**：`versions/` 目录下文件较多，建议使用 NTFS 等支持大量文件的文件系统
