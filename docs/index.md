---
layout: home

hero:
  name: AniaBot
  text: 插件驱动型 QQ 机器人框架
  tagline: 基于 Go 语言开发，高性能、模块化、易扩展
  image:
    src: /logo.png
    alt: AniaBot
  actions:
    - theme: brand
      text: 快速开始
      link: /guide/getting-started
    - theme: alt
      text: 查看源码
      link: https://github.com/jeanhua/AniaBot

features:
  - icon: ⚡
    title: 高性能
    details: 基于 Go 语言开发，充分利用并发特性，轻松应对高并发消息场景。
  - icon: 🧩
    title: 插件驱动
    details: 采用插件化架构，功能模块化，每个插件职责单一，易于扩展和维护。
  - icon: 🔌
    title: 协议兼容
    details: 支持 napcat WebSocket/HTTP 等多种 QQ 协议适配器，灵活接入。
  - icon: 🛠️
    title: 开箱即用
    details: 内置 AI 对话、防撤回、复读机、新闻推送等多个实用插件。
  - icon: 📦
    title: 简洁 API
    details: 提供直观的插件接口和消息构造器，几十行代码即可完成一个功能插件。
  - icon: 💾
    title: 双层存储
    details: 缓存层（Redis/内存）+ 持久化层（SQLite/MySQL），均纯 Go 无 CGO，插件数据自动隔离，无需关心命名冲突。
---
