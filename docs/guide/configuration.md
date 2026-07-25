# 配置详解

AniaBot 的全部配置存储在**数据库**中（持久化存储的 `ania_kv` 表），通过内置的 **Web 控制面板**查看与修改，不再使用 `config.yaml`。

- 首次启动时自动写入默认配置，并进入[首次设置向导](/guide/web-panel#首次设置向导)。
- 配置在面板中保存后**重启 Bot 生效**。
- 面板操作方式见 [Web 控制面板](/guide/web-panel)。

::: tip 键名约定
配置键为点分路径（大小写不敏感），与历史 `config.yaml` 的层级一一对应，如 `plugin.ai_chat_bot.model`。本文仍以 YAML 形式展示各配置的结构与默认值。
:::

## 引导配置（环境变量）

唯一不经过数据库的配置是**持久化存储本身的位置**（配置中心的载体，必须先于配置加载），通过环境变量设置：

| 变量 | 说明 | 默认值 |
| --- | --- | --- |
| `ANIABOT_STORE_DRIVER` | 持久化驱动：`sqlite` / `mysql` | `sqlite` |
| `ANIABOT_SQLITE_PATH` | SQLite 数据库文件路径 | `./data/aniabot.db` |
| `ANIABOT_MYSQL_DSN` | MySQL 标准 go-sql-driver DSN | 无（驱动为 mysql 时必填） |

其余所有配置（管理员、适配器、缓存、面板、插件）都在数据库中，通过面板编辑。

## bot —— 框架配置

### admin_id

```yaml
bot:
  admin_id: 123456789
```

管理员 QQ 号。拥有最高权限：远程 `/exit` 退出、强制执行定时推送、查看全部定时任务、接收 panic 告警与启动通知等。

### admin_panel —— Web 控制面板

```yaml
bot:
  admin_panel:
    enable: true                # 是否启用面板
    listen: "127.0.0.1:7700"    # 监听地址；改为 0.0.0.0:7700 可局域网访问（面板有密码保护）
```

首次启动会在控制台打印**随机初始密码**（仅显示一次），登录后可在面板右上角修改。详见 [Web 控制面板](/guide/web-panel)。

### adapter —— 协议适配器

WebSocket 与 HTTP **二选一**，取决于 `cmd/main.go` 中创建的适配器：

::: code-group

```yaml [WebSocket（推荐）]
bot:
  adapter:
    # token: xxx        # 若 NapCat 端设置了 access token 则设置该键
    ws:
      address: ws://localhost:4455  # NapCat WebSocket 服务端地址
      max_retries: 5                # 连接失败最大重连次数
      worker_count: 0               # 事件处理线程数，0 = 按 CPU 自动调整
      worker_queue_size: 1024       # 消息队列长度，超出则丢弃
```

```yaml [HTTP]
bot:
  adapter:
    http:
      listen_port: 6679                      # 本地监听端口，接收 NapCat 事件上报
      target_url: http://localhost:6680      # NapCat HTTP 服务端地址
```

:::

::: warning Docker 部署注意
HTTP 模式下 NapCat 向 `localhost` 上报会失败，请将 NapCat 的 HTTP Client 地址改为 AniaBot 所在机器的内网 IP。
:::

### store.cache —— 缓存存储

```yaml
bot:
  store:
    cache:                      # 缓存层：易失，支持 TTL 与列表语义
      driver: memory            # memory（默认，进程内，重启清空）| redis（多实例共享）
      redis:
        address: "localhost:6379"
        password: ""
        db: 0
```

持久化层的位置由上方[环境变量](#引导配置-环境变量)决定，不在此处配置。

## plugin.ai_chat_bot —— AI 对话插件

### 基础配置

```yaml
plugin:
  ai_chat_bot:
    base_url: "https://api.deepseek.com"  # 任意 OpenAI 兼容 API
    api_key: "sk-xxxx"
    model: "deepseek-chat"
    multimodal: false   # 主模型是否支持图片输入
    rate_limit: 2       # 每秒最多调用次数
```

### 模型参数

```yaml
    max_context_tokens: 128000  # 上下文 token 预算，超过 80% 自动压缩历史
    temperature: 1.2
    top_p: 0.9
    top_k: 100
    max_token: 8192
    thinking:
      enable: false      # 深度思考开关
      mode: "auto"       # none / low / medium / high / auto
    prompt: |-
      你是一个ai对话机器人，在QQ上和别人聊天，说话不要长篇大论
```

::: tip 按群/按人定制人格
在面板的「文件编辑 → Prompt 覆盖」页（配置键 `files.prompt_json`，原 `aniabot.prompt.json`），可为特定群聊或好友覆盖 system prompt：

```json
{
  "groups": { "123456": "你是这个群的管理助手..." },
  "friends": { "7891011": "你是我的私人秘书..." }
}
```
:::

### Skill 系统

```yaml
    skills_dir: "./skills"  # Skill 目录
    skills: []              # 指定加载的 skill 名称，空 = 加载全部
```

### 联网搜索

```yaml
    search:
      token: "jina_xxx"   # Jina AI token，用于 web_search / web_explore 工具
```

### OCR 备用识图

主模型不支持多模态时，可配置一个备用视觉模型把图片转述为文字：

```yaml
    ocr:
      enable: false
      base_url: "https://api.siliconflow.cn/v1"
      api_key: ""
      model: "Qwen/Qwen3-VL-8B-Instruct"
      temperature: 0.6
      top_p: 0.95
      top_k: 20
      max_token: 600
      prompt: |-
        你负责将看到的图片用markdown格式描述出来，不要有无关的其他对话
```

### 高危工具（默认关闭）

::: danger 安全提醒
以下工具允许 AI 直接操作宿主机，存在风险，**默认全部关闭**，请确认环境隔离后再开启。
:::

```yaml
    bash:               # 允许 AI 在宿主机执行 shell 命令
      enable: false
      shell: "/bin/bash"
      env: []           # 环境变量，如 ["HOME=/root"]
      whitelist: []     # 非空时仅允许匹配这些正则的命令
      blacklist:        # 匹配这些正则的命令被禁止
        - "^mkfs"
        - "^shutdown"

    local_image:        # 允许 AI 读取宿主机本地图片
      enable: false
```

### AI 定时任务（clock）

```yaml
    clock:
      enable: true              # 启用后 AI 可自主创建定时任务
      default_timeout_sec: 120  # 单次触发默认超时
      max_log_entries: 500      # 执行日志保留条数（滚动覆盖）
```

与框架的 `StartCron` 静态任务不同，clock 任务由 AI / 用户**动态创建**、持久化保存、重启不丢。执行日志可在面板「状态总览」页查看。详见 [AI 对话插件](/guide/builtin-plugins#ai-对话插件)。

### AI 长期记忆（memory）

```yaml
    memory:
      enable: true              # 启用后 AI 可通过 memory_save/search/forget 工具管理长期记忆
      max_entries: 200          # 单个会话（群/好友）的记忆条数上限
```

记忆按群聊 / 好友隔离、持久化保存、重启不丢。详见 [AI 对话插件](/guide/builtin-plugins#ai-对话插件)。

## plugin.dailyNews —— 每日新闻插件

```yaml
plugin:
  dailyNews:
    api: "https://60s.viki.moe/v2/60s?encoding=image-proxy"  # 新闻图 API
    cron: "0 18 * * *"    # cron 表达式，每天 18:00 触发
    groups:               # 接收推送的群号列表
      - 123456
      - 7891011
```

## files.mcp_json —— MCP 服务定义

配置键 `files.mcp_json`（面板「文件编辑 → MCP 服务器」页，原 `aniabot.mcp.json`）定义 AI 可用的 MCP Server，支持 stdio / SSE / Streamable HTTP 三种传输：

```json
{
  "servers": [
    {
      "name": "my-server",
      "transport": "stdio",
      "command": "python",
      "args": ["-m", "mcp_server"],
      "env": { "API_KEY": "xxx" },
      "timeout": 30
    },
    {
      "name": "remote-server",
      "transport": "sse",
      "endpoint": "http://localhost:3000",
      "headers": { "Authorization": "Bearer token123" },
      "timeout": 30
    }
  ]
}
```

MCP 工具采用两阶段懒加载：AI 先通过发现工具查看有哪些 MCP 能力，按需加载到当前会话，避免工具描述撑爆上下文。

## 其他环境变量

| 变量 | 说明 |
| --- | --- |
| `MAX_ITERATIONS` | AI Agent 单轮对话的最大工具迭代次数，默认 20 |
