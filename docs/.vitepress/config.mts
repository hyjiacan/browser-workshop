import { defineConfig } from 'vitepress'

// 共享配置
const sharedConfig = {
  lastUpdated: true,
  cleanUrls: true,
  base: '/browser-workshop/',

  head: [
    ['link', { rel: 'icon', type: 'image/png', href: '/logo.png' }],
    ['link', { rel: 'icon', type: 'image/x-icon', href: '/logo.ico' }],
    ['script', { src: 'https://cdn.jsdelivr.net/npm/mermaid@10.6.1/dist/mermaid.min.js' }],
  ],

  markdown: {
    config: (md: any) => {
      const originalFence = md.renderer.rules.fence!
      md.renderer.rules.fence = (tokens: any, idx: number, options: any, env: any, self: any) => {
        const token = tokens[idx]
        if (token.info.trim() === 'mermaid') {
          const escapedContent = token.content
            .replace(/&/g, '&amp;')
            .replace(/"/g, '&quot;')
            .replace(/</g, '&lt;')
            .replace(/>/g, '&gt;')
          return `<div class="mermaid-diagram" data-mermaid="${escapedContent}"></div>`
        }
        return originalFence(tokens, idx, options, env, self)
      }
    },
  },
} as const

// 中文 themeConfig
const zhThemeConfig = {
  logo: '/logo.png',
  siteTitle: 'Browser Workshop',

  nav: [
    { text: '指南', link: '/guide/getting-started' },
    { text: '命令参考', link: '/guide/commands' },
    { text: '插件系统', link: '/guide/plugin' },
    { text: 'Serve 服务', link: '/guide/serve' },
    { text: 'GitHub', link: 'https://github.com/hyjiacan/browser-workshop' },
    { text: 'Gitee', link: 'https://gitee.com/hyjiacan/browser-workshop' },
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
          { text: '代理支持', link: '/guide/proxy' },
          { text: '指纹隔离', link: '/guide/fingerprint' },
          { text: '插件系统', link: '/guide/plugin' },
          { text: '配置管理', link: '/guide/config' },
        ],
      },
      {
        text: 'Serve 服务',
        items: [
          { text: '概述', link: '/guide/serve' },
          { text: '自动同步', link: '/guide/serve-sync' },
          { text: 'API 参考', link: '/guide/serve-api' },
          { text: '团队部署', link: '/guide/team-deploy' },
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
    { icon: 'github', link: 'https://github.com/hyjiacan/browser-workshop' },
    { icon: 'git', link: 'https://gitee.com/hyjiacan/browser-workshop' },
  ],

  footer: {
    message: 'MIT Licensed',
    copyright: `Copyright © ${new Date().getFullYear()} Browser Workshop contributors`,
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
}

// 英文 themeConfig
const enThemeConfig = {
  logo: '/logo.png',
  siteTitle: 'Browser Workshop',

  nav: [
    { text: 'Guide', link: '/en/guide/getting-started' },
    { text: 'Commands', link: '/en/guide/commands' },
    { text: 'Plugin', link: '/en/guide/plugin' },
    { text: 'Serve', link: '/en/guide/serve' },
    { text: 'GitHub', link: 'https://github.com/hyjiacan/browser-workshop' },
    { text: 'Gitee', link: 'https://gitee.com/hyjiacan/browser-workshop' },
  ],

  sidebar: {
    '/en/guide/': [
      {
        text: 'Getting Started',
        items: [
          { text: 'Introduction', link: '/en/guide/introduction' },
          { text: 'Installation', link: '/en/guide/installation' },
          { text: 'Getting Started', link: '/en/guide/getting-started' },
          { text: 'Browser Short Aliases', link: '/en/guide/short-aliases' },
        ],
      },
      {
        text: 'Core Features',
        items: [
          { text: 'Version Management', link: '/en/guide/version-management' },
          { text: 'Local Import', link: '/en/guide/import' },
          { text: 'Remote Download', link: '/en/guide/download' },
          { text: 'Run Browser', link: '/en/guide/run' },
          { text: 'Profile Management', link: '/en/guide/profile' },
          { text: 'Proxy Support', link: '/en/guide/proxy' },
          { text: 'Fingerprint Isolation', link: '/en/guide/fingerprint' },
          { text: 'Plugin System', link: '/en/guide/plugin' },
          { text: 'Configuration', link: '/en/guide/config' },
        ],
      },
      {
        text: 'Serve Service',
        items: [
          { text: 'Overview', link: '/en/guide/serve' },
          { text: 'Auto Sync', link: '/en/guide/serve-sync' },
          { text: 'API Reference', link: '/en/guide/serve-api' },
          { text: 'Team Deployment', link: '/en/guide/team-deploy' },
        ],
      },
      {
        text: 'Reference',
        items: [
          { text: 'Commands', link: '/en/guide/commands' },
          { text: 'Data Storage', link: '/en/guide/data-storage' },
          { text: 'Logging', link: '/en/guide/logging' },
        ],
      },
    ],
  },

  socialLinks: [
    { icon: 'github', link: 'https://github.com/hyjiacan/browser-workshop' },
    { icon: 'git', link: 'https://gitee.com/hyjiacan/browser-workshop' },
  ],

  footer: {
    message: 'MIT Licensed',
    copyright: `Copyright © ${new Date().getFullYear()} Browser Workshop contributors`,
  },

  outline: {
    level: [2, 3],
    label: 'On this page',
  },

  docFooter: {
    prev: 'Previous page',
    next: 'Next page',
  },

  lastUpdated: {
    text: 'Last updated',
    formatOptions: {
      dateStyle: 'short',
      timeStyle: 'medium',
    },
  },

  darkModeSwitchLabel: 'Theme',
  lightModeSwitchTitle: 'Switch to light mode',
  darkModeSwitchTitle: 'Switch to dark mode',
  sidebarMenuLabel: 'Menu',
  returnToTopLabel: 'Return to top',
  langMenuLabel: 'Language',
}

export default defineConfig({
  ...sharedConfig,

  themeConfig: {
    search: {
      provider: 'local',
      options: {
        locales: {
          root: {
            translations: {
              button: {
                buttonText: '搜索文档',
                buttonAriaLabel: '搜索文档',
              },
              modal: {
                noResultsText: '无法找到相关结果',
                resetButtonTitle: '清除查询条件',
                footer: {
                  selectText: '选择',
                  navigateText: '切换',
                  closeText: '关闭',
                },
              },
            },
          },
          en: {
            translations: {
              button: {
                buttonText: 'Search',
                buttonAriaLabel: 'Search',
              },
            },
          },
        },
      },
    },
  },

  locales: {
    root: {
      label: '简体中文',
      lang: 'zh-CN',
      title: 'Browser Workshop',
      description: '多版本浏览器管理工具，支持本地导入、远程下载、版本切换、隔离运行。',
      themeConfig: zhThemeConfig,
    },
    en: {
      label: 'English',
      lang: 'en',
      link: '/en/',
      title: 'Browser Workshop',
      description: 'Multi-version browser management tool supporting local import, remote download, version switching, and isolated execution.',
      themeConfig: enThemeConfig,
    },
  },
})
