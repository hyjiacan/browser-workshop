# 运行浏览器

bws 的 `run` 命令用于启动指定版本的浏览器，支持多种运行模式和参数选项。本章详细介绍运行浏览器的各种用法。

## 基本用法

### 运行指定版本

```bash
# 运行指定完整版本
bws run chrome@120.0.6099.109

# 运行部分版本号（自动选择最新匹配版本）
bws run chrome@120
```

### 运行系统版本

运行系统已安装的浏览器版本：

```bash
bws run chrome@system
```

### 运行默认版本

运行通过 `bws use` 设置的默认版本：

```bash
bws run chrome
```

如果没有设置默认版本，会提示错误。

### 使用短别名

```bash
bws run gc@120       # chrome
bws run ff           # firefox 默认版本
bws run cm@latest    # chromium 最新版
```

### 打开指定 URL

在运行浏览器时直接打开指定网址：

```bash
bws run chrome@120 https://example.com
bws run gc https://github.com
```

## 无头模式

使用 `-H` 或 `--headless` 参数以无头模式运行浏览器，适用于自动化测试和脚本场景。

```bash
bws run chrome@120 -H
bws run chrome@120 --headless
```

无头模式下浏览器不会显示图形界面，所有操作在后台完成。常用于：

- 自动化测试
- 网页截图
- 性能测试
- 爬虫脚本

## 隐身模式

使用 `-i` 或 `--incognito` 参数以隐身/无痕模式运行浏览器。

```bash
bws run chrome@120 -i
bws run chrome@120 --incognito
```

隐身模式的特点：

- 不保存浏览历史
- 不保存 Cookie 和网站数据
- 不保存表单数据
- 关闭浏览器后自动清除会话数据

## 新窗口

使用 `-w` 或 `--new-window` 参数强制在新窗口中打开浏览器。

```bash
bws run chrome@120 -w
bws run chrome@120 --new-window
```

即使该版本的浏览器已经在运行，也会打开一个新的窗口。

## 命名 Profile

使用 `-p` 或 `--profile` 参数指定命名 Profile 运行浏览器。

```bash
bws run chrome@120 -p myprofile
bws run chrome@120 --profile work
```

### 命名 Profile 的特点

- 同一个命名 Profile 可以在不同版本间共享
- 每个命名 Profile 有独立的数据目录
- 适合区分工作、个人、测试等不同场景

### 示例

```bash
# 工作用 Profile
bws run chrome@120 -p work

# 同一个 work Profile 也可以在 121 版本上使用
bws run chrome@121 -p work

# 测试用 Profile
bws run chrome@120 -p test
```

更多 Profile 管理功能请参考 [Profile 管理](./profile.md) 章节。

## 原生模式

使用 `--native` 参数以原生模式运行浏览器，即不使用 bws 管理的 Profile，直接使用系统默认的用户数据目录。

```bash
bws run chrome@120 --native
```

原生模式的特点：

- 使用系统默认的浏览器用户数据目录
- 与直接运行浏览器效果相同
- 不同版本可能共享同一个 Profile
- 适用于需要与系统浏览器保持一致的场景

> **注意**：原生模式下，不同版本共享 Profile 可能导致配置冲突或数据损坏，建议谨慎使用。

## 后台运行

使用 `-d` 或 `--detach` 参数让浏览器在后台运行，bws 命令立即返回，不等待浏览器进程结束。

```bash
bws run chrome@120 -d
bws run chrome@120 --detach
```

### 使用场景

- 脚本中启动浏览器后继续执行其他操作
- 不需要等待浏览器关闭
- 自动化脚本中的后台服务

默认情况下（不带 `-d`），`bws run` 命令会等待浏览器进程结束后才返回。

## 试运行

使用 `--dry-run` 参数进行试运行，只显示将要执行的命令而不实际启动浏览器。

```bash
bws run chrome@120 --dry-run
```

### 输出示例

```
将要执行的命令：
C:\bws\bws-data\versions\chrome\120.0.6099.109\chrome.exe --user-data-dir=C:\bws\bws-data\runtime\chrome\120.0\profile
```

试运行的用途：

- 调试命令参数
- 确认浏览器路径和启动参数
- 查看 Profile 目录位置
- 验证参数传递是否正确

## 传递浏览器原生参数

使用 `--` 分隔符，可以向浏览器传递原生命令行参数。`--` 之后的所有参数都会原样传递给浏览器。

```bash
bws run chrome@120 -- --disable-gpu --no-sandbox
bws run chrome@120 -i -- --window-size=1920,1080
bws run ff -- --private-window
```

### 常用原生参数示例

Chrome 常用参数：

```bash
# 禁用 GPU 加速
bws run chrome@120 -- --disable-gpu

# 禁用沙箱
bws run chrome@120 -- --no-sandbox

# 指定窗口大小
bws run chrome@120 -- --window-size=1920,1080

# 指定启动位置
bws run chrome@120 -- --window-position=0,0

# 禁用扩展
bws run chrome@120 -- --disable-extensions

# 启动时最大化
bws run chrome@120 -- --start-maximized
```

Firefox 常用参数：

```bash
# 隐私窗口
bws run firefox -- --private-window

# 安全模式
bws run firefox -- --safe-mode
```

## 部分版本号匹配

当使用部分版本号时，bws 会列出所有匹配的版本并自动选择最新版本。

### 匹配输出示例

```
匹配 chrome@85 的版本：
> 85.0.4183.121
  85.0.4183.83
  85.0.4183.10
```

其中 `>` 标记表示当前选中的版本（最新版）。

## 运行选项汇总

| 选项 | 简写 | 说明 |
|------|------|------|
| `--headless` | `-H` | 无头模式 |
| `--incognito` | `-i` | 隐身/无痕模式 |
| `--new-window` | `-w` | 新窗口打开 |
| `--profile <name>` | `-p` | 指定命名 Profile |
| `--native` | - | 原生模式（使用系统 Profile） |
| `--detach` | `-d` | 后台运行（不等待进程） |
| `--dry-run` | - | 试运行（不实际启动） |
| `--` | - | 传递浏览器原生参数 |

## 组合使用示例

多个选项可以组合使用：

```bash
# 无头模式 + 命名 Profile + 原生参数
bws run chrome@120 -H -p test -- --disable-gpu --no-sandbox

# 隐身模式 + 新窗口 + 打开 URL
bws run chrome@120 -i -w https://example.com

# 后台运行 + 原生模式
bws run chrome@system -d --native
```
