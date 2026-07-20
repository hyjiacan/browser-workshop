# 浏览器短别名

为了方便输入，bws 为常见浏览器提供了简短别名，可在所有命令中使用。使用短别名可以大幅减少命令输入量，提高操作效率。

## 支持的短别名列表

| 短别名 | 完整名称 | 说明 |
|--------|----------|------|
| `gc` | chrome / googlechrome | Google Chrome 浏览器 |
| `ff` | firefox | Mozilla Firefox 浏览器 |
| `cm` | chromium | Chromium 开源浏览器 |

## 使用示例

### 列出版本

```bash
# 完整写法
bws ls chrome

# 短别名写法
bws ls gc
```

### 安装浏览器

```bash
# 完整写法
bws install chrome@latest

# 短别名写法
bws install gc@latest
```

### 运行浏览器

```bash
# 完整写法
bws run firefox@120

# 短别名写法
bws run ff@120
```

### 版本筛选

```bash
# 完整写法
bws ls chromium@79

# 短别名写法
bws ls cm@79
```

### 远程列表

```bash
# 完整写法
bws ls --remote chrome

# 短别名写法
bws ls -R gc
```

### 查看版本信息

```bash
# 完整写法
bws info chrome@120

# 短别名写法
bws info gc@120
```

### 设置默认版本

```bash
# 完整写法
bws use chrome@120

# 短别名写法
bws use gc@120
```

### 卸载版本

```bash
# 完整写法
bws uninstall chrome@120

# 短别名写法
bws uninstall gc@120
```

### 仅下载不安装

```bash
# 完整写法
bws download chrome@120

# 短别名写法
bws download gc@120
```

### Profile 管理

```bash
# 完整写法
bws profile list chrome

# 短别名写法
bws profile list gc
```

## 支持的命令范围

短别名在 bws 的所有命令中都可以使用，包括但不限于：

| 命令 | 支持短别名 | 示例 |
|------|-----------|------|
| `ls` / `list` | 是 | `bws ls gc` |
| `ls --remote` / `ls -R` | 是 | `bws ls -R ff` |
| `info` | 是 | `bws info cm@120` |
| `run` | 是 | `bws run gc@120` |
| `install` | 是 | `bws install ff@latest` |
| `import` | 否 | 批量导入，无需指定浏览器 |
| `uninstall` | 是 | `bws uninstall gc@120` |
| `use` | 是 | `bws use cm@120` |
| `download` | 是 | `bws download ff@beta` |
| `profile` | 是 | `bws profile list gc` |
| `config` | 否 | 配置管理命令 |
| `serve` | 否 | 服务端命令 |
| `repo` | 否 | 仓库管理命令 |
| `cache` | 否 | 缓存管理命令 |
| `doctor` | 否 | 系统检查命令 |

## 使用技巧

### 与版本号组合使用

短别名可以与版本号灵活组合：

```bash
bws ls gc@120          # 列出 chrome 120.x 版本
bws install ff@beta    # 安装 firefox beta 版
bws run cm@latest      # 运行最新版 chromium
```

### 与渠道组合使用

```bash
bws ls -R gc --channel beta     # 查看 chrome beta 渠道版本
bws install ff --channel dev    # 安装 firefox dev 版
```

### 与系统版本组合使用

```bash
bws run gc@system       # 运行系统安装的 Chrome
bws info ff@system      # 查看系统 Firefox 信息
```

## 注意事项

1. 短别名不区分大小写，`gc` 和 `GC` 效果相同
2. 短别名仅用于命令行输入简化，内部存储和显示仍使用完整名称
3. 如果输入的名称既不是短别名也不是完整浏览器名，会提示错误
4. 未来版本可能会添加更多短别名，以 `bws help` 中的说明为准
