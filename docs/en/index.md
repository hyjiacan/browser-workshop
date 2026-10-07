---
layout: home

hero:
  name: Browser Workshop
  text: ''
  tagline: Isolated browser multi-version manager for fast, clean test environment switching.
  image:
    src: /logo.png
    alt: Browser Workshop logo
  actions:
    - theme: brand
      text: Getting Started
      link: /en/guide/getting-started
    - theme: alt
      text: View on GitHub
      link: https://github.com/hyjiacan/browser-workshop

features:
  - icon: 📦
    title: Multi-version Management
    details: Install and manage multiple browser versions side by side, fully isolated from each other.
  - icon: 🌐
    title: Flexible Version Sources
    details: Auto-detect and import from local directories or archives, download from official sources, and detect browsers already installed on the system.
  - icon: 🔒
    title: Isolated Execution
    details: Each version runs with its own profile, with named profiles shareable across versions.
  - icon: 🔄
    title: Offline Distribution
    details: A built-in serve command sets up a LAN distribution service, so intranets can fetch browsers and drivers too.
  - icon: 🤖
    title: Automation Integration
    details: One command launches a browser and exposes CDP / WebDriver endpoints, preparing a version-matched chromedriver automatically.
  - icon: 🗂️
    title: Background Instance Management
    details: Start with run --daemon, inspect with ps, stop with stop — a scriptable lifecycle.
---