# 安装

本章介绍 bws 的安装方法，包括源码编译和下载预编译二进制两种方式，以及便携模式的说明。

## 源码编译

如果你已安装 Go 环境，可以通过源码编译安装：

```bash
go build -o bws.exe .
```

编译完成后，将生成的 `bws.exe` 放置到你想要的目录即可使用。

### 编译要求

- Go 1.22 或更高版本
- 支持 Windows、macOS、Linux 等主流平台

## 下载预编译二进制

你可以从项目发布页面下载对应平台的预编译二进制文件：

1. 前往项目 Release 页面
2. 下载对应操作系统和架构的压缩包
3. 解压后将二进制文件放到合适的目录

### Windows

下载 `bws_windows_amd64.zip` 或 `bws_windows_386.zip`，解压后得到 `bws.exe`。

### macOS

下载对应架构的版本，解压后赋予执行权限：

```bash
chmod +x bws
```

### Linux

下载对应架构的版本，解压后赋予执行权限：

```bash
chmod +x bws
```

## 便携模式

bws 默认采用便携模式，所有数据都存储在程序同级的 `bws-data/` 目录中。

### 工作原理

将 `bws.exe` 放在任意目录，首次运行后自动在同目录生成 `bws-data/` 文件夹，所有数据（配置、版本、缓存、日志）都在其中。整个程序可以连同数据一起拷贝到 U 盘或其他电脑上使用。

### 目录结构

```
bws/
├── bws.exe
└── bws-data/              # 所有数据都在这里
    ├── config.json       # 配置文件
    ├── logs/             # 日志目录
    ├── cache/            # 下载缓存
    ├── versions/         # 安装的浏览器版本
    └── runtime/          # 运行时数据（Profile 等）
```

### 便携模式的优势

- **即插即用**：拷贝整个目录即可在其他机器上使用
- **数据隔离**：所有数据都在程序目录下，不污染系统
- **易于备份**：只需备份 `bws-data/` 目录即可完整备份所有配置和数据
- **适合 U 盘**：可以放在 U 盘随身携带，在不同电脑上使用

### 传统模式

如果不希望使用便携模式，可以通过以下方式切换到传统模式：

- 设置 `BWS_HOME` 环境变量，指定数据存储目录
- 如果 exe 路径无法获取，会自动回退到用户主目录 `~/.bws/`

```bash
# Windows
set BWS_HOME=D:\browser-data

# Linux / macOS
export BWS_HOME=~/browser-data
```

也可以通过配置命令设置数据目录：

```bash
bws cfg set data-dir D:\browser-data
```
