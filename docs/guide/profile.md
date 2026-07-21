# Profile 管理

每个浏览器版本都有独立的用户数据目录（Profile），用于存储书签、历史记录、Cookie、扩展程序等数据。bws 提供了完整的 Profile 管理功能，包括默认 Profile、命名 Profile、重置和清理等。

## 默认 Profile

### 工作原理

每个版本的浏览器默认使用独立的 Profile 目录，数据互不干扰。这是 bws 的核心特性之一，确保不同版本的浏览器之间不会产生配置冲突。

默认 Profile 的路径：

```
bws-data/runtime/{browser}/{version}/profile/
```

例如，Chrome 120.0.6099.109 的默认 Profile 路径为：

```
bws-data/runtime/chrome/120.0.6099.109/profile/
```

### 特点

- 每个版本一个独立 Profile
- 版本之间数据完全隔离
- 无需手动指定，自动创建
- 卸载浏览器时不会自动删除（防止误删）

### 运行默认 Profile

运行浏览器时如果不指定 Profile，默认使用该版本的默认 Profile：

```bash
bws r chrome@120
```

## 命名 Profile

命名 Profile 是用户自定义的 Profile，可以在不同版本的同一浏览器之间共享。

### 工作原理

命名 Profile 的路径：

```
bws-data/runtime/{browser}/profiles/{name}/
```

例如，名为 `work` 的 Chrome 命名 Profile 路径为：

```
bws-data/runtime/chrome/profiles/work/
```

### 使用命名 Profile

使用 `-p` 或 `--profile` 参数指定命名 Profile：

```bash
bws r chrome@120 -p work
bws r chrome@121 -p work
```

### 命名 Profile 的特点

- 同一个命名 Profile 可以在不同版本间共享
- 适合区分工作、个人、测试等不同场景
- 数据持久化，不受版本卸载影响
- 按浏览器类型隔离（Chrome 和 Firefox 的同名 Profile 互不影响）

### 使用场景示例

```bash
# 工作用 Profile - 保存工作相关的书签和登录状态
bws r chrome@120 -p work

# 个人用 Profile - 保存个人浏览数据
bws r chrome@120 -p personal

# 测试用 Profile - 干净的测试环境
bws r chrome@120 -p test

# 开发用 Profile - 安装开发扩展
bws r chrome@120 -p dev
```

## 列出 Profile

使用 `bws pf list` 命令列出所有 Profile。

### 列出所有 Profile

```bash
bws pf list
```

### 列出指定浏览器的 Profile

```bash
bws pf list chrome
bws pf list gc
```

### 输出内容

列出的信息包括：

- 默认 Profile：每个已安装版本的默认 Profile
- 命名 Profile：用户创建的所有命名 Profile
- 孤立 Profile：已卸载版本残留的 Profile

## 查看 Profile 路径

使用 `bws pf path` 命令查看 Profile 的实际路径。

### 查看默认 Profile 路径

```bash
bws pf path chrome@120
```

### 查看命名 Profile 路径

```bash
bws pf path chrome myprofile
```

### 使用场景

- 手动备份 Profile 数据
- 查看 Profile 中的具体文件
- 调试浏览器问题
- 手动清理某些数据

## 重置 Profile

使用 `bws pf reset` 命令重置 Profile，清除所有数据恢复初始状态。

### 重置默认 Profile

```bash
bws pf reset chrome@120
```

### 重置命名 Profile

```bash
bws pf reset chrome@120 myprofile
```

### 跳过确认

默认情况下，重置前会显示确认提示。使用 `-f` 参数跳过确认：

```bash
bws pf reset chrome@120 -f
bws pf reset chrome@120 myprofile -f
```

### 重置效果

重置 Profile 会：

1. 删除 Profile 目录下的所有文件
2. 重新创建空的 Profile 目录
3. 浏览器下次启动时生成全新的配置

### 使用场景

- 浏览器出现异常，需要恢复初始状态
- 测试需要干净的环境
- 清除所有浏览数据
- 排查扩展程序冲突

> **注意**：重置操作不可逆，Profile 中的所有数据（书签、密码、扩展等）都会被永久删除，请谨慎操作。

## 清理孤立 Profile

卸载浏览器版本后，对应的 Profile 数据不会自动删除，会成为"孤立 Profile"。使用 `bws pf clean` 命令可以清理这些残留的 Profile 数据。

### 清理所有孤立 Profile

```bash
bws pf clean
```

### 清理指定浏览器的孤立 Profile

```bash
bws pf clean chrome
bws pf clean gc
```

### 跳过确认

使用 `-f` 参数跳过确认提示：

```bash
bws pf clean -f
bws pf clean chrome -f
```

### 什么是孤立 Profile

孤立 Profile 指的是：

- 已卸载版本的默认 Profile
- 没有任何版本在使用的命名 Profile（通常不会出现）

### 为什么需要清理

- 释放磁盘空间
- 保持数据目录整洁
- 删除不再需要的旧数据

### 清理流程

1. 扫描所有 Profile 目录
2. 检查对应版本是否仍已安装
3. 列出所有未关联的孤立 Profile
4. 确认后删除

## Profile 命令汇总

| 命令 | 说明 |
|------|------|
| `bws pf list` | 列出所有 Profile |
| `bws pf list <browser>` | 列出指定浏览器的 Profile |
| `bws pf path <browser@version>` | 查看默认 Profile 路径 |
| `bws pf path <browser> <name>` | 查看命名 Profile 路径 |
| `bws pf reset <browser@version>` | 重置默认 Profile |
| `bws pf reset <browser@version> <name>` | 重置命名 Profile |
| `bws pf reset ... -f` | 跳过确认直接重置 |
| `bws pf clean` | 清理所有孤立 Profile |
| `bws pf clean <browser>` | 清理指定浏览器的孤立 Profile |
| `bws pf clean ... -f` | 跳过确认直接清理 |

## Profile 目录结构

```
bws-data/runtime/
└── chrome/
    ├── 120.0.6099.109/
    │   └── profile/           # 120 版本的默认 Profile
    ├── 121.0.6167.85/
    │   └── profile/           # 121 版本的默认 Profile
    └── profiles/
        ├── work/              # 命名 Profile "work"
        ├── test/              # 命名 Profile "test"
        └── personal/          # 命名 Profile "personal"
```

## 注意事项

1. **数据安全**：重置和清理操作不可逆，操作前请确认已备份重要数据
2. **版本隔离**：不同版本的默认 Profile 完全独立，互不影响
3. **命名共享**：命名 Profile 可以在同一浏览器的不同版本间共享
4. **跨浏览器隔离**：不同浏览器的 Profile 完全隔离，即使同名也不共享
5. **卸载保留**：卸载浏览器版本时，Profile 数据保留，需手动清理
6. **磁盘空间**：Profile 数据可能占用较大空间（尤其是缓存），定期清理可释放空间
