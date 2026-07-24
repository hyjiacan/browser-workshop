# 文件更新清单

本文档用于说明每次需求、代码变更后，需要检查并更新的文件列表。新增功能或修改命令时，请按此清单逐一确认相关文件已同步更新。

## 一、命令变更（新增/修改/删除命令或别名）

当新增、修改或删除 CLI 命令及其别名时，需更新以下文件：

| 优先级 | 文件路径 | 说明 |
|--------|----------|------|
| 高 | `internal/cli/commands.go` | 命令定义（名称、别名、描述、用法、示例、参数） |
| 高 | `internal/help/files/main.txt` | 主帮助页面的命令列表 |
| 高 | `internal/help/files/<命令>.txt` | 该命令的详细帮助文件（新增命令需新建） |
| 高 | `docs/guide/commands.md` | VitePress 命令参考文档 |
| 高 | `docs/en/guide/commands.md` | 英文版命令参考文档 |
| 中 | `docs/guide/getting-started.md` | 快速上手文档中的相关示例 |
| 中 | `docs/en/guide/getting-started.md` | 英文版快速上手文档 |
| 中 | `docs/guide/changelog.md` | 版本变更记录 |
| 中 | `docs/en/guide/changelog.md` | 英文版版本变更记录 |
| 低 | `README.md` | 项目根目录 README |
| 低 | `docs/en/index.md` | 英文版首页（如命令总览有变化） |

## 二、配置项变更（新增/修改/删除配置项）

当新增、修改或删除配置项时，需更新以下文件：

| 优先级 | 文件路径 | 说明 |
|--------|----------|------|
| 高 | `internal/cli/commands.go` | `readableConfigKeys()` / `writableConfigKeys()` 函数 |
| 高 | `internal/cli/commands.go` | `runConfigGet()` / `runConfigSet()` 中的 switch 分支 |
| 高 | `internal/config/config.go` | 配置结构体定义及存取方法 |
| 高 | `internal/help/files/config.txt` | 配置管理帮助文件 |
| 高 | `docs/guide/config.md` | VitePress 配置文档 |
| 高 | `docs/en/guide/config.md` | 英文版配置文档 |
| 中 | `docs/guide/commands.md` | 命令参考中 config 子命令的示例 |
| 中 | `docs/en/guide/commands.md` | 英文版命令参考 |
| 中 | `docs/guide/changelog.md` | 版本变更记录 |

## 三、帮助系统变更（帮助文件内容或结构变更）

当修改帮助文件的内容、结构或新增帮助主题时，需更新以下文件：

| 优先级 | 文件路径 | 说明 |
|--------|----------|------|
| 高 | `internal/help/files/*.txt` | 相关帮助主题文件 |
| 高 | `internal/help/help.go` | `topicDescription()` 函数（新增主题时需添加描述） |
| 中 | `docs/guide/commands.md` | VitePress 中 help 命令的文档 |

## 四、数据源/下载相关变更

当修改数据源行为、下载逻辑或支持的浏览器类型时，需更新以下文件：

| 优先级 | 文件路径 | 说明 |
|--------|----------|------|
| 高 | `internal/source/*.go` | 数据源实现代码 |
| 高 | `internal/help/files/sources.txt` | 数据源说明帮助文件 |
| 高 | `docs/guide/config.md` | 配置文档中的数据源相关说明 |
| 中 | `docs/guide/commands.md` | install/ls/download 命令的示例和说明 |
| 中 | `docs/guide/changelog.md` | 版本变更记录 |

## 五、插件系统变更

当修改插件系统的架构、API 或管理方式时，需更新以下文件：

| 优先级 | 文件路径 | 说明 |
|--------|----------|------|
| 高 | `internal/plugin/*.go` | 插件系统实现代码 |
| 高 | `internal/cli/plugin_commands.go` | 插件 CLI 命令 |
| 高 | `plugins/README.md` | 插件开发文档（极易遗漏！） |
| 高 | `docs/guide/plugin.md` | VitePress 插件文档（含 Registry 注册说明） |
| 高 | `docs/en/guide/plugin.md` | 英文版插件文档（含 Registry 注册说明） |
| 高 | `bws-registry/registry.json` | 独立 Registry 仓库索引文件 |
| 中 | `docs/guide/commands.md` | 命令参考中 plugin 命令的文档 |
| 中 | `docs/guide/changelog.md` | 版本变更记录 |

## 六、Profile/指纹/代理等子系统变更

当修改各子系统功能时，需更新以下文件：

| 子系统 | 高优先级文件 | 中优先级文件 |
|--------|-------------|-------------|
| Profile | `internal/cli/commands.go` (profile 子命令), `internal/help/files/profile.txt` | `docs/guide/profile.md`, `docs/guide/commands.md` |
| 指纹隔离 | `internal/fingerprint/*.go`, `internal/cli/commands.go` (run 命令 --fingerprint) | `docs/guide/fingerprint.md`, `docs/guide/commands.md` |
| 代理 | `internal/cli/commands.go` (run/config), `internal/config/config.go` | `docs/guide/proxy.md`, `docs/guide/commands.md` |
| 缓存 | `internal/cli/commands.go` (cache 子命令), `internal/help/files/cache.txt` | `docs/guide/commands.md` |
| Serve 服务 | `internal/serve/*.go`, `internal/help/files/serve.txt` | `docs/guide/serve.md`, `docs/guide/serve-api.md` |

## 七、多语言/i18n 变更

当修改界面语言、翻译内容或 i18n 架构时，需更新以下文件：

| 优先级 | 文件路径 | 说明 |
|--------|----------|------|
| 高 | `internal/i18n/langs/zh.json` | 中文翻译（内置） |
| 高 | `internal/i18n/langs/template.json` | 语言模板（新增键时必须同步更新） |
| 高 | `internal/i18n/i18n.go` | i18n 核心逻辑 |
| 中 | `docs/guide/config.md` | language 配置项说明 |
| 低 | `docs/en/**` | 英文文档（如新增功能需同步翻译） |

## 八、版本发布时的全量检查清单

每次发布新版本前，请确认以下文件已更新：

- [ ] `main.go` 中的 `version` 常量
- [ ] `docs/guide/changelog.md` 新增版本记录
- [ ] `docs/en/guide/changelog.md` 英文版变更记录
- [ ] `internal/help/files/*.txt` 所有帮助文件内容与实际命令一致
- [ ] `docs/guide/commands.md` 命令总览表完整且别名正确
- [ ] `docs/guide/commands.md` 所有示例代码块后注明别名关系
- [ ] `plugins/README.md` 与实际插件系统实现一致
- [ ] `README.md` 中的功能描述和版本信息准确

## 九、当前已知需修复项

基于本次审查，以下文件存在已知问题，需优先修复：

| 文件 | 问题描述 | 优先级 |
|------|----------|--------|
| `internal/cli/commands.go` | `bws ls --json` 选项已注册但实际输出逻辑未实现 | 高 |
| `internal/source/chrome.go` | Chrome 特定版本下载 URL 生成是 placeholder，实际需 Omaha update check | 中 |
| `internal/plugin/*.go` | 插件 Hooks `post_run`/`pre_install`/`post_install`/`on_exit` 仅定义常量，代码中未实际调用 | 中 |
| `docs/en/guide/commands.md` | 英文版命令参考文档未同步别名注释更新 | 中 |
| `docs/en/guide/getting-started.md` | 英文版快速上手文档未同步 | 低 |

> **已修复项归档**：`internal/help/files/*.txt` 系列帮助文件、`plugins/README.md` 插件路径、`docs/guide/commands.md` 别名注释 等问题已在本轮修复中解决。
