import { defineConfig } from 'vitepress'

export default defineConfig({
  title: 'browser-workshop',
  description: '多版本浏览器管理工具，支持本地导入、远程下载、版本切换、隔离运行。',
  lang: 'zh-CN',
  lastUpdated: true,
  cleanUrls: true,
  base: '/browser-workshop/',

  head: [
    ['link', { rel: 'icon', type: 'image/png', href: '/logo.png' }],
  ],

  themeConfig: {
    logo: '/logo.png',
    siteTitle: 'bws',

    nav: [
      { text: '指南', link: '/guide/getting-started' },
      { text: '命令参考', link: '/guide/commands' },
      { text: 'Serve 服务', link: '/guide/serve' },
      { text: 'GitHub', link: 'https://github.com/hyjiacan/bws' },
      { text: 'Gitee', link: 'https://gitee.com/hyjiacan/bws' },
    ],

    sidebar: {
      '/guide/': [
        {
          text: '快速开始',
          items: [
            { text: '介绍', link: '/guide/introduction' },
            { text: '安装', link: '/guide/installation' },
            { text: '快速上手', link: '/guide/getting-started' },
            { text: '浏览器短别名', link: '/guide/short-aliases' },
          ],
        },
        {
          text: '核心功能',
          items: [
            { text: '版本管理', link: '/guide/version-management' },
            { text: '本地导入', link: '/guide/import' },
            { text: '远程下载', link: '/guide/download' },
            { text: '运行浏览器', link: '/guide/run' },
            { text: 'Profile 管理', link: '/guide/profile' },
            { text: '配置管理', link: '/guide/config' },
          ],
        },
        {
          text: 'Serve 服务',
          items: [
            { text: '概述', link: '/guide/serve' },
            { text: '自动同步', link: '/guide/serve-sync' },
            { text: 'API 参考', link: '/guide/serve-api' },
          ],
        },
        {
          text: '参考',
          items: [
            { text: '命令参考', link: '/guide/commands' },
            { text: '数据存储', link: '/guide/data-storage' },
            { text: '日志系统', link: '/guide/logging' },
          ],
        },
      ],
    },

    socialLinks: [
      { icon: 'github', link: 'https://github.com/hyjiacan/bws' },
      { icon: 'git', link: 'https://gitee.com/hyjiacan/bws' },
    ],

    footer: {
      message: 'MIT Licensed',
      copyright: `Copyright © ${new Date().getFullYear()} bws contributors`,
    },

    search: {
      provider: 'local',
    },

    outline: {
      level: [2, 3],
      label: '目录',
    },

    docFooter: {
      prev: '上一页',
      next: '下一页',
    },

    lastUpdated: {
      text: '最后更新',
      formatOptions: {
        dateStyle: 'short',
        timeStyle: 'medium',
      },
    },

    darkModeSwitchLabel: '主题',
    lightModeSwitchTitle: '切换到浅色模式',
    darkModeSwitchTitle: '切换到深色模式',
    sidebarMenuLabel: '菜单',
    returnToTopLabel: '回到顶部',
    langMenuLabel: '语言',
  },
})
