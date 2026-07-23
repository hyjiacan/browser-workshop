-- fingerprint-enhanced.lua: 增强版指纹隔离
-- 安装: bws plugin install ./plugins/examples/fingerprint-enhanced.lua
-- 使用: bws r chrome@120 --plugin fingerprint-enhanced

function pre_run()
    -- 通用 WebRTC 防护
    if ctx.browser == "chrome" or ctx.browser == "chromium" then
        ctx.add_arg("--force-webrtc-ip-handling-policy=disable_non_proxied_udp")
        ctx.add_arg("--enforce-webrtc-local-ip-allowed-check")
        ctx.add_arg("--use-fake-device-for-media-stream")
        ctx.add_arg("--use-fake-ui-for-media-stream")
    end

    -- Firefox: 写入 user.js
    if ctx.browser == "firefox" and ctx.profile_dir ~= "" then
        local prefs = [[
// Fingerprint protection by bws plugin: fingerprint-enhanced
user_pref("privacy.resistFingerprinting", true);
user_pref("privacy.resistFingerprinting.letterboxing", true);
user_pref("media.peerconnection.enabled", false);
user_pref("geo.enabled", false);
user_pref("device.sensors.enabled", false);
user_pref("dom.battery.enabled", false);
]]
        local userjs = ctx.profile_dir .. "/user.js"
        local err = ctx.write_file(userjs, prefs)
        if err ~= nil then
            ctx.log("fingerprint-enhanced: failed to write user.js: " .. err)
        else
            ctx.log("fingerprint-enhanced: Firefox fingerprint prefs applied")
        end
    end
end
