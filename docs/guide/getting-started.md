# 快速开始

本指南带你在 5 分钟内从零跑起一个 AniaBot 机器人。

## 前置准备

| 依赖 | 版本要求 | 说明 |
| --- | --- | --- |
| Go | **1.25+** | 见 `go.mod`，交叉编译友好 |
| NapCat | 任意近期版本 | QQ 协议端，提供 OneBot v11 接口 |
| Redis | 可选 | 缓存后端之一；默认使用内存缓存，无需安装 |
| LLM API Key | 可选 | 启用 AI 对话时需要（DeepSeek / OpenAI 等） |

::: tip 想用 Redis？
默认缓存驱动为 `memory`（零依赖，重启清空）。在面板「配置管理」页将 `bot.store.cache.driver` 改为 `redis` 并填写地址即可，多实例部署时建议切换。
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

## 第三步：启动与配置

```bash
go run cmd/main.go
```

AniaBot 的配置存储在数据库中，**首次启动**会自动写入默认配置，并在控制台打印 Web 控制面板的**随机初始密码**（仅显示一次）：

```
============================================================
  Web 控制面板初始密码（仅显示一次，登录后可修改）:
    Xx9aBcDeFg
============================================================
```

使用该密码登录 `http://127.0.0.1:7700`，首次登录会自动进入**设置向导**：依次填写 NapCat 连接地址、管理员 QQ 与 AI 模型配置（Base URL / API Key / 模型），保存后一键重启即可生效。

完整配置项说明见 [配置详解](/guide/configuration)，面板使用见 [Web 控制面板](/guide/web-panel)。

## 第四步：开始使用

看到插件注册日志后，机器人就已上线：

- 私聊机器人发送 `/help` —— 查看已加载插件
- 群里 **@机器人 发送 `/help`** —— 群聊版帮助
- **@机器人 随便说点什么** —— 开始 AI 对话 🎉

启动成功后，系统插件会自动私聊管理员发送「AniaBot启动成功」。

## 构建发布

```bash
make linux     # 交叉编译 Linux amd64 → build/AniaBot
make windows   # 编译 Windows → build/AniaBot.exe
make web       # 重新构建 Web 面板前端（修改 web/ 后需要）
make clean     # 清理 build/
```

所有存储后端均为纯 Go 实现（无 CGO），面板前端产物已随仓库提交并嵌入二进制，交叉编译开箱即用。

## 目录结构速览

```
AniaBot/
├── cmd/main.go            # 入口：注册适配器工厂与插件、启动（适配器按 bot.adapter.mode 创建）
├── web/                   # Web 控制面板前端（Vite + Vue3 + Tailwind）
├── common/                # 公共接口：plugin / bot / msgchain / storage
├── bot/
│   ├── core/              # 核心：插件生命周期、事件分发、DI 注入、配置中心
│   ├── adminpanel/        # Web 控制面板后端（配置/状态 API + 内嵌前端）
│   ├── adapter/napcat/    # NapCat WebSocket / HTTP 适配器
│   ├── component/         # AI 引擎：aichat / llmtool / functool
│   ├── plugins/           # 六个内置插件
│   └── utils/             # 命令解析、消息提取等工具
└── custom/                # 自定义插件示例与模板
```

配置存于数据库（默认 `./data/aniabot.db`，可用环境变量调整，见 [配置详解](/guide/configuration#引导配置-环境变量)）。

## 下一步

- [配置详解](/guide/configuration) —— 每一个配置项的含义
- [内置插件](/guide/builtin-plugins) —— 各插件的命令与用法
- [第一个插件](/plugin/first-plugin) —— 开始写你自己的功能
