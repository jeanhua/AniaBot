# 配置详解

AniaBot 的全部配置存储在**数据库**中（持久化存储的 `ania_kv` 表），通过内置的 **Web 控制面板**查看与修改，不使用任何 yaml 配置文件。

- 首次启动时自动写入默认配置，并进入[首次设置向导](/guide/web-panel#首次设置向导)。
- 配置在面板中保存后**重启 Bot 生效**。
- 面板操作方式见 [Web 控制面板](/guide/web-panel)。

::: tip 键名约定
配置键为点分路径（大小写不敏感），如 `plugin.ai_chat_bot.model`。在面板的「配置编辑」页按键名查找并修改即可；本文各节列出所有键名、默认值与说明。
:::

## 引导配置（环境变量）

唯一不经过数据库的配置是**持久化存储本身的位置**（配置中心的载体，必须先于配置加载），通过环境变量设置：

| 变量 | 说明 | 默认值 |
| --- | --- | --- |
| `ANIABOT_STORE_DRIVER` | 持久化驱动：`sqlite` / `mysql` | `sqlite` |
| `ANIABOT_SQLITE_PATH` | SQLite 数据库文件路径 | `./data/aniabot.db` |
| `ANIABOT_MYSQL_DSN` | MySQL 标准 go-sql-driver DSN | 无（驱动为 mysql 时必填） |

其余所有配置（管理员、适配器、缓存、面板、插件）都在数据库中，通过面板编辑。

## 环境变量覆盖（ANIA 前缀）

数据库中的**任意配置键**都可以用环境变量临时覆盖（优先级高于数据库中的值，不写回数据库），适合容器部署或临时调试。命名规则：`ANIA_` + 配置键全大写、点与横线转为下划线：

| 配置键 | 环境变量 |
| --- | --- |
| `bot.admin_panel.listen` | `ANIA_BOT_ADMIN_PANEL_LISTEN` |
| `bot.adapter.ws.address` | `ANIA_BOT_ADAPTER_WS_ADDRESS` |
| `plugin.ai_chat_bot.api_key` | `ANIA_PLUGIN_AI_CHAT_BOT_API_KEY` |

非字符串类型（int / bool / 数组等）按 JSON 解析，如 `ANIA_BOT_ADMIN_ID=123456789`。覆盖生效时启动日志会打印 `环境变量覆盖配置 key=...`。

::: tip 典型用途：恢复被关闭的面板
如果在面板中误将 `bot.admin_panel.enable` 关闭导致面板无法访问，可用 `ANIA_BOT_ADMIN_PANEL_ENABLE=true` 启动临时拉起面板，改回后再正常启动。详见 [Web 控制面板](/guide/web-panel#启用与访问)。
:::

## bot —— 框架配置

面板位置：**配置编辑 → Bot 配置**

### admin_id —— 管理员

| 配置键 | 默认值 | 说明 |
| --- | --- | --- |
| `bot.admin_id` | `123456789` | 管理员 ID。QQ 为纯数字 QQ 号，其他平台为带前缀的 ID（如飞书 `fs:ou_xxx`）。拥有最高权限：远程 `/exit` 退出、强制执行定时推送、查看全部定时任务、接收 panic 告警与启动通知等 |

### admin_panel —— Web 控制面板

| 配置键 | 默认值 | 说明 |
| --- | --- | --- |
| `bot.admin_panel.enable` | `true` | 是否启用面板 |
| `bot.admin_panel.listen` | `127.0.0.1:7700` | 监听地址；改为 `0.0.0.0:7700` 可局域网访问（面板有密码保护） |

首次启动会在控制台打印**随机初始密码**（仅显示一次），登录后可在面板右上角修改。详见 [Web 控制面板](/guide/web-panel)。

### platform —— 平台适配器开关

多平台并存，各自独立开关（`bot.platform.<name>.enable`）。默认仅启用 QQ，飞书 / Telegram 默认关闭：

| 配置键 | 默认值 | 说明 |
| --- | --- | --- |
| `bot.platform.napcat.enable` | `true` | 是否启用 QQ（NapCat）平台 |
| `bot.platform.feishu.enable` | `false` | 是否启用飞书平台（需同时配置下方 `bot.feishu.*`） |
| `bot.platform.telegram.enable` | `false` | 是否启用 Telegram 平台（需同时配置下方 `bot.telegram.*`） |

勾选后**重启生效**。未来新增平台同样在此出现对应开关。

### feishu —— 飞书适配器

在飞书开放平台创建企业自建应用，在「凭证与基础信息」拿到 App ID / Secret，并在「权限管理」开通 `im:message`、`im:message:send_as_bot`、`im:resource` 等权限，然后在面板勾选启用飞书并填写：

| 配置键 | 默认值 | 说明 |
| --- | --- | --- |
| `bot.feishu.app_id` | 空 | 应用 App ID |
| `bot.feishu.app_secret` | 空 | 应用 App Secret（敏感字段） |
| `bot.feishu.mode` | `ws` | 事件订阅方式：`ws`（WebSocket 长连接，推荐，**无需公网地址**）/ `webhook`（需公网 HTTPS 回调地址） |
| `bot.feishu.webhook.listen` | `127.0.0.1:7777` | webhook 模式本地监听地址 |
| `bot.feishu.webhook.path` | `/webhook/event` | webhook 模式回调路径，飞书后台填 `https://<公网地址><此路径>` |
| `bot.feishu.webhook.verification_token` | 空 | webhook 模式在飞书「事件订阅」页配置（ws 模式无需） |
| `bot.feishu.webhook.encrypt_key` | 空 | webhook 模式可选的事件加密密钥（ws 模式无需） |

::: tip 飞书能做什么 / 不能做什么
飞书适配器支持文本 / @提及 / 富文本 / 图片 / 文件 / 回复，撤回 / 表情回应 / 成员进出会映射到对应公共通知。合并转发、戳一戳、群签到、rkey 等 QQ 专属能力飞书**没有**——依赖它们的插件（如防撤回）在飞书不生效。
:::

### telegram —— Telegram 适配器

在 Telegram 中向 [@BotFather](https://t.me/BotFather) 发送 `/newbot` 创建机器人并获取 **Bot Token**，然后在面板勾选启用 Telegram 并填写。Telegram 采用 **Bot API 长轮询**（`getUpdates`）接收事件，**无需公网地址、无需部署协议端**：

| 配置键 | 默认值 | 说明 |
| --- | --- | --- |
| `bot.telegram.token` | 空 | Bot Token（敏感字段），形如 `123456:ABC-DEF...` |
| `bot.telegram.api_base` | `https://api.telegram.org` | Bot API 地址；国内部署可填自建 Bot API 网关/反代地址 |
| `bot.telegram.proxy` | 空 | HTTP/SOCKS5 代理（`http://host:port` 或 `socks5://host:port`），留空直连 |
| `bot.telegram.polling.timeout` | `30` | `getUpdates` 长轮询等待秒数（建议 10-50） |

::: tip Telegram 能做什么 / 不能做什么
Telegram 适配器支持文本 / @提及 / 图片 / 文件 / 语音 / 视频 / 回复，成员进出、表情回应、机器人被拉群/移出会映射到对应公共通知与平台事件；群聊中 @机器人 触发 AI 对话。**平台限制**：@ 只能以 `@username` 形式（无按 ID @ 的 API）；仅当机器人是管理员或关闭隐私模式时才能收到其他成员的加入/离开消息与表情回应；Bot API 无消息历史端点，历史消息仅覆盖适配器运行期间的缓存（AI 会话历史不受影响，由持久化存储承载）；消息撤回、合并转发等 QQ 专属能力 Telegram **没有**。
:::

### adapter —— QQ(NapCat) 协议适配器

WebSocket 与 HTTP **二选一**，由配置键 `bot.adapter.mode`（`ws` / `http`）决定。在[首次设置向导](/guide/web-panel#首次设置向导)中选择，也可在面板「配置编辑」中修改，**重启后生效**：

::: code-group

```text [WebSocket（推荐）]
bot.adapter.mode               = ws                    # 连接模式（默认）
bot.adapter.token              # 若 NapCat 端设置了 access token 则设置该键
bot.adapter.ws.address         = ws://localhost:4455   # NapCat WebSocket 服务端地址
bot.adapter.ws.worker_count    = 0                     # 事件处理线程数，0 = 按 CPU 自动调整
bot.adapter.ws.worker_queue_size = 1024                # 消息队列长度，超出则丢弃
```

```text [HTTP]
bot.adapter.mode               = http                  # 连接模式
bot.adapter.token              # 若 NapCat 端设置了 access token 则设置该键
bot.adapter.http.listen_port   = 6679                  # 本地监听端口，接收 NapCat 事件上报
bot.adapter.http.target_url    = http://localhost:6680 # NapCat HTTP 服务端地址
```

:::

::: warning Docker 部署注意
HTTP 模式下 NapCat 向 `localhost` 上报会失败，请将 NapCat 的 HTTP Client 地址改为 AniaBot 所在机器的内网 IP。
:::

### store.cache —— 缓存存储

| 配置键 | 默认值 | 说明 |
| --- | --- | --- |
| `bot.store.cache.driver` | `memory` | 缓存层（易失，支持 TTL 与列表语义）：`memory`（进程内，重启清空）/ `redis`（多实例共享） |
| `bot.store.cache.redis.address` | `localhost:6379` | Redis 地址（driver 为 redis 时生效） |
| `bot.store.cache.redis.password` | 空 | Redis 密码 |
| `bot.store.cache.redis.db` | `0` | Redis 数据库编号 |

持久化层的位置由上方[环境变量](#引导配置-环境变量)决定，不在面板中配置。

## plugin.ai_chat_bot —— AI 对话插件

面板位置：**配置编辑 → 插件配置**

### 基础配置

| 配置键 | 默认值 | 说明 |
| --- | --- | --- |
| `plugin.ai_chat_bot.base_url` | `https://api.deepseek.com` | 任意 OpenAI 兼容 API 地址 |
| `plugin.ai_chat_bot.api_key` | 空（必填） | API 密钥 |
| `plugin.ai_chat_bot.model` | `deepseek-chat` | 主模型名称 |
| `plugin.ai_chat_bot.multimodal` | `false` | 主模型是否支持图片输入 |
| `plugin.ai_chat_bot.rate_limit` | `2` | 同时处理的 AI 请求并发上限，超出后直接拒绝 |

### 模型参数

| 配置键 | 默认值 | 说明 |
| --- | --- | --- |
| `plugin.ai_chat_bot.max_context_tokens` | `128000` | 上下文 token 预算，超过 80% 自动压缩历史 |
| `plugin.ai_chat_bot.max_iterations` | `20` | 主对话单次回复的最大工具调用轮数，超出后强制结束 |
| `plugin.ai_chat_bot.temperature` | `1.2` | 采样温度 |
| `plugin.ai_chat_bot.top_p` | `0.9` | 核采样 |
| `plugin.ai_chat_bot.top_k` | `100` | Top-K 采样 |
| `plugin.ai_chat_bot.max_token` | `8192` | 单次回复最大 token |
| `plugin.ai_chat_bot.thinking.enable` | `false` | 深度思考开关 |
| `plugin.ai_chat_bot.thinking.mode` | `auto` | `none` / `low` / `medium` / `high` / `auto` |
| `plugin.ai_chat_bot.prompt` | 你是一个ai对话机器人，在QQ上和别人聊天，说话不要长篇大论<br><br>## 注意<br>- 当你不理解用户的问题时，要先获取用户最近的历史消息，再根据历史消息回答用户的问题 | 系统提示词（system prompt） |

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

| 配置键 | 默认值 | 说明 |
| --- | --- | --- |
| `plugin.ai_chat_bot.skills_dir` | `./skills` | Skill 目录 |
| `plugin.ai_chat_bot.skills` | `[]` | 指定加载的 skill 名称，空 = 加载全部 |

### 联网搜索

| 配置键 | 默认值 | 说明 |
| --- | --- | --- |
| `plugin.ai_chat_bot.search.token` | 空 | Jina AI token，用于 `web_search` / `web_explore` 工具 |

### OCR 备用识图

主模型不支持多模态时，可配置一个备用视觉模型把图片转述为文字：

| 配置键 | 默认值 | 说明 |
| --- | --- | --- |
| `plugin.ai_chat_bot.ocr.enable` | `false` | 是否启用 OCR 备用识图 |
| `plugin.ai_chat_bot.ocr.base_url` | `https://api.siliconflow.cn/v1` | 视觉模型 API 地址 |
| `plugin.ai_chat_bot.ocr.api_key` | 空 | API 密钥 |
| `plugin.ai_chat_bot.ocr.model` | `Qwen/Qwen3-VL-8B-Instruct` | 视觉模型名称 |
| `plugin.ai_chat_bot.ocr.temperature` | `0.6` | 采样温度 |
| `plugin.ai_chat_bot.ocr.top_p` | `0.95` | 核采样 |
| `plugin.ai_chat_bot.ocr.top_k` | `20` | Top-K 采样 |
| `plugin.ai_chat_bot.ocr.max_token` | `600` | 单次描述最大 token |
| `plugin.ai_chat_bot.ocr.prompt` | `你负责将看到的图片用markdown格式描述出来，不要有无关的其他对话` | 图片描述提示词 |

### 高危工具（默认关闭）

::: danger 安全提醒
以下工具允许 AI 直接操作宿主机，存在风险，**默认全部关闭**，请确认环境隔离后再开启。
:::

| 配置键 | 默认值 | 说明 |
| --- | --- | --- |
| `plugin.ai_chat_bot.bash.enable` | `false` | 允许 AI 在宿主机执行 shell 命令 |
| `plugin.ai_chat_bot.bash.shell` | 空 | 命令解释器，留空使用系统默认（Linux/macOS 为 `sh`，Windows 为 `cmd`），可填 `/bin/bash`、`/bin/ash` 等 |
| `plugin.ai_chat_bot.bash.env` | `[]` | 环境变量，如 `["HOME=/root"]` |
| `plugin.ai_chat_bot.bash.whitelist` | `[]` | 非空时仅允许匹配这些正则的命令 |
| `plugin.ai_chat_bot.bash.blacklist` | `["config(\\.dev)?\\.(yaml|yml|json)", "^mkfs", "^shutdown", "^reboot"]` | 匹配这些正则的命令被禁止 |
| `plugin.ai_chat_bot.local_image.enable` | `false` | 允许 AI 读取宿主机本地图片 |

### AI 定时任务（clock）

| 配置键 | 默认值 | 说明 |
| --- | --- | --- |
| `plugin.ai_chat_bot.clock.enable` | `true` | 启用后 AI 可自主创建定时任务 |
| `plugin.ai_chat_bot.clock.default_timeout_sec` | `120` | 单次触发默认超时（秒） |
| `plugin.ai_chat_bot.clock.max_log_entries` | `500` | 执行日志保留条数（滚动覆盖） |

与框架的 `StartCron` 静态任务不同，clock 任务由 AI / 用户**动态创建**、持久化保存、重启不丢。执行日志可在面板「状态总览」页查看。详见 [AI 对话插件](/guide/builtin-plugins#ai-对话插件)。

### AI 长期记忆（memory）

| 配置键 | 默认值 | 说明 |
| --- | --- | --- |
| `plugin.ai_chat_bot.memory.enable` | `true` | 启用后 AI 可通过 `memory_save` / `memory_search` / `memory_forget` 工具管理长期记忆 |
| `plugin.ai_chat_bot.memory.max_entries` | `200` | 单个会话（群/好友）的记忆条数上限 |

记忆按群聊 / 好友隔离、持久化保存、重启不丢。详见 [AI 对话插件](/guide/builtin-plugins#ai-对话插件)。

### AI 子代理（subagent）

| 配置键 | 默认值 | 说明 |
| --- | --- | --- |
| `plugin.ai_chat_bot.subagent.enable` | `true` | 启用后 AI 可通过 `subagent_run` 工具委派子任务 |
| `plugin.ai_chat_bot.subagent.timeout_sec` | `300` | 单次执行默认超时（秒），单次调用可覆盖（上限 1800；实际还会按框架单次消息处理预算自动收缩，为主请求预留收尾时间） |
| `plugin.ai_chat_bot.subagent.max_iterations` | `10` | 子代理工具调用循环的最大轮数 |
| `plugin.ai_chat_bot.subagent.max_result_len` | `4000` | 返回结果最大字符数，超出截断以防污染主对话上下文 |

子代理以全新一次性上下文运行、拥有与主 AI 一致的工具能力，但**不能再委派子代理**。详见 [AI 对话插件](/guide/builtin-plugins#ai-对话插件)。

## plugin.interceptor —— 请求拦截插件

| 配置键 | 默认值 | 说明 |
| --- | --- | --- |
| `plugin.interceptor.enable` | `false` | 是否启用请求拦截，关闭时放行全部消息 |
| `plugin.interceptor.mode` | `blacklist` | 名单模式：`blacklist` 名单内屏蔽 / `whitelist` 仅名单内放行 |
| `plugin.interceptor.groups` | `[]` | 群 ID 名单，每行一个（QQ 为群号，其他平台为带前缀的群 ID，如 `fs:oc_xxx`） |
| `plugin.interceptor.friends` | `[]` | 用户 ID 名单，每行一个（QQ 为 QQ 号，其他平台带前缀），对私聊及群聊消息发送者均生效 |

被拦截的会话消息不再传播到后续插件（AI 对话插件收不到，不产生 AI 请求）。

::: warning `whitelist` 模式下的放行逻辑
群聊消息必须同时满足「群号在 `groups` 名单」且「发送者 QQ 在 `friends` 名单」才会放行；只填 `groups` 不会放行该群内其他成员的消息。`whitelist` 模式下名单留空会拦截所有会话。
:::

详见 [请求拦截插件](/guide/builtin-plugins#请求拦截插件)。

## plugin.dailyNews —— 每日新闻插件

| 配置键 | 默认值 | 说明 |
| --- | --- | --- |
| `plugin.dailyNews.api` | `https://60s.viki.moe/v2/60s?encoding=image-proxy` | 新闻图 API |
| `plugin.dailyNews.cron` | `0 18 * * *` | cron 表达式，默认每天 18:00 触发 |
| `plugin.dailyNews.groups` | `[123456, 7891011]` | 接收推送的群 ID 列表（QQ 为群号，其他平台带前缀） |

## files.mcp_json —— MCP 服务定义

配置键 `files.mcp_json`（面板「文件编辑 → MCP 服务器」页，原 `aniabot.mcp.json`）定义 AI 可用的 MCP Server，内容为 JSON，支持 stdio / SSE / Streamable HTTP 三种传输：

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
