# Browser Workshop 命令测试计划

> 版本: 0.2.0 | 更新日期: 2026-07-24
> 仓库: https://github.com/hyjiacan/browser-workshop | https://gitee.com/hyjiacan/browser-workshop

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
| INS-01 | 正常 — 安装最新版 | `bws i ff@latest` | 下载并安装最新稳定版 | P0 |
| INS-02 | 正常 — 安装指定版本 | `bws i ff@101.0.1` | 下载并安装该版本 | P0 |
| INS-03 | 正常 — 版本前缀安装 | `bws i gc@120` | 安装 120.x 最新匹配版 | P0 |
| INS-04 | 正常 — 使用短别名 | `bws i ff@latest` | 与全称效果一致 | P0 |
| INS-05 | 正常 — 指定渠道 | `bws i ff@latest -c esr` | 安装 ESR 版本 | P1 |
| INS-06 | 正常 — 指定平台 | `bws i gc@120 -p mac` | 下载 mac 版本 | P1 |
| INS-07 | 正常 — 指定架构 | `bws i gc@120 -a arm64` | 下载 arm64 版本 | P1 |
| INS-08 | 正常 — 从目录安装 | `bws i -d /path/to/dir` | 识别并安装目录中的浏览器 | P1 |
| INS-09 | 正常 — 从文件安装 | `bws i --from-file /path/to.zip` | 解压并安装 | P1 |
| INS-10 | 正常 — 强制覆盖 | `bws i ff@101 -f` | 覆盖已存在的同名版本 | P1 |
| INS-11 | 异常 — 版本不存在 | `bws i ff@999.999` | 报错 "版本未找到" | P1 |
| INS-12 | 异常 — 磁盘空间不足 | `bws i ff@latest` (空间不足) | 提示确认或报错 | P1 |
| INS-13 | 异常 — 无网络 | `bws i ff@latest` (断网) | 报错 "网络连接失败" | P1 |
| INS-14 | 边界 — 重复安装（无 -f） | `bws i ff@101` (已存在) | 提示已存在，跳过或询问 | P1 |

---

## 4. run 命令测试

| ID | 场景 | 输入 | 预期 | 优先级 |
|----|------|------|------|--------|
| RUN-01 | 正常 — 运行指定版本 | `bws r gc@120` | 启动 Chrome 120 | P0 |
| RUN-02 | 正常 — 隐身模式 | `bws r gc@120 -i` | 以隐身模式启动 | P1 |
| RUN-03 | 正常 — 指定 Profile | `bws r gc@120 -p myprofile` | 使用指定 Profile 启动 | P1 |
| RUN-04 | 正常 — 无头模式 | `bws r gc@120 -H` | 以无头模式启动 | P1 |
| RUN-05 | 正常 — 新窗口 | `bws r gc@120 -w` | 打开新窗口 | P2 |
| RUN-06 | 正常 — 传递浏览器参数 | `bws r gc@120 -- https://example.com` | 启动并打开指定 URL | P1 |
| RUN-07 | 正常 — 试运行 | `bws r gc@120 --dry-run` | 打印启动命令，不执行 | P2 |
| RUN-08 | 异常 — 版本未安装 | `bws r gc@999` | 报错 "版本未安装" | P1 |
| RUN-09 | 异常 — 不支持的浏览器 | `bws r safari@latest` | 报错 "不支持的浏览器" | P1 |

---

## 5. uninstall / remove 命令测试

| ID | 场景 | 输入 | 预期 | 优先级 |
|----|------|------|------|--------|
| UNI-01 | 正常 — 卸载指定版本 | `bws rm ff@101` | 删除该版本文件和配置 | P0 |
| UNI-02 | 正常 — 使用 rm 别名 | `bws rm ff@101` | 与 uninstall 效果一致 | P0 |
| UNI-03 | 正常 — 使用 remove 别名 | `bws remove ff@101` | 与 uninstall 效果一致 | P0 |
| UNI-04 | 异常 — 版本未安装 | `bws rm ff@999` | 报错 "版本未安装" | P1 |
| UNI-05 | 异常 — 版本正在运行 | `bws rm ff@101` (运行中) | 提示占用或强制卸载 | P2 |

---

## 6. install -d 本地目录安装测试

| ID | 场景 | 输入 | 预期 | 优先级 |
|----|------|------|------|--------|
| IMP-01 | 正常 — 从目录安装 | `bws i -d /path/to/browser-dir` | 识别并安装目录中的浏览器 | P0 |
| IMP-02 | 正常 — 强制覆盖 | `bws i -d /path/to/browser-dir -f` | 覆盖已存在的版本 | P1 |
| IMP-03 | 异常 — 目录不存在 | `bws i -d /not/exist` | 报错 "目录不存在" | P1 |
| IMP-04 | 异常 — 空目录 | `bws i -d /empty/dir` | 提示 "未找到可安装的文件" | P1 |
| IMP-05 | 边界 — 大目录安装 | `bws i -d /very/large/dir` | 显示流式进度，不卡住 | P1 |

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
| SRV-01 | 首次运行 — 创建配置 | `bws sv`（无 bws-serve.ini） | 创建默认配置文件，提示编辑后重新运行 | P0 |
| SRV-02 | 正常 — 启动服务 | `bws sv`（已有 bws-serve.ini） | 启动 HTTP 服务，监听配置端口 | P0 |
| SRV-03 | 异常 — 端口占用 | `bws sv`（端口占用） | 报错 "端口已被占用" | P1 |
| SRV-04 | 正常 — 指定目录 | `bws sv -d D:\bws-data` | 从指定目录读取配置并启动服务 | P1 |

---

## 9. config 命令测试

| ID | 场景 | 输入 | 预期 | 优先级 |
|----|------|------|------|--------|
| CFG-01 | 正常 — 查看所有配置 | `bws cfg show` | 显示所有配置项及当前值 | P0 |
| CFG-02 | 正常 — 获取配置项 | `bws cfg get repo-path` | 显示该配置项的值 | P0 |
| CFG-03 | 正常 — 设置配置项 | `bws cfg set repo-path D:\browsers` | 保存配置 | P0 |
| CFG-04 | 正常 — 设置数据源开关 | `bws cfg set omaha-source false` | 禁用 Omaha 源 | P1 |
| CFG-05 | 异常 — 配置项不存在 | `bws cfg get not-exist` | 报错 "未知配置项" | P1 |
| CFG-06 | 异常 — 值类型错误 | `bws cfg set disk-threshold abc` | 报错 "值类型错误" | P2 |

---

## 10. profile 命令测试

| ID | 场景 | 输入 | 预期 | 优先级 |
|----|------|------|------|--------|
| PROF-01 | 正常 — 列出 Profile | `bws pf list` | 显示所有 Profile | P1 |
| PROF-02 | 正常 — 按浏览器筛选 | `bws pf list chrome` | 只显示 chrome 的 Profile | P1 |
| PROF-03 | 正常 — 查看路径 | `bws pf path` | 显示 Profile 存储路径 | P1 |
| PROF-04 | 正常 — 重置 Profile | `bws pf reset myprofile` | 清空该 Profile 数据 | P2 |
| PROF-05 | 异常 — Profile 不存在 | `bws pf reset notexist` | 报错 "Profile 不存在" | P2 |

---

## 11. cache 命令测试

| ID | 场景 | 输入 | 预期 | 优先级 |
|----|------|------|------|--------|
| CACHE-01 | 正常 — 查看缓存 | `bws cc info` | 显示缓存大小和路径 | P2 |
| CACHE-02 | 正常 — 清空缓存 | `bws cc clear` | 删除所有缓存文件 | P2 |

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
| ALIAS-01 | 正常 — 设置默认版本 | `bws u gc@120` | 设置 gc@120 为默认版本 | P1 |
| ALIAS-02 | 正常 — 列出别名 | `bws alias list` | 显示所有别名 | P2 |
| ALIAS-03 | 正常 — 添加别名 | `bws alias add stable120 gc@120` | 创建别名 | P2 |
| ALIAS-04 | 正常 — 删除别名 | `bws alias remove stable120` | 删除别名 | P2 |

---

## 14. download 命令测试

| ID | 场景 | 输入 | 预期 | 优先级 |
|----|------|------|------|--------|
| DL-01 | 正常 — 下载到指定路径 | `bws dl ff@101 -o /output` | 下载安装包到指定目录 | P1 |
| DL-02 | 正常 — 默认路径下载 | `bws dl ff@101` | 下载到仓库目录 | P1 |
| DL-03 | 异常 — 版本不存在 | `bws dl ff@999` | 报错 "版本未找到" | P1 |

---

## 15. info / doctor 命令测试

| ID | 场景 | 输入 | 预期 | 优先级 |
|----|------|------|------|--------|
| INFO-01 | 正常 — 查看版本信息 | `bws show gc@120` | 显示该版本的详细信息 | P2 |
| DOC-01 | 正常 — 系统诊断 | `bws dt` | 检查环境并输出诊断报告 | P2 |

---

## 16. Plugin 命令测试

### 16.1 插件管理命令

| ID | 场景 | 输入 | 预期 | 优先级 |
|----|------|------|------|--------|
| PLG-01 | 正常 — 列出已安装插件 | `bws pl list` | 显示已安装插件列表 | P1 |
| PLG-02 | 正常 — 使用 ls 别名 | `bws pl ls` | 与 list 效果一致 | P1 |
| PLG-03 | 正常 — 搜索远程插件 | `bws pl search adblock` | 从注册表搜索并显示匹配插件 | P1 |
| PLG-04 | 正常 — 使用 find 别名 | `bws pl find proxy` | 与 search 效果一致 | P1 |
| PLG-05 | 正常 — 从本地文件安装 Lua 插件 | `bws pl i ./my-plugin.lua` | 安装插件并记录 manifest | P1 |
| PLG-06 | 正常 — 从 Registry 安装插件 | `bws pl i auto-arg` | 从远程注册表下载并安装 | P1 |
| PLG-07 | 正常 — 卸载插件 | `bws pl rm my-plugin` | 删除插件文件和 manifest | P1 |
| PLG-08 | 正常 — 使用 install 别名 | `bws pl add auto-arg` | 与 install 效果一致 | P1 |
| PLG-09 | 异常 — 插件不存在 | `bws pl i notexist` | 报错 "插件未找到" | P1 |
| PLG-10 | 异常 — 插件已安装 | `bws pl i auto-arg` (已安装) | 提示已存在 | P1 |

### 16.2 插件执行 — run 命令集成

| ID | 场景 | 输入 | 预期 | 优先级 |
|----|------|------|------|--------|
| PLG-R-01 | 正常 — 加载单个插件运行 | `bws r ff@latest --plugin auto-arg` | 插件 pre-run 钩子修改启动参数 | P1 |
| PLG-R-02 | 正常 — 加载多个插件 | `bws r ff@latest --plugin auto-arg,workspace` | 两个插件依次执行 | P1 |
| PLG-R-03 | 正常 — Lua 插件修改参数 | `bws r ff@latest --plugin fingerprint-enhanced` | 插件添加额外启动参数 | P1 |
| PLG-R-04 | 正常 — IPC 插件执行 | `bws r ff@latest --plugin browser-alias` | 外部进程插件正常通信 | P1 |
| PLG-R-05 | 异常 — 插件未安装 | `bws r ff@latest --plugin notexist` | 报错 "插件未安装" | P1 |
| PLG-R-06 | 边界 — 插件执行失败 | `bws r ff@latest --plugin broken-plugin` | 优雅跳过，不阻塞主流程 | P1 |

### 16.3 插件注册表

| ID | 场景 | 输入 | 预期 | 优先级 |
|----|------|------|------|--------|
| PLG-REG-01 | 正常 — 注册表缓存 | 首次安装后查询 | 注册表缓存 24 小时 | P1 |
| PLG-REG-02 | 正常 — 本地文件安装 | `bws pl i /path/to/plugin.lua` | 从本地文件安装 | P1 |
| PLG-REG-03 | 异常 — 注册表不可用 | `bws pl search xxx` (断网) | 使用缓存或提示不可用 | P2 |

---

## 17. i18n / 语言设置测试

| ID | 场景 | 输入 | 预期 | 优先级 |
|----|------|------|------|--------|
| I18N-01 | 正常 — 查看语言设置 | `bws cfg get language` | 显示当前语言配置 | P1 |
| I18N-02 | 正常 — 设置中文 | `bws cfg set language zh` | 设置后重启生效，输出中文 | P1 |
| I18N-03 | 正常 — 设置英文 | `bws cfg set language en` | 设置后重启生效，输出英文 | P1 |
| I18N-04 | 正常 — 配置中显示语言 | `bws cfg show` | 显示 `界面语言: zh` 或 `language: en` | P1 |
| I18N-05 | 正常 — 环境变量检测 | `LANG=en bws ls` | 自动检测并使用英文 | P1 |
| I18N-06 | 正常 — 外部翻译文件覆盖 | 创建 `<dataDir>/i18n/zh.json` | 自定义翻译覆盖内置翻译 | P1 |
| I18N-07 | 异常 — 不支持的语言 | `bws cfg set language ja` | 报错 "不支持的语言" | P1 |
| I18N-08 | 边界 — i18n 目录不存在 | 删除 i18n 目录后启动 | 自动创建 i18n 目录 | P1 |
| I18N-09 | 边界 — 外部文件格式错误 | 创建无效 JSON 的翻译文件 | 回退到内置翻译，不报错 | P2 |

---

## 18. 智能拼写纠错测试

| ID | 场景 | 输入 | 预期 | 优先级 |
|----|------|------|------|--------|
| TYPO-01 | 正常 — 编辑距离提示 | `bws insall` | 提示 "你是不是想用 "install"? (相似度: 96%)" | P1 |
| TYPO-02 | 正常 — 前缀匹配提示 | `bws confg` | 提示 "你是不是想用 "config"? (相似度: 93%)" | P1 |
| TYPO-03 | 正常 — 交换字符提示 | `bws donwload` | 提示 "你是不是想用 "download"? (相似度: 85%)" | P1 |
| TYPO-04 | 正常 — 短命令别名提示 | `bws isntall` | 提示 "你是不是想用 "install"? (相似度: 90%)" | P1 |
| TYPO-05 | 边界 — 低相似度不提示 | `bws xyzabc` | 报错 "未知命令"，不显示拼写建议 | P1 |
| TYPO-06 | 边界 — 相似度阈值 | 相似度低于 35% 的命令 | 不展示拼写建议 | P1 |
| TYPO-07 | 边界 — 子命令不触发 | `bws cfg set lang` (lang 是未知子命令) | 不触发顶层命令拼写纠错 | P1 |

---

## 19. 指纹隔离测试

| ID | 场景 | 输入 | 预期 | 优先级 |
|----|------|------|------|--------|
| FPR-01 | 正常 — Chrome 随机指纹 | `bws r gc@120 --fingerprint random` | 每次运行生成不同 UA/语言/分辨率组合 | P1 |
| FPR-02 | 正常 — Firefox 随机指纹 | `bws r ff@latest --fingerprint random` | 每次运行生成不同指纹组合 | P1 |
| FPR-03 | 正常 — 无指纹模式 | `bws r gc@120` | 使用原生浏览器配置启动 | P1 |
| FPR-04 | 正常 — 指纹隔离命令行参数 | `bws r gc@120 --fingerprint random` | 浏览器启动参数包含 --user-agent 等 | P1 |
| FPR-05 | 正常 — UA 随机选择 | `bws r gc@120 --fingerprint random` | 从 Windows/Mac/Linux 三套 UA 池中随机 | P1 |
| FPR-06 | 正常 — 分辨率随机选择 | `bws r gc@120 --fingerprint random` | 从 8 种常见分辨率中随机选择 | P1 |
| FPR-07 | 正常 — WebRTC 随机禁用 | `bws r gc@120 --fingerprint random` | 50% 概率禁用 WebRTC | P2 |
| FPR-08 | 正常 — WebGL 随机禁用 | `bws r gc@120 --fingerprint random` | 50% 概率禁用 WebGL | P2 |
| FPR-09 | 正常 — 虚拟媒体设备 | `bws r gc@120 --fingerprint random` | 始终启用虚拟媒体设备 | P2 |
| FPR-10 | 正常 — 语言随机选择 | `bws r gc@120 --fingerprint random` | 从 7 种常见语言中随机选择 | P2 |
| FPR-11 | 正常 — DPR 按平台设置 | `bws r gc@120 --fingerprint random` | Mac 平台 DPR=2.0，其他 1.0 | P2 |

---

## 20. 代理支持测试

| ID | 场景 | 输入 | 预期 | 优先级 |
|----|------|------|------|--------|
| PRX-01 | 正常 — 设置全局代理 | `bws cfg set proxy http://127.0.0.1:8080` | 保存代理配置到全局配置 | P1 |
| PRX-02 | 正常 — 查看代理配置 | `bws cfg get proxy` | 显示当前代理配置 | P1 |
| PRX-03 | 正常 — 运行时指定代理 | `bws r gc@120 --proxy socks5://127.0.0.1:1080` | 浏览器通过 SOCKS5 代理访问 | P1 |
| PRX-04 | 正常 — 禁用代理 | `bws r gc@120 --no-proxy` | 浏览器直连 | P1 |
| PRX-05 | 正常 — 全局代理下载 | `bws ls -R ff` (配置全局代理) | 通过代理查询版本信息 | P1 |
| PRX-06 | 正常 — 代理支持 socks5h | `bws cfg set proxy socks5h://127.0.0.1:1080` | DNS 通过代理解析 | P1 |
| PRX-07 | 异常 — 无效代理 URL | `bws cfg set proxy invalid://url` | 报错 "无效的代理 URL" | P1 |
| PRX-08 | 异常 — 代理不可达 | `bws ls -R ff` (代理不可达) | 超时或报错 "代理连接失败" | P1 |

---

## 21. 归档解压测试（魔数检测）

| ID | 场景 | 输入 | 预期 | 优先级 |
|----|------|------|------|--------|
| ARC-01 | 正常 — 扩展名识别 ZIP | 安装 .zip 文件 | 正确识别并解压 | P0 |
| ARC-02 | 正常 — 扩展名识别 7z | 安装 .7z 文件 | 正确识别并解压 | P0 |
| ARC-03 | 正常 — 扩展名识别 tar.gz | 安装 .tar.gz 文件 | 正确识别并解压 | P0 |
| ARC-04 | 正常 — 扩展名识别 tar.bz2 | 安装 .tar.bz2 文件 | 正确识别并解压（Firefox 格式） | P0 |
| ARC-05 | 正常 — 扩展名识别 tar.xz | 安装 .tar.xz 文件 | 正确识别并解压 | P0 |
| ARC-06 | 正常 — 扩展名识别 tar.zst | 安装 .tar.zst 文件 | 正确识别并解压 | P0 |
| ARC-07 | 正常 — 魔数检测 ZIP | 无扩展名文件（PK 签名） | 通过魔数识别为 ZIP 并解压 | P1 |
| ARC-08 | 正常 — 魔数检测 7z | 无扩展名文件（7z 签名） | 通过魔数识别为 7z 并解压 | P1 |
| ARC-09 | 正常 — 魔数检测 Gzip | 无扩展名文件（gzip 签名） | 通过魔数识别为 gzip 并解压 | P1 |
| ARC-10 | 正常 — 魔数检测 Bzip2 | 无扩展名文件（BZh 签名） | 通过魔数识别为 bzip2 并解压 | P1 |
| ARC-11 | 正常 — 魔数检测 XZ | 无扩展名文件（XZ 签名） | 通过魔数识别为 XZ 并解压 | P1 |
| ARC-12 | 正常 — Firefox 无扩展名下载 | `bws i ff@latest` (下载 URL 无 filename) | 生成带扩展名的文件名，解压成功 | P0 |
| ARC-13 | 正常 — 自解压 EXE | 安装 .exe 自解压包 | 识别为 ZIP 格式并解压 | P1 |
| ARC-14 | 异常 — 不支持格式 | 未知格式文件 | 报错 "不支持的压缩格式" | P1 |
| ARC-15 | 边界 — 嵌套解压 | 包内包含 ZIP 或 7z | 递归解压所有嵌套包 | P1 |
| ARC-16 | 边界 — 大文件限制 | 超过 10GB 解压 | 报错 "解压大小超过限制" | P2 |

---

## 22. 下载文件名生成测试

| ID | 场景 | 输入 | 预期 | 优先级 |
|----|------|------|------|--------|
| DLF-01 | 正常 — 从 URL 路径提取文件名 | `getDownloadFilename("chrome", "120.0.0.0", "https://dl.google.com/chrome_120.zip", "windows")` | 返回 `chrome_120.zip` | P0 |
| DLF-02 | 正常 — 查询参数 URL 生成 Firefox 文件名 | `getDownloadFilename("firefox", "121.0", "https://download.mozilla.org/?product=...", "windows")` | 返回 `firefox-121.0.exe` | P0 |
| DLF-03 | 正常 — 查询参数 URL 生成 Firefox Linux 文件名 | `getDownloadFilename("firefox", "121.0", "https://download.mozilla.org/?product=...", "linux")` | 返回 `firefox-121.0.tar.bz2` | P0 |
| DLF-04 | 正常 — 查询参数 URL 生成 Chrome Windows 文件名 | `getDownloadFilename("chrome", "120.0.0.0", "https://dl.google.com/?..., "windows")` | 返回 `chrome-120.0.0.0.exe` | P0 |
| DLF-05 | 边界 — 空路径段 | `getDownloadFilename("chrome", "120.0.0.0", "https://example.com/", "windows")` | 返回 `chrome-120.0.0.0.exe` | P1 |
| DLF-06 | 边界 — 未知浏览器 | `getDownloadFilename("unknown", "1.0", "https://example.com/dl", "windows")` | 返回 `unknown-1.0.zip` | P1 |

---

## 23. 全局行为测试

| ID | 场景 | 输入 | 预期 | 优先级 |
|----|------|------|------|--------|
| GLB-01 | 正常 — 版本号输出 | `bws --version` | 显示 `bws x.x.x` | P0 |
| GLB-02 | 正常 — 根帮助 | `bws` (无参数) | 显示命令列表和帮助 | P0 |
| GLB-03 | 正常 — 命令帮助 | `bws --help` | 显示根帮助 | P0 |
| GLB-04 | 正常 — 命令帮助 | `bws install --help` | 显示 install 命令详情 | P0 |
| GLB-05 | 异常 — 未知命令（无相似） | `bws xyzabc` | 报错 "未知命令" 并提示可用命令 | P1 |
| GLB-06 | 异常 — 未知命令（有相似） | `bws insall` | 报错并提示 "你是不是想用 "install"?" | P1 |
| GLB-07 | 异常 — 未知 flag | `bws ls --unknown` | 报错 "未知选项: --unknown" | P1 |
| GLB-08 | 边界 — 启动信息精简 | `bws ls` | 只显示版本号和分隔线，无多余信息 | P0 |

---

## 24. 数据源相关测试

| ID | 场景 | 输入 | 预期 | 优先级 |
|----|------|------|------|--------|
| SRC-01 | 正常 — serve 源优先 | `bws ls -R gc` (serve 可用) | 优先从 serve 获取 | P0 |
| SRC-02 | 正常 — Omaha 源查询 Chrome | `bws ls -R gc` (serve 不可用) | 从 Omaha 获取 Chrome 版本 | P0 |
| SRC-03 | 正常 — Omaha 源查询 Chromium | `bws ls -R cm` | 从 Omaha 获取 Chromium 版本 | P0 |
| SRC-04 | 正常 — Firefox 源 | `bws ls -R ff` | 从 Mozilla API 获取 Firefox 版本 | P0 |
| SRC-05 | 正常 — 按浏览器过滤源 | `bws ls -R ff` | 不查询 Omaha 源（Firefox 不支持） | P0 |
| SRC-06 | 正常 — 源开关生效 | `bws cfg set omaha-source false` | 查询时不再访问 Omaha 源 | P1 |
| SRC-07 | 边界 — 所有源禁用 | `bws ls -R gc` (所有源禁用) | 提示 "没有可用的远程源" | P1 |

---

## 25. 自动化测试脚本

### 25.1 快速回归脚本

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
bws help plugin

# 本地列表
bws ls

# 远程列表（需要网络）
bws ls -R ff -n 3
bws ls -R gc -n 3

# 配置
bws cfg show
bws cfg get language
bws repo path

# 拼写纠错（验证 typo 提示功能正常）
bws insall 2>&1 || true
bws confg 2>&1 || true

# 插件列表
bws pl list 2>&1 || true

echo "=== 冒烟测试通过 ==="
```

### 25.2 完整回归脚本

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
bws help i
bws help sources
bws help faq
bws help plugin

# 2. 配置管理
bws cfg show
bws cfg get repo-path
bws cfg get language
bws cfg set language zh
bws cfg set disk-threshold 1073741824
bws cfg set language en
bws cfg set language zh

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
# bws i ff@101.0.1

# 6. 拼写纠错测试
echo "--- 拼写纠错测试 ---"
bws insall 2>&1 || true
bws confg 2>&1 || true
bws donwload 2>&1 || true
bws xyzabc 2>&1 || true

# 7. 插件管理
echo "--- 插件管理 ---"
bws pl list 2>&1 || true
bws pl search auto-arg 2>&1 || true

# 8. 代理配置
echo "--- 代理配置 ---"
bws cfg get proxy 2>&1 || true

# 9. serve 配置
bws sv

# 10. 指纹隔离（dry-run 测试命令生成）
bws r gc@120 --fingerprint random --dry-run 2>&1 || true

echo "=== 完整测试通过 ==="
```

---

## 26. 测试环境要求

| 环境 | 配置 | 用途 |
|------|------|------|
| Windows 10/11 | amd64 | 主测试平台 |
| Windows + WSL | amd64 | Linux 兼容性测试 |
| macOS | arm64/amd64 | macOS 兼容性测试 |
| 无网络 | — | 离线场景测试 |
| 内网环境 | 自签名证书 | HTTPS 跳过测试 |

---

## 27. 覆盖率目标

| 模块 | 目标覆盖率 | 当前状态 |
|------|-----------|---------|
| internal/source | >= 80% | 待测量 |
| internal/cli | >= 70% | 待测量 |
| internal/config | >= 80% | 待测量 |
| internal/repo | >= 60% | 待测量 |
| internal/install | >= 60% | 待测量 |
| internal/plugin | >= 70% | 待测量 |
| internal/archive | >= 80% | 待测量 |
| internal/fingerprint | >= 70% | 待测量 |
| internal/i18n | >= 70% | 待测量 |
| internal/download | >= 60% | 待测量 |
| E2E 场景 | 100% (P0) | 手动验证 |
