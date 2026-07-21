# 远程下载

bws 支持从远程源下载浏览器版本，包括在线源和离线源两种方式。本章详细介绍远程下载的配置和使用方法。

## 在线源和离线源

bws 支持两种类型的远程源：

### 在线源

在线源是 bws 内置的官方下载源，直接从浏览器厂商的官方服务器获取版本信息和安装包。

- **Chrome**：通过 Google Omaha 协议（官方更新协议）获取版本列表和下载地址
- **Firefox**：从 Mozilla 官方 FTP/下载服务器获取
- **其他浏览器**：各自的官方更新渠道

在线源的特点：
- 版本最新、最全
- 无需配置，开箱即用
- 需要访问外网
- 下载速度取决于网络环境

### 离线源

离线源是通过 `bws sv` 搭建的本地/局域网分发服务，提供浏览器版本的内网分发。

- 由 `bws sv` 命令提供服务
- 存储在本地 `packages/` 目录中
- 支持自动同步在线源的版本
- 适合团队内部或离线环境使用

离线源的特点：
- 下载速度快（局域网内）
- 可离线使用
- 需要手动搭建和维护
- 版本取决于同步情况

## 源优先级

bws 采用固定的源优先级策略：

```
离线源 → 在线源
```

即 **离线源优先，在线源兜底**。

### 优先级规则

1. 如果配置了离线源（`source` 配置项），优先从离线源查询和下载
2. 离线源中找不到的版本，自动回退到在线源
3. 两个源都找不到时，提示错误

### 优先级示例

假设配置了离线源 `http://server:8080`，执行 `bws i chrome@120` 时：

1. 先查询 `http://server:8080` 是否有 chrome 120 版本
2. 如果有，从离线源下载（速度快）
3. 如果没有，从 Google Omaha 在线源下载
4. 如果在线源也没有，报错"未找到该版本"

## 配置离线源

使用 `bws cfg` 命令配置离线源地址。

### 设置离线源

```bash
bws cfg set source http://server:8080
```

也可以使用 `remote-source` 配置项（与 `source` 等效）：

```bash
bws cfg set remote-source http://server:8080
```

### 查看当前源

```bash
bws cfg get source
```

### 清除离线源配置

```bash
bws cfg set source ""
```

清除后将只使用在线源。

### 客户端配置步骤

1. 确保服务端已启动 `bws sv`
2. 在客户端执行配置命令：

```bash
bws cfg set source http://server-ip:8080
```

3. 验证配置是否生效：

```bash
bws ls -R chrome
```

如果能从离线源获取版本列表，说明配置成功。

## 列出远程版本

使用 `ls --remote` 或 `ls -R` 命令列出远程可用的浏览器版本。

### 基本用法

```bash
# 列出所有浏览器的远程版本
bws ls --remote
bws ls -R

# 列出指定浏览器的远程版本
bws ls -R chrome
bws ls -R gc
```

### 指定渠道

```bash
bws ls -R chrome --channel beta
bws ls -R ff --channel dev
```

### 版本前缀筛选

```bash
bws ls -R chrome@79
bws ls -R gc@120
```

### 输出示例

```
chrome 的可用版本：

版本              渠道      平台       架构      状态
--------------  ------  -------  ------  ------
150.0.7871.115  stable  windows  amd64
120.0.6099.109  stable  windows  x64     已安装
  79.0.3945.79  stable  windows  x64     已安装
148.0.7778.167  beta    windows  amd64

  已安装 2 个版本。
```

输出说明：
- 表格列出版本号、渠道、平台、架构和状态
- "已安装"标记表示该版本已在本地安装
- 版本按版本号从高到低排序

## 下载安装

使用 `install` 命令从远程源下载并安装浏览器版本。

### 基本用法

```bash
# 安装最新稳定版
bws i chrome@latest

# 安装指定渠道的最新版
bws i chrome@beta
bws i chrome@dev
bws i chrome@canary

# 安装指定完整版本
bws i chrome@120.0.6478.114

# 安装部分版本号（自动匹配最新的匹配版本）
bws i chrome@85
```

### 安装过程

1. 查询远程源获取版本信息
2. 下载安装包到缓存目录
3. 校验文件完整性
4. 解压安装到 versions 目录
5. 更新版本清单

下载过程中会显示进度条和下载速度。

### 使用短别名

```bash
bws i gc@latest     # chrome
bws i ff@beta       # firefox
bws i cm@120        # chromium
```

## 仅下载不安装

使用 `download` 命令仅下载安装包但不安装，适用于需要缓存安装包或手动处理的场景。

### 基本用法

```bash
# 下载最新稳定版
bws dl chrome@latest

# 下载指定版本
bws dl chrome@120.0.6478.114

# 下载部分版本号
bws dl chrome@85
```

### 下载文件位置

下载的文件默认保存在当前工作目录中。可通过 `bws dl --output` 指定其他保存路径。

### 与 install 的区别

| 特性 | `download` | `install` |
|------|-----------|-----------|
| 下载文件 | 是 | 是 |
| 安装到 versions | 否 | 是 |
| 可直接运行 | 否 | 是 |
| 占用空间 | 仅压缩包 | 压缩包 + 解压后文件 |

### 使用场景

- 预先下载多个版本，后续离线安装
- 下载安装包用于其他用途
- 批量下载用于搭建离线源

下载后的安装包可以通过 `bws i --from-file` 命令进行本地安装。
