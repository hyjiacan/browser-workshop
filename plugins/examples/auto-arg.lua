-- auto-arg.lua: 根据浏览器类型自动添加启动参数
-- 安装: bws plugin install ./plugins/examples/auto-arg.lua
-- 使用: bws r chrome@120 --plugin auto-arg

function pre_run()
    if ctx.browser == "chrome" or ctx.browser == "chromium" then
        ctx.add_arg("--disable-background-timer-throttling")
        ctx.add_arg("--disable-renderer-backgrounding")
    end
    if ctx.browser == "firefox" then
        ctx.log("auto-arg: firefox detected, adding devtools flag")
    end
end
