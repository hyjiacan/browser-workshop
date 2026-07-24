# 快速上手

本章将带你在 5-10 分钟内快速了解 bws 的基本使用方法。

## 前置条件

确保你已经安装了 bws。如果还没有安装，请参考 [安装指南](./installation.md)。

## 1. 查看已安装版本

首先，让我们看看当前系统中已安装的浏览器版本：

```bash
bws ls
```

输出示例：

```
Google Chrome（已安装 3 个，系统 1 个）
  126.0.6478.114 [stable]
  121.0.6167.85
  120.0.6099.109
  125.0.6422.112 [stable] [系统]
```

### 常用筛选方式

```bash
# 只查看指定浏览器
bws ls chrome

# 使用短别名（gc=chrome, ff=firefox, cm=chromium）
bws ls gc

# 按版本前缀筛选
bws ls chrome@79

# 不显示系统浏览器
bws ls --no-system
```

## 2. 从本地目录安装

如果你已经有一些浏览器安装包或绿色版目录，可以使用 `install -d` 命令从本地目录安装：

```bash
# 从目录自动识别并安装
bws i -d /path/to/browser-dir

# 强制重新安装
bws i -d /path/to/browser-dir -f
```

安装过程中会实时显示进度，无法识别的文件会即时提示。

> **提示**：支持的文件格式包括 zip、7z、tar.gz、tar.bz2、tar.xz、.exe 等多种压缩包格式。文件名会被自动识别，详细规则请参考 [本地安装](./import.md) 章节。

## 3. 远程下载安装

如果你没有本地安装包，可以直接从远程源下载安装：

```bash
# 安装最新稳定版
bws i chrome@latest

# 安装指定渠道
bws i chrome@beta

# 安装指定完整版本
bws i chrome@120.0.6478.114

# 安装部分版本号（自动匹配最新的 85.x）
bws i chrome@85
```

### 查看远程可用版本

在安装之前，你可以先查看远程源中有哪些可用版本：

```bash
bws ls --remote chrome
bws ls -R gc@79
bws ls -R chrome --channel beta
```

远程列表会标记本地已安装的版本：

```
chrome 的可用版本：

版本              渠道      平台       架构      状态
--------------  ------  -------  ------  ------
150.0.7871.115  stable  windows  amd64
120.0.6099.109  stable  windows  x64     已安装
  79.0.3945.79  stable  windows  x64     已安装

  已安装 2 个版本。
```

## 4. 运行浏览器

安装完成后，使用 `run` 命令运行浏览器：

```bash
# 运行指定版本
bws r chrome@120

# 运行系统已安装的版本
bws r chrome@system

# 运行默认版本（通过 bws u 设置）
bws r chrome
```

### 常用运行选项

```bash
# 无痕模式
bws r chrome@120 -i

# 新窗口打开
bws r chrome@120 -w

# 无头模式
bws r chrome@120 -H

# 指定命名 Profile
bws r chrome@120 -p myprofile

# 后台运行（不等待进程）
bws r chrome@120 -d

# 打开指定 URL
bws r chrome@120 https://example.com

# 传递浏览器原生参数
bws r chrome@120 -- --disable-gpu --no-sandbox
```

> **提示**：部分版本号匹配时，会列出所有匹配版本并自动选择最新版本。更多运行选项请参考 [运行浏览器](./run.md) 章节。

## 5. 设置默认版本

如果你经常使用某个版本，可以将其设置为默认版本：

```bash
bws u chrome@120
```

设置后可以直接用浏览器名运行，无需指定版本：

```bash
bws r chrome
```

## 下一步

恭喜你完成了 bws 的快速上手！接下来你可以：

- 了解 [浏览器短别名](./short-aliases.md)，减少输入量
- 学习 [版本管理](./version-management.md) 的更多技巧
- 探索 [Profile 管理](./profile.md) 功能
- 配置 [离线源](./config.md)，加速下载
- 搭建 [Serve 服务](./serve.md)，实现团队共享
