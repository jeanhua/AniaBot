import { defineConfig } from 'vitepress'

export default defineConfig({
  title: 'AniaBot',
  description: '一个插件驱动型 QQ 机器人框架',
  lang: 'zh-CN',
  base: '/AniaBot/',

  head: [
    ['link', { rel: 'icon', href: '/AniaBot/logo.png' }],
  ],

  themeConfig: {
    logo: '/AniaBot/logo.png',
    siteTitle: 'AniaBot',

    nav: [
      { text: '指南', link: '/guide/introduction', activeMatch: '/guide/' },
      { text: '插件开发', link: '/plugin/overview', activeMatch: '/plugin/' },
      { text: 'API 参考', link: '/api/events', activeMatch: '/api/' },
      {
        text: '链接',
        items: [
          { text: '主分支', link: 'https://github.com/jeanhua/AniaBot' },
          { text: '部署分支 (示例插件)', link: 'https://github.com/jeanhua/AniaBot/tree/dev/deploy' },
          { text: '提交 Issue', link: 'https://github.com/jeanhua/AniaBot/issues' },
        ],
      },
    ],

    sidebar: {
      '/guide/': [
        {
          text: '开始',
          items: [
            { text: '项目介绍', link: '/guide/introduction' },
            { text: '快速开始', link: '/guide/getting-started' },
            { text: '配置说明', link: '/guide/configuration' },
            { text: '内置插件', link: '/guide/builtin-plugins' },
            { text: '部署分支插件', link: '/guide/deploy-plugins' },
          ],
        },
      ],
      '/plugin/': [
        {
          text: '插件开发',
          items: [
            { text: '概览', link: '/plugin/overview' },
            { text: '第一个插件', link: '/plugin/first-plugin' },
            { text: '消息构造器', link: '/plugin/message-builder' },
            { text: '命令解析', link: '/plugin/commands' },
            { text: '数据存储', link: '/plugin/storage' },
            { text: '定时任务', link: '/plugin/cron' },
            { text: '完整示例', link: '/plugin/examples' },
          ],
        },
      ],
      '/api/': [
        {
          text: 'API 参考',
          items: [
            { text: '事件接口', link: '/api/events' },
            { text: '存储接口', link: '/api/storage' },
          ],
        },
      ],
    },

    socialLinks: [
      { icon: 'github', link: 'https://github.com/jeanhua/AniaBot' },
    ],

    footer: {
      message: '基于 MIT 许可证发布',
      copyright: 'Copyright © 2026 jeanhua',
    },

    search: {
      provider: 'local',
    },

    editLink: {
      pattern: 'https://github.com/jeanhua/AniaBot/edit/main/docs/:path',
      text: '在 GitHub 上编辑此页',
    },

    lastUpdated: {
      text: '最后更新于',
    },

    outline: {
      label: '本页目录',
      level: [2, 3],
    },

    docFooter: {
      prev: '上一页',
      next: '下一页',
    },

    darkModeSwitchLabel: '外观',
    lightModeSwitchTitle: '切换到浅色模式',
    darkModeSwitchTitle: '切换到深色模式',
    sidebarMenuLabel: '菜单',
    returnToTopLabel: '返回顶部',
  },
})
