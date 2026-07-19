# 配置详解

AniaBot 使用 [Viper](https://github.com/spf13/viper) 加载 YAML 配置。启动时**优先读取 `config.dev.yaml`**，不存在则回退到 `config.yaml`。

配置分为两大块：`bot`（框架级）与 `plugin`（插件级）。

## bot —— 框架配置

### admin_id

```yaml
bot:
  admin_id: 123456789
```

管理员 QQ 号。拥有最高权限：远程 `/exit` 退出、强制执行定时推送、查看全部定时任务、接收 panic 告警与启动通知等。

### adapter —— 协议适配器

WebSocket 与 HTTP **二选一**，取决于 `cmd/main.go` 中创建的适配器：

::: code-group

```yaml [WebSocket（推荐）]
bot:
  adapter:
    # token: xxx        # 若 NapCat 端设置了 access token 则取消注释
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

### store —— 双层存储

缓存与持久化两层独立配置、互不影响。

```yaml
bot:
  store:
    cache:                      # 缓存层：易失，支持 TTL 与列表语义
      driver: redis             # redis（默认）| memory（进程内，重启清空）
      redis:
        address: "localhost:6379"
        password: ""
        db: 0

    persistent:                 # 持久化层：重启不丢
      driver: sqlite            # sqlite（默认，纯 Go 无 CGO）| mysql
      sqlite:
        path: "./data/aniabot.db"
      mysql:
        dsn: "root:password@tcp(localhost:3306)/aniabot?charset=utf8mb4&parseTime=true&loc=Local"
        max_open_conns: 20          # <=0 时默认 10
        max_idle_conns: 5           # <=0 时默认 5
        conn_max_lifetime_sec: 300  # <=0 时默认 1800
```

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
在项目根目录创建 `aniabot.prompt.json`，可为特定群聊或好友覆盖 system prompt：

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
        - config(\.dev)?\.(yaml|yml|json)
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

与框架的 `StartCron` 静态任务不同，clock 任务由 AI / 用户**动态创建**、持久化保存、重启不丢。详见 [AI 对话插件](/guide/builtin-plugins#ai-对话插件)。

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

## aniabot.mcp.json —— MCP 服务定义

项目根目录下的 `aniabot.mcp.json` 定义 AI 可用的 MCP Server，支持 stdio / SSE / Streamable HTTP 三种传输：

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

## 环境变量

| 变量 | 说明 |
| --- | --- |
| `MAX_ITERATIONS` | AI Agent 单轮对话的最大工具迭代次数，默认 20 |
