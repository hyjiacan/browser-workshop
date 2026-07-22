---
layout: home

hero:
  name: Browser Workshop
  text: ''
  tagline: Multi-version browser management tool, supporting local import, remote download, version switching, and isolated execution.
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
    details: Install and manage multiple browser versions simultaneously, with quick filtering by version prefix.
  - icon: 📥
    title: Local Import
    details: Automatically identify and import browser versions from directories or archives. Supports zip, 7z, tar.gz, tar.bz2, tar.xz, and more. Intelligent filename recognition eliminates the need to manually specify version information.
  - icon: 🌐
    title: Remote Download
    details: Download specified browser versions from official sources (Chrome Omaha protocol).
  - icon: 🔄
    title: Offline Distribution
    details: Built-in `serve` command to set up a LAN browser version distribution service, supporting automatic synchronization.
  - icon: 🔒
    title: Isolated Execution
    details: Each version uses an independent Profile, with no interference between versions. Supports named Profiles.
  - icon: 📱
    title: Portable Mode
    details: Data is stored in the `bws-data/` subdirectory, ready to carry on a USB drive and use plug-and-play.
  - icon: 🖥️
    title: Desktop Shortcuts
    details: One-click creation of desktop shortcuts. Double-click to launch the browser directly. Supports Windows, Linux, and macOS.
  - icon: ⚡
    title: Short Aliases
    details: Supports short aliases such as `gc` (chrome), `ff` (firefox), `cm` (chromium), making input faster.
  - icon: 📊
    title: Detailed Logging
    details: Hierarchical logging system, with file DEBUG + configurable console levels, fully recording all operations.
---
