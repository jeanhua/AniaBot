---
layout: home

hero:
  name: AniaBot
  text: 插件驱动型 QQ 机器人框架
  tagline: 基于 Go 语言 · 插件化架构 · 内置 AI 对话 / MCP / 定时任务，几十行代码写出你的第一个插件
  image:
    src: /logo.png
    alt: AniaBot
  actions:
    - theme: brand
      text: 🚀 快速开始
      link: /guide/getting-started
    - theme: alt
      text: 项目介绍
      link: /guide/introduction
    - theme: alt
      text: GitHub
      link: https://github.com/jeanhua/AniaBot

features:
  - icon: ⚡
    title: 高性能
    details: 基于 Go 语言开发，工作池 + 协程模型处理消息事件，轻松应对高并发群聊场景。
  - icon: 🧩
    title: 插件驱动
    details: 一切功能皆为插件。中间件链式执行、生命周期管理、panic 自动恢复，扩展只需实现一个接口。
  - icon: 🤖
    title: AI 对话引擎
    details: 内置完整 Agent 能力：工具调用、MCP 协议、Skill 系统、多模态识图、token 预算上下文窗口。
  - icon: ⏰
    title: AI 定时任务
    details: AI 与用户均可动态创建 cron 任务，持久化保存、重启不丢，到点自动以完整工具链执行。
  - icon: 🔌
    title: 协议兼容
    details: 基于 OneBot v11 协议，支持 NapCat WebSocket / HTTP 双适配器，自由切换接入方式。
  - icon: 💾
    title: 双层存储
    details: 缓存层（Redis / 内存）+ 持久化层（SQLite / MySQL），纯 Go 无 CGO，插件数据自动按命名空间隔离。
---

<HomeSections />
