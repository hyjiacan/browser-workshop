# bws 命令测试计划

> 版本: 0.1.0 | 更新日期: 2026-07-14
> 仓库: https://github.com/hyjiacan/bws | https://gitee.com/hyjiacan/bws

## 1. 测试策略

### 1.1 测试分层

| 层级 | 范围 | 工具 | 目标 |
|------|------|------|------|
| 单元测试 | 内部函数、工具方法 | Go testing | 覆盖核心逻辑 |
| 集成测试 | 命令执行（带模拟依赖） | Go testing + mock | 覆盖命令路径 |
| E2E 测试 | 完整命令链（真实网络/文件） | 脚本 + 断言 | 覆盖用户场景 |
| 手动测试 | UI/UX、文档、边缘场景 | 人工验证 | 覆盖体验问题 |

### 1.2 优先级定义

- **P0 (阻塞)** — 核心功能，必须100%通过，失败则阻断发布
- **P1 (重要)** — 常用功能，应通过，失败需修复
- **P2 (一般)** — 辅助功能，建议通过，可延期修复

---

## 2. ls / list 命令测试

### 2.1 本地列表 (bws ls)

| ID | 场景 | 输入 | 预期 | 优先级 |
|----|------|------|------|--------|
| LS-01 | 正常 — 列出所有本地版本 | `bws ls` | 显示表格，含版本/渠道/平台/架构/状态 | P0 |
| LS-02 | 正常 — 按浏览器筛选 | `bws ls chrome` | 只显示 chrome 版本 | P0 |
| LS-03 | 正常 — 使用短别名 | `bws ls gc` | 与 `bws ls chrome` 结果一致 | P0 |
| LS-04 | 正常 — 按版本前缀筛选 | `bws ls gc@120` | 只显示 120.x 版本 | P0 |
| LS-05 | 正常 — 精确版本匹配 | `bws ls gc@120.0.6099.109` | 显示该精确版本或空 | P1 |
| LS-06 | 异常 — 不支持的浏览器 | `bws ls safari` | 报错 "不支持的浏览器" | P1 |
| LS-07 | 异常 — 空仓库 | `bws ls` (空仓库) | 提示 "暂无已安装版本" | P1 |
| LS-08 | 边界 — 版本号格式错误 | `bws ls gc@abc` | 空结果或友好提示 | P2 |

### 2.2 远程列表 (bws ls -R)

| ID | 场景 | 输入 | 预期 | 优先级 |
|----|------|------|------|--------|
| LS-R-01 | 正常 — 查询远程版本 | `bws ls -R ff` | 从远程源获取并显示版本列表 | P0 |
| LS-R-02 | 正常 — 按版本前缀筛选 | `bws ls -R ff@101` | 只显示匹配 101 的版本 | P0 |
| LS-R-03 | 正常 — 指定渠道 | `bws ls -R ff -c beta` | 只显示 beta 渠道版本 | P0 |
| LS-R-04 | 正常 — 所有渠道 | `bws ls -R ff -a` | 显示所有渠道版本 | P1 |
| LS-R-05 | 正常 — 限制数量 | `bws ls -R ff -n 5` | 最多显示 5 个版本 | P1 |
| LS-R-06 | 正常 — 大量结果分组 | `bws ls -R ff` (>30 版本) | 按主版本分组显示 | P0 |
| LS-R-07 | 异常 — 无网络 | `bws ls -R ff` (断网) | 提示网络错误或回退本地 | P1 |
| LS-R-08 | 异常 — 远程源关闭 | `bws ls -R ff` (serve 关闭) | 提示源不可用，尝试其他源 | P1 |
| LS-R-09 | 边界 — 0 结果 | `bws ls -R ff@999` | 提示 "未找到匹配的版本" | P1 |

---

## 3. install 命令测试

| ID | 场景 | 输入 | 预期 | 优先级 |
|----|------|------|------|--------|
| INS-01 | 正常 — 安装最新版 | `bws install ff@latest` | 下载并安装最新稳定版 | P0 |
| INS-02 | 正常 — 安装指定版本 | `bws install ff@101.0.1` | 下载并安装该版本 | P0 |
| INS-03 | 正常 — 版本前缀安装 | `bws install gc@120` | 安装 120.x 最新匹配版 | P0 |
| INS-04 | 正常 — 使用短别名 | `bws install ff@latest` | 与全称效果一致 | P0 |
| INS-05 | 正常 — 指定渠道 | `bws install ff@latest -c esr` | 安装 ESR 版本 | P1 |
| INS-06 | 正常 — 指定平台 | `bws install gc@120 -p mac` | 下载 mac 版本 | P1 |
| INS-07 | 正常 — 指定架构 | `bws install gc@120 -a arm64` | 下载 arm64 版本 | P1 |
| INS-08 | 正常 — 从目录安装 | `bws install -d /path/to/dir` | 识别并安装目录中的浏览器 | P1 |
| INS-09 | 正常 — 从文件安装 | `bws install --from-file /path/to.zip` | 解压并安装 | P1 |
| INS-10 | 正常 — 强制覆盖 | `bws install ff@101 -f` | 覆盖已存在的同名版本 | P1 |
| INS-11 | 异常 — 版本不存在 | `bws install ff@999.999` | 报错 "版本未找到" | P1 |
| INS-12 | 异常 — 磁盘空间不足 | `bws install ff@latest` (空间不足) | 提示确认或报错 | P1 |
| INS-13 | 异常 — 无网络 | `bws install ff@latest` (断网) | 报错 "网络连接失败" | P1 |
| INS-14 | 边界 — 重复安装（无 -f） | `bws install ff@101` (已存在) | 提示已存在，跳过或询问 | P1 |

---

## 4. run 命令测试

| ID | 场景 | 输入 | 预期 | 优先级 |
|----|------|------|------|--------|
| RUN-01 | 正常 — 运行指定版本 | `bws run gc@120` | 启动 Chrome 120 | P0 |
| RUN-02 | 正常 — 隐身模式 | `bws run gc@120 -i` | 以隐身模式启动 | P1 |
| RUN-03 | 正常 — 指定 Profile | `bws run gc@120 -p myprofile` | 使用指定 Profile 启动 | P1 |
| RUN-04 | 正常 — 无头模式 | `bws run gc@120 -H` | 以无头模式启动 | P1 |
| RUN-05 | 正常 — 新窗口 | `bws run gc@120 -w` | 打开新窗口 | P2 |
| RUN-06 | 正常 — 传递浏览器参数 | `bws run gc@120 -- https://example.com` | 启动并打开指定 URL | P1 |
| RUN-07 | 正常 — 试运行 | `bws run gc@120 --dry-run` | 打印启动命令，不执行 | P2 |
| RUN-08 | 异常 — 版本未安装 | `bws run gc@999` | 报错 "版本未安装" | P1 |
| RUN-09 | 异常 — 不支持的浏览器 | `bws run safari@latest` | 报错 "不支持的浏览器" | P1 |

---

## 5. uninstall / remove 命令测试

| ID | 场景 | 输入 | 预期 | 优先级 |
|----|------|------|------|--------|
| UNI-01 | 正常 — 卸载指定版本 | `bws uninstall ff@101` | 删除该版本文件和配置 | P0 |
| UNI-02 | 正常 — 使用 rm 别名 | `bws rm ff@101` | 与 uninstall 效果一致 | P0 |
| UNI-03 | 正常 — 使用 remove 别名 | `bws remove ff@101` | 与 uninstall 效果一致 | P0 |
| UNI-04 | 异常 — 版本未安装 | `bws uninstall ff@999` | 报错 "版本未安装" | P1 |
| UNI-05 | 异常 — 版本正在运行 | `bws uninstall ff@101` (运行中) | 提示占用或强制卸载 | P2 |

---

## 6. import 命令测试

| ID | 场景 | 输入 | 预期 | 优先级 |
|----|------|------|------|--------|
| IMP-01 | 正常 — 从目录导入 | `bws import /path/to/browsers` | 扫描并导入所有识别到的浏览器 | P0 |
| IMP-02 | 正常 — flags 前置 | `bws import -f /path/to/browsers` | 正确识别 -f 并导入 | P0 |
| IMP-03 | 正常 — 强制覆盖 | `bws import /path -f` | 覆盖已存在的版本 | P1 |
| IMP-04 | 异常 — 目录不存在 | `bws import /not/exist` | 报错 "目录不存在" | P1 |
| IMP-05 | 异常 — 空目录 | `bws import /empty/dir` | 提示 "未找到可导入的文件" | P1 |
| IMP-06 | 边界 — 大目录导入 | `bws import /very/large/dir` | 显示流式进度，不卡住 | P1 |

---

## 7. repo 命令测试

| ID | 场景 | 输入 | 预期 | 优先级 |
|----|------|------|------|--------|
| REPO-01 | 正常 — 设置路径 | `bws repo set D:\browsers` | 保存路径到配置 | P0 |
| REPO-02 | 正常 — 查看路径 | `bws repo path` | 显示当前仓库路径 | P0 |
| REPO-03 | 正常 — 扫描仓库 | `bws repo scan` | 扫描并识别仓库中的所有版本 | P0 |
| REPO-04 | 正常 — 导入到仓库 | `bws repo import /path` | 导入并归类到仓库 | P1 |
| REPO-05 | 异常 — 路径不存在 | `bws repo set /not/exist` | 报错或创建目录 | P1 |
| REPO-06 | 边界 — 路径含空格 | `bws repo set "D:\My Browsers"` | 正确处理空格 | P1 |

---

## 8. serve 命令测试

| ID | 场景 | 输入 | 预期 | 优先级 |
|----|------|------|------|--------|
| SRV-01 | 首次运行 — 创建配置 | `bws serve`（无 bws-serve.ini） | 创建默认配置文件，提示编辑后重新运行 | P0 |
| SRV-02 | 正常 — 启动服务 | `bws serve`（已有 bws-serve.ini） | 启动 HTTP 服务，监听配置端口 | P0 |
| SRV-03 | 异常 — 端口占用 | `bws serve`（端口占用） | 报错 "端口已被占用" | P1 |
| SRV-04 | 正常 — 指定目录 | `bws serve -d D:\bws-data` | 从指定目录读取配置并启动服务 | P1 |

---

## 9. config 命令测试

| ID | 场景 | 输入 | 预期 | 优先级 |
|----|------|------|------|--------|
| CFG-01 | 正常 — 查看所有配置 | `bws config show` | 显示所有配置项及当前值 | P0 |
| CFG-02 | 正常 — 获取配置项 | `bws config get repo-path` | 显示该配置项的值 | P0 |
| CFG-03 | 正常 — 设置配置项 | `bws config set repo-path D:\browsers` | 保存配置 | P0 |
| CFG-04 | 正常 — 设置数据源开关 | `bws config set omaha-source false` | 禁用 Omaha 源 | P1 |
| CFG-05 | 异常 — 配置项不存在 | `bws config get not-exist` | 报错 "未知配置项" | P1 |
| CFG-06 | 异常 — 值类型错误 | `bws config set disk-threshold abc` | 报错 "值类型错误" | P2 |

---

## 10. profile 命令测试

| ID | 场景 | 输入 | 预期 | 优先级 |
|----|------|------|------|--------|
| PROF-01 | 正常 — 列出 Profile | `bws profile list` | 显示所有 Profile | P1 |
| PROF-02 | 正常 — 按浏览器筛选 | `bws profile list chrome` | 只显示 chrome 的 Profile | P1 |
| PROF-03 | 正常 — 查看路径 | `bws profile path` | 显示 Profile 存储路径 | P1 |
| PROF-04 | 正常 — 重置 Profile | `bws profile reset myprofile` | 清空该 Profile 数据 | P2 |
| PROF-05 | 异常 — Profile 不存在 | `bws profile reset notexist` | 报错 "Profile 不存在" | P2 |

---

## 11. cache 命令测试

| ID | 场景 | 输入 | 预期 | 优先级 |
|----|------|------|------|--------|
| CACHE-01 | 正常 — 查看缓存 | `bws cache info` | 显示缓存大小和路径 | P2 |
| CACHE-02 | 正常 — 清空缓存 | `bws cache clear` | 删除所有缓存文件 | P2 |

---

## 12. help 命令测试

| ID | 场景 | 输入 | 预期 | 优先级 |
|----|------|------|------|--------|
| HELP-01 | 正常 — 主帮助 | `bws help` | 显示主帮助内容 | P0 |
| HELP-02 | 正常 — 命令帮助 | `bws help ls` | 显示 ls 命令的详细帮助 | P0 |
| HELP-03 | 正常 — 主题帮助 | `bws help sources` | 显示数据源说明 | P0 |
| HELP-04 | 正常 — 快速帮助 | `bws ls --help` | 显示 ls 命令的快速帮助 | P0 |
| HELP-05 | 异常 — 未知主题 | `bws help notexist` | 提示可用主题列表 | P1 |

---

## 13. alias / use 命令测试

| ID | 场景 | 输入 | 预期 | 优先级 |
|----|------|------|------|--------|
| ALIAS-01 | 正常 — 设置默认版本 | `bws use gc@120` | 设置 gc@120 为默认版本 | P1 |
| ALIAS-02 | 正常 — 列出别名 | `bws alias list` | 显示所有别名 | P2 |
| ALIAS-03 | 正常 — 添加别名 | `bws alias add stable120 gc@120` | 创建别名 | P2 |
| ALIAS-04 | 正常 — 删除别名 | `bws alias remove stable120` | 删除别名 | P2 |

---

## 14. download 命令测试

| ID | 场景 | 输入 | 预期 | 优先级 |
|----|------|------|------|--------|
| DL-01 | 正常 — 下载到指定路径 | `bws download ff@101 -o /output` | 下载安装包到指定目录 | P1 |
| DL-02 | 正常 — 默认路径下载 | `bws download ff@101` | 下载到仓库目录 | P1 |
| DL-03 | 异常 — 版本不存在 | `bws download ff@999` | 报错 "版本未找到" | P1 |

---

## 15. info / doctor 命令测试

| ID | 场景 | 输入 | 预期 | 优先级 |
|----|------|------|------|--------|
| INFO-01 | 正常 — 查看版本信息 | `bws info gc@120` | 显示该版本的详细信息 | P2 |
| DOC-01 | 正常 — 系统诊断 | `bws doctor` | 检查环境并输出诊断报告 | P2 |

---

## 16. 全局行为测试

| ID | 场景 | 输入 | 预期 | 优先级 |
|----|------|------|------|--------|
| GLB-01 | 正常 — 版本号输出 | `bws --version` | 显示 `bws version x.x.x` | P0 |
| GLB-02 | 正常 — 根帮助 | `bws` (无参数) | 显示命令列表和帮助 | P0 |
| GLB-03 | 正常 — 命令帮助 | `bws --help` | 显示根帮助 | P0 |
| GLB-04 | 异常 — 未知命令 | `bws notexist` | 报错 "未知命令" 并提示可用命令 | P1 |
| GLB-05 | 异常 — 未知 flag | `bws ls --unknown` | 报错 "未知选项: --unknown" | P1 |
| GLB-06 | 边界 — 启动信息精简 | `bws ls` | 只显示版本号和分隔线，无多余信息 | P0 |

---

## 17. 数据源相关测试

| ID | 场景 | 输入 | 预期 | 优先级 |
|----|------|------|------|--------|
| SRC-01 | 正常 — serve 源优先 | `bws ls -R gc` (serve 可用) | 优先从 serve 获取 | P0 |
| SRC-02 | 正常 — Omaha 源查询 Chrome | `bws ls -R gc` (serve 不可用) | 从 Omaha 获取 Chrome 版本 | P0 |
| SRC-03 | 正常 — Omaha 源查询 Chromium | `bws ls -R cm` | 从 Omaha 获取 Chromium 版本 | P0 |
| SRC-04 | 正常 — Firefox 源 | `bws ls -R ff` | 从 Mozilla API 获取 Firefox 版本 | P0 |
| SRC-05 | 正常 — 按浏览器过滤源 | `bws ls -R ff` | 不查询 Omaha 源（Firefox 不支持） | P0 |
| SRC-06 | 正常 — 源开关生效 | `bws config set omaha-source false` | 查询时不再访问 Omaha 源 | P1 |
| SRC-07 | 边界 — 所有源禁用 | `bws ls -R gc` (所有源禁用) | 提示 "没有可用的远程源" | P1 |

---

## 18. 自动化测试脚本

### 18.1 快速回归脚本

```bash
#!/bin/bash
# test-smoke.sh — 快速冒烟测试
set -e

echo "=== 冒烟测试开始 ==="

# 帮助命令
bws --version
bws --help
bws help
bws help ls

# 本地列表
bws ls

# 远程列表（需要网络）
bws ls -R ff -n 3
bws ls -R gc -n 3

# 配置
bws config show
bws repo path

echo "=== 冒烟测试通过 ==="
```

### 18.2 完整回归脚本

```bash
#!/bin/bash
# test-full.sh — 完整回归测试
# 需要: 网络连接、足够磁盘空间、管理员权限（serve 测试）
set -e

echo "=== 完整测试开始 ==="

# 1. 基础命令
bws --version
bws --help
bws help
bws help ls
bws help install
bws help sources
bws help faq

# 2. 配置管理
bws config show
bws config get repo-path
bws config set disk-threshold 1073741824

# 3. 仓库管理
bws repo path
bws repo scan

# 4. 远程查询（所有浏览器）
bws ls -R gc -n 5
bws ls -R cm -n 5
bws ls -R ff -n 5
bws ls -R ff -c esr -n 5
bws ls -R ff@101

# 5. 安装（下载小版本测试）
# bws install ff@101.0.1

# 6. serve 配置
bws serve

echo "=== 完整测试通过 ==="
```

---

## 19. 测试环境要求

| 环境 | 配置 | 用途 |
|------|------|------|
| Windows 10/11 | amd64 | 主测试平台 |
| Windows + WSL | amd64 | Linux 兼容性测试 |
| macOS | arm64/amd64 | macOS 兼容性测试 |
| 无网络 | — | 离线场景测试 |
| 内网环境 | 自签名证书 | HTTPS 跳过测试 |

---

## 20. 覆盖率目标

| 模块 | 目标覆盖率 | 当前状态 |
|------|-----------|---------|
| internal/source | >= 80% | 待测量 |
| internal/cli | >= 70% | 待测量 |
| internal/config | >= 80% | 待测量 |
| internal/repo | >= 60% | 待测量 |
| internal/install | >= 60% | 待测量 |
| E2E 场景 | 100% (P0) | 手动验证 |
