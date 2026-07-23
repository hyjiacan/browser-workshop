-- workspace.lua: 根据当前工作目录切换浏览器配置
--
-- 功能：检测启动 bws 时所在的目录，如果是特定工作空间，
--       自动加载对应的 Cookie 文件、设置环境变量、添加专用参数。
--
-- 安装: bws plugin install ./workspace.lua
-- 使用: bws r chrome@120 --plugin workspace

function pre_run()
    -- 获取当前工作目录（通过环境变量推断）
    -- 注意：Lua 脚本内无法直接获取 CWD，但可以通过预设环境变量或配置文件实现
    -- 这里演示通过 bws 配置文件读取工作空间映射

    local workspace = ctx.config("workspace") or "default"
    ctx.log("workspace.lua: active workspace = " .. workspace)

    -- 工作空间 A：开发环境
    if workspace == "dev" then
        ctx.add_arg("--auto-open-devtools-for-tabs")
        ctx.set_env("NODE_ENV", "development")
        ctx.log("workspace.lua: dev mode enabled")
        return
    end

    -- 工作空间 B：测试环境
    if workspace == "test" then
        ctx.add_arg("--headless")
        ctx.add_arg("--disable-gpu")
        ctx.set_env("NODE_ENV", "test")
        ctx.log("workspace.lua: test mode enabled")
        return
    end

    -- 工作空间 C：生产环境（严格模式）
    if workspace == "prod" then
        ctx.add_arg("--incognito")
        ctx.add_arg("--no-first-run")
        -- 禁用所有外部通信（仅演示，实际可能过于严格）
        -- ctx.add_arg("--host-resolver-rules=MAP * ~NOTFOUND")
        ctx.log("workspace.lua: prod mode enabled")
        return
    end

    -- 默认：不做特殊处理
    ctx.log("workspace.lua: default mode")
end
