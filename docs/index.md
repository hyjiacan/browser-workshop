---
layout: home

hero:
  name: Browser Workshop
  text: ''
  tagline: 浏览器多版本隔离运行工具，快速搭建独立测试环境，自由切换版本。
  image:
    src: /logo.png
    alt: Browser Workshop logo
  actions:
    - theme: brand
      text: 快速开始
      link: /guide/getting-started
    - theme: alt
      text: 在 GitHub 上查看
      link: https://github.com/hyjiacan/browser-workshop

features:
  - icon: 📦
    title: 多版本管理
    details: 同时安装和管理多个浏览器版本，版本间完全隔离，互不干扰。
  - icon: 🌐
    title: 灵活的版本来源
    details: 本地目录或压缩包自动识别导入，也可从官方源远程下载，并自动识别系统已安装的版本。
  - icon: 🔒
    title: 隔离运行
    details: 每个版本使用独立的 Profile，支持命名 Profile 在不同版本间共享。
  - icon: 🔄
    title: 离线分发
    details: 内置 serve 命令，一键搭建局域网分发服务，内网也能获取浏览器与驱动。
  - icon: 🤖
    title: 自动化集成
    details: 一条命令启动浏览器并暴露 CDP / WebDriver 端点，自动准备与版本匹配的 chromedriver。
  - icon: 🗂️
    title: 后台实例管理
    details: run --daemon 启动、ps 查看、stop 停止，构成可脚本化的生命周期闭环。
---