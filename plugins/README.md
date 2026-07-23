# bws 插件开发指南

bws 插件系统允许你通过 Lua 脚本在浏览器启动时自动执行自定义逻辑。

## 插件目录

插件存放在以下目录之一：

- **便携模式**：`bws-data/plugins/`（与 bws 可执行文件同级）
- **用户目录**：`~/.bws/plugins/`

## 快速开始

创建一个 `.lua` 文件即可：

```lua
-- ~/.bws/plugins/hello.lua
function pre_run()
    ctx.log("Hello from plugin! Browser: " .. ctx.browser)
end
```

安装并运行：

```bash
bws plugin install ./hello.lua
bws r chrome@120 --plugin hello
```

## 生命周期钩子

| 钩子 | 触发时机 | 说明 |
|------|---------|------|
| `pre_run` | 浏览器启动前 | 修改启动参数、写入配置文件 |

未来可能扩展：
- `post_run`：浏览器启动后
- `pre_install`：安装前
- `post_install`：安装后

## ctx API 参考

### 只读字段

| 字段 | 类型 | 说明 |
|------|------|------|
| `ctx.browser` | string | 浏览器名称，如 `"chrome"`、`"firefox"`、`"chromium"` |
| `ctx.version` | string | 版本号 |
| `ctx.profile` | string | Profile 名称（用户通过 `--profile` 指定） |
| `ctx.profile_dir` | string | Profile 目录绝对路径 |

### 函数

| 函数 | 参数 | 返回值 | 说明 |
|------|------|--------|------|
| `ctx.config(key)` | string | string | 读取 bws 配置项，如 `ctx.config("proxy")` |
| `ctx.add_arg(arg)` | string | - | 添加浏览器启动参数 |
| `ctx.set_env(key, value)` | string, string | - | 设置环境变量 |
| `ctx.write_file(path, content)` | string, string | nil / string | 写入文件，失败返回错误字符串 |
| `ctx.read_file(path)` | string | string, string | 读取文件，返回 (内容, 错误) |
| `ctx.log(message)` | string | - | 输出日志到 stderr |

## 示例

### 按浏览器添加参数

```lua
function pre_run()
    if ctx.browser == "chrome" or ctx.browser == "chromium" then
        ctx.add_arg("--disable-background-timer-throttling")
        ctx.add_arg("--disable-renderer-backgrounding")
    end
end
```

### 写入 Firefox user.js

```lua
function pre_run()
    if ctx.browser == "firefox" and ctx.profile_dir ~= "" then
        local prefs = [[
user_pref("privacy.resistFingerprinting", true);
user_pref("geo.enabled", false);
]]
        local err = ctx.write_file(ctx.profile_dir .. "/user.js", prefs)
        if err ~= nil then
            ctx.log("failed: " .. err)
        end
    end
end
```

### 读取配置

```lua
function pre_run()
    local proxy = ctx.config("proxy")
    if proxy ~= "" then
        ctx.log("using proxy: " .. proxy)
    end
end
```

## 完整示例

见 [examples/](examples/) 目录：

- `auto-arg.lua` — 按浏览器类型自动添加启动参数
- `fingerprint-enhanced.lua` — 增强版指纹隔离

## 发布插件

1. 创建 GitHub/Gitee 仓库，命名如 `bws-plugin-xxx`
2. 编写插件代码 + README
3. 向官方 Registry 提交 PR（在 [registry.json](https://gitee.com/hyjiacan/bws/blob/master/plugins/registry.json) 中添加条目）

## 限制

- 当前仅支持 `pre_run` 钩子
- 插件按 `--plugin` 指定的顺序依次执行
- Lua 脚本无法直接调用外部进程（安全考虑）

## 未来：IPC 插件

除了 Lua 脚本插件，未来计划支持 **IPC 插件**：
bws 启动一个独立进程，通过 stdin/stdout JSON-RPC 通信。
这意味着插件可以用 **任何编程语言** 编写（Python、Node.js、Go、Rust 等）。

```
bws r chrome@120 --plugin my-python-plugin
# bws 启动: python my-python-plugin.py
# 通过 JSON-RPC 发送上下文，接收修改后的参数
```

IPC 插件适合需要复杂逻辑的场景（如 CDP 操作、截图、自动化测试对接等）。
