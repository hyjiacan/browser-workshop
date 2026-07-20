# 本地导入

bws 支持从本地目录或文件导入浏览器版本，适用于已有浏览器安装包或绿色版的场景。本章详细介绍本地导入的各种方式和规则。

## 从目录批量导入

使用 `import` 命令可以从指定目录批量导入浏览器版本，自动识别目录下所有可识别的浏览器文件。

### 基本用法

```bash
# 自动识别目录下所有浏览器版本
bws import /path/to/browsers
```

### 强制重新导入

默认情况下，已安装的版本会被跳过。使用 `-f` 参数可以强制重新导入：

```bash
bws import /path/to/browsers -f
```

### 导入过程

导入过程中会实时显示进度，包括：

- 当前正在处理的文件
- 识别到的浏览器名称和版本
- 导入进度百分比
- 成功/失败状态

无法识别的文件会即时提示，但不会中断整体导入流程。

### 示例输出

```
扫描目录: D:\browsers
发现 5 个文件，开始识别...

✓ Chrome_120.0.6099.109_Windows_x64.exe → chrome 120.0.6099.109
✓ firefox-121.0-win64.zip → firefox 121.0
✗ unknown_setup.exe → 无法识别
✓ chrome-79.0.3945.79.zip → chrome 79.0.3945.79
✓ Edge_120.0.2210.91_x64.msi → edge 120.0.2210.91

导入完成：成功 4 个，失败 1 个
```

## 从目录安装

使用 `install -d` 命令从单个目录安装，适用于只有一个浏览器版本目录的情况。

### 基本用法

```bash
# 从目录自动识别并安装
bws install -d /path/to/browser-dir

# 指定版本号安装（当无法自动识别时）
bws install -d /path/to/browser-dir chrome@120
```

### 适用场景

- 绿色版浏览器目录
- 解压后的浏览器安装目录
- 手动构建的浏览器目录

## 从文件安装

使用 `install -f` 命令从单个文件安装，支持压缩包和安装包格式。

### 基本用法

```bash
# 从压缩包安装（自动识别）
bws install -f /path/to/chrome-setup.exe
bws install -f /path/to/chrome.zip

# 指定版本号安装（当文件名无法识别时）
bws install -f /path/to/file.exe chrome@120
```

### 与 import 的区别

| 特性 | `import` | `install -d` / `install -f` |
|------|----------|---------------------------|
| 处理数量 | 批量（目录下所有文件） | 单个（一个目录或文件） |
| 自动识别 | 是 | 是（可手动指定） |
| 强制重导 | 支持 `-f` | 每次都重新安装 |
| 适用场景 | 大批量导入 | 单个版本安装 |

## 支持的文件格式

bws 支持 25+ 种文件格式，主要包括以下类别：

### 压缩包格式

| 格式 | 扩展名 | 说明 |
|------|--------|------|
| ZIP | `.zip` | 最常见的压缩格式 |
| 7-Zip | `.7z` | 高压缩比格式 |
| RAR | `.rar` | 常见压缩格式 |
| Tar | `.tar` | Unix 归档格式 |
| Tar+Gzip | `.tar.gz`, `.tgz` | 常见 Unix 压缩格式 |
| Tar+Bzip2 | `.tar.bz2`, `.tbz2` | 高压缩比格式 |
| Tar+XZ | `.tar.xz`, `.txz` | 高压缩比格式 |

### 安装包格式

| 格式 | 扩展名 | 说明 |
|------|--------|------|
| Windows 可执行 | `.exe` | 自解压安装包，直接解压不执行安装 |
| Windows Installer | `.msi` | Microsoft Installer 安装包 |

### 目录格式

直接包含浏览器可执行文件的目录也可以被识别和导入。

## 文件名自动识别规则

bws 通过文件名或目录名中的关键词自动识别浏览器信息，包括浏览器名称、版本号、平台、架构和渠道。

### 识别要素

| 要素 | 识别关键词 | 示例 |
|------|-----------|------|
| **浏览器名** | chrome, chromium, firefox, edge, brave, opera 等 | `chrome`, `firefox` |
| **版本号** | 形如 `x.y.z.w` 的数字组合 | `120.0.6099.109` |
| **架构** | `win64`/`x64`/`amd64` 或 `win32`/`x86`/`386` | `x64`, `win64` |
| **平台** | `windows`/`win`、`macos`/`mac`、`linux` | `windows`, `win` |
| **渠道** | `stable`, `beta`, `dev`, `canary`, `esr` | `stable`, `beta` |

### 文件名示例

以下是一些可以被正确识别的文件名示例：

```
44.0.2403.107_chrome64_stable_windows_installer.exe
GoogleChrome_148.0.7778.167_Windows_x64_Offline.exe
firefox-115.0esr-win64.zip
Chrome_120.0.6099.109_Windows_x64.zip
chromium-85.0.4183.121-linux-x64.tar.gz
MicrosoftEdge_120.0.2210.91_x64.msi
```

### 识别优先级

1. 首先识别浏览器名称
2. 然后提取版本号
3. 接着识别架构、平台、渠道
4. 如果缺少部分信息，使用默认值（如默认 stable 渠道）

## 无法识别时的处理方法

如果文件名或目录名无法被自动识别，可以通过手动指定版本的方式进行安装。

### 指定版本安装

使用 `install -d` 或 `install -f` 命令时，在末尾加上版本标识：

```bash
# 从目录安装，手动指定版本
bws install -d /path/to/dir chrome@120

# 从文件安装，手动指定版本
bws install -f /path/to/file.exe chrome@120
```

### 指定版本的格式

版本标识的格式为 `browser@version`：

- `browser`：浏览器名称（如 chrome、firefox、chromium）或短别名（gc、ff、cm）
- `version`：版本号，可以是完整版本号或部分版本号

示例：

```bash
bws install -f ./mystery.exe gc@120.0.6099.109
bws install -d ./my-browser ff@115.0esr
```

### 指定渠道和架构

如果需要更精确的指定，还可以配合其他参数：

```bash
bws install -f ./file.exe chrome@120 --channel beta
```

### 无法识别的常见原因

1. 文件名中没有明显的浏览器名称关键词
2. 版本号格式不标准
3. 缺少架构或平台标识（会使用系统默认值）
4. 自定义命名的文件无法匹配规则

当遇到无法识别的文件时，建议使用手动指定版本的方式进行安装。
