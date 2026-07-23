#!/usr/bin/env python3
# browser-alias.py: 为浏览器添加固定别名和启动参数
#
# IPC 插件示例。bws 通过 stdin 发送 JSON 上下文，通过 stdout 读取 JSON 响应。
#
# 安装: bws plugin install ./plugins/examples/browser-alias.py
# 使用: bws r chrome@120 --plugin browser-alias.py

import sys
import json

# 读取 bws 发送的上下文
req = json.loads(sys.stdin.read())
resp = {}

browser = req.get("browser", "")
version = req.get("version", "")

# 根据浏览器类型添加固定参数
if browser in ("chrome", "chromium"):
    resp["extraArgs"] = [
        "--disable-background-timer-throttling",
        "--disable-renderer-backgrounding",
    ]
elif browser == "firefox":
    resp["extraArgs"] = []

# 输出响应
print(json.dumps(resp))