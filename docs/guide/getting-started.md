# 快速开始

本指南带你在 5 分钟内从零跑起一个 AniaBot 机器人。

## 前置准备

| 依赖 | 版本要求 | 说明 |
| --- | --- | --- |
| Go | **1.26+** | 见 `go.mod`，交叉编译友好 |
| NapCat | 任意近期版本 | QQ 协议端，提供 OneBot v11 接口 |
| Redis | 可选 | 默认缓存后端；没有可改用内存模式 |
| LLM API Key | 可选 | 启用 AI 对话时需要（DeepSeek / OpenAI 等） |

::: tip 没有 Redis？
将 `bot.store.cache.driver` 改为 `memory` 即可零依赖运行，代价是重启后缓存丢失。
:::

## 第一步：部署 NapCat

AniaBot 不直接实现 QQ 协议，而是通过 [NapCat](https://napneko.github.io/) 连接 QQ。请先按照 NapCat 官方文档完成部署并登录你的机器人 QQ 号，然后开放网络接口（二选一）：

- **WebSocket 服务端**：端口如 `4455`（推荐，事件推送更及时）
- **HTTP**：开放 HTTP 服务端（如 `6680`）并配置 HTTP 客户端上报到 AniaBot 的监听地址

## 第二步：获取源码

```bash
git clone https://github.com/jeanhua/AniaBot.git
cd AniaBot
go mod tidy
```

## 第三步：配置

编辑 `config.yaml`，最关键的三处：

```yaml
bot:
  admin_id: 123456789          # ① 你的 QQ 号（管理员）
  adapter:
    ws:
      address: ws://localhost:4455   # ② NapCat 的 WebSocket 地址

plugin:
  ai_chat_bot:
    base_url: "https://api.deepseek.com"
    api_key: "sk-xxxx"         # ③ 你的 LLM API Key
    model: "deepseek-chat"
```

::: warning 开发环境提示
框架会**优先加载 `config.dev.yaml`**，找不到才回退到 `config.yaml`。日常开发建议复制一份 `config.dev.yaml` 用于本地调试，避免改动生产配置。
:::

完整配置项说明见 [配置详解](/guide/configuration)。

## 第四步：启动

```bash
go run cmd/main.go
```

看到插件注册日志后，机器人就已上线：

- 私聊机器人发送 `/help` —— 查看已加载插件
- 群里 **@机器人 发送 `/help`** —— 群聊版帮助
- **@机器人 随便说点什么** —— 开始 AI 对话 🎉

启动成功后，系统插件会自动私聊管理员发送「AniaBot启动成功」。

## 构建发布

```bash
make linux     # 交叉编译 Linux amd64 → build/AniaBot
make windows   # 编译 Windows → build/AniaBot.exe
make clean     # 清理 build/
```

所有存储后端均为纯 Go 实现（无 CGO），交叉编译开箱即用。

## 目录结构速览

```
AniaBot/
├── cmd/main.go            # 入口：创建适配器、注册插件、启动
├── config.yaml            # 生产配置
├── config.dev.yaml        # 开发配置（优先加载）
├── aniabot.mcp.json       # MCP Server 定义
├── common/                # 公共接口：plugin / bot / msgchain / storage
├── bot/
│   ├── core/              # 核心：插件生命周期、事件分发、DI 注入
│   ├── adapter/napcat/    # NapCat WebSocket / HTTP 适配器
│   ├── component/         # AI 引擎：aichat / llmtool / functool
│   ├── plugins/           # 六个内置插件
│   └── utils/             # 命令解析、消息提取等工具
└── custom/                # 自定义插件示例与模板
```

## 下一步

- [配置详解](/guide/configuration) —— 每一个配置项的含义
- [内置插件](/guide/builtin-plugins) —— 各插件的命令与用法
- [第一个插件](/plugin/first-plugin) —— 开始写你自己的功能
