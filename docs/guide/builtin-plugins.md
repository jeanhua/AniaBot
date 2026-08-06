# 内置插件

AniaBot 自带七个插件，在 `cmd/main.go` 中注册。它们既是开箱即用的功能，也是插件开发的最佳参考实现。

```go
bot.AddPlugin(pluginsys.NewPluginSys())          // 系统插件
bot.AddPlugin(pluginlog.NewPlugin())             // 日志插件
bot.AddPlugin(pluginrepeat.NewPlugin())          // 复读机
bot.AddPlugin(pluginantiwithdrawal.NewPlugin())  // 防撤回
bot.AddPlugin(plugininterceptor.NewPlugin())     // 请求拦截
bot.AddPlugin(pluginaichat.NewAIChatPlugin())    // AI 对话
bot.AddPlugin(pluginnews.NewNewsPlugin())        // 每日新闻
```

<PluginCards :plugins="[
  { icon: 'gear', name: '系统插件', desc: '帮助、远程退出、panic 告警', cmds: ['/help', '/exit'] },
  { icon: 'chat', name: 'AI 对话', desc: '大模型对话 · 工具调用 · 定时任务', cmds: ['#新对话', '/stop', '/clock'] },
  { icon: 'shield', name: '防撤回', desc: '消息缓存与合并转发回顾', cmds: ['/explore [n]'] },
  { icon: 'ban', name: '请求拦截', desc: '黑白名单放行或屏蔽 AI 请求', cmds: [] },
  { icon: 'repeat', name: '复读机', desc: '三连同样消息自动跟读', cmds: ['/close repeat', '/enable repeat'] },
  { icon: 'news', name: '每日新闻', desc: '定时推送 60s 新闻图', cmds: ['/news', '/news force'] },
  { icon: 'log', name: '日志插件', desc: '控制台消息流水打印', cmds: [] },
]" />

::: tip 命令中的 @ 约定
群聊中标注「需 @」的命令，都需要 **@机器人** 后再发命令才会触发（如 `@机器人 /news`）；私聊中直接发送即可。
:::

## 系统插件

`pluginsys` · Order = -1000（最先执行）· 群聊 + 私聊

框架的「管家」，负责帮助信息、远程控制与故障告警。

| 命令 | 场景 | 说明 |
| --- | --- | --- |
| `/help` | 私聊 | 列出已加载插件及帮助说明（按 `ShowFor` 过滤） |
| `@机器人 /help` | 群聊 | 群聊版帮助 |
| `/exit` | 私聊 | **仅管理员**，远程停止机器人进程 |

其他行为：

- **启动通知**：Bot 启动完成后自动私聊管理员「AniaBot启动成功」
- **panic 告警**：任何插件运行时 panic，系统插件会私聊通知管理员（1 分钟内防抖，避免刷屏）

## AI 对话插件

`pluginaichat` · Order = 1000（最后执行，兜底响应）· 群聊 + 私聊

AniaBot 的核心插件，支持三种 LLM API 格式：OpenAI 兼容（Chat Completions，如 DeepSeek）、OpenAI Responses API 与 Anthropic Messages API（Claude），经 `plugin.ai_chat_bot.api_format` 切换。

### 对话方式

| 操作 | 效果 |
| --- | --- |
| 群聊中 **@机器人 + 内容** | 发起/继续对话 |
| 私聊直接发送内容 | 发起/继续对话 |
| 消息中带 `#新对话` | 清空当前会话历史，开始新对话 |
| `/stop` | 立即停止正在进行的 AI 响应 |

其他细节：

- 每个群 / 每个好友拥有**独立会话**，互不影响
- 对话历史持久化保存，重启后继续聊；超过 token 预算 80% 时自动压缩。SQL 后端（sqlite/mysql）下历史按消息行级存储（`ania_chat_session` + `ania_chat_message` 两表），新消息增量落盘；非 SQL 后端回退整段 KV
- 闲置会话会被自动回收（默认闲置 120 分钟或驻留超过 128 个会话时淘汰最久未活动的，均可在配置中调整或关闭）；淘汰只释放内存，历史已持久化，下次发言自动重建继续聊
- 群内连续 30 条消息无人 @ 机器人时，自动清理该群对话缓存
- 同一时刻同一会话只允许一个进行中的请求；响应期间到达的消息会进入排队队列（每会话上限 20 条），当前响应结束后自动合并为一轮请求逐条回应，`/stop` 会同时清空队列

### 内置工具

AI 可在对话中自主调用：

| 工具 | 说明 |
| --- | --- |
| `time` | 获取当前时间 |
| `webSearch` / `webExplore` | 联网搜索 / 抓取网页（需配置 Jina token） |
| `meme` | 发送梗图 |
| `get_msg_history` | 读取群/私聊最近消息 |
| `load_images` | 加载用户消息中的图片（多模态或 OCR 识别） |
| `get_private_file_url` / `file` | 私聊文件链接获取与发送生成的文件（`file` 需配置开启） |
| `bash` | 执行宿主机命令（默认关闭，有黑白名单） |
| `local_image` | 读取宿主机本地图片（默认关闭） |

此外还有 MCP 发现/加载工具与 `skill_read`，见 [配置详解](/guide/configuration#files-mcp-json-——-mcp-服务定义)。

### AI 定时任务（clock）

让 AI 拥有「闹钟」。用户和 AI 都能创建 cron 任务，到点后以一个全新的一次性会话执行任务内容（可调用全部工具），执行结果发送到目标群/好友。

**用户命令**：

```
/clock                                  查看当前会话的定时任务
/clock list [all]                       列出任务（all 仅管理员）
/clock add [--once] <cron> | <标题> | <内容>   新增任务（--once 为单次任务）
/clock del <id>                         删除任务
/clock on <id> / /clock off <id>        启用 / 停用
/clock info <id>                        任务详情与最近执行记录
/clock timeout <id> <秒数>              设置单次执行超时（0 恢复默认）
/clock run <id>                         立即执行一次（仅管理员）
/clock log [n]                          最近 n 条执行记录（默认 10）
```

**示例**：

```
/clock add 0 8 * * * | 早安播报 | 告诉大家早上好，并播报今日日期
```

也可以直接对 AI 说：「每天早上 8 点提醒大家喝水」，它会通过 `clock_create` 工具自己建好任务。

### AI 长期记忆（memory）

让 AI 拥有「记性」。AI 通过 `memory_save` / `memory_search` / `memory_forget` 三个工具自行管理长期记忆：得知用户的称呼、偏好、重要信息，或群里形成约定时主动保存；对话涉及过去的事情时先检索回忆；记忆过时或用户要求忘记时删除。

- 记忆**跨会话、跨重启**保留，持久化保存（SQL 后端下存于 `ania_memory` 表，每行一条；非 SQL 后端回退 `memory:` 命名空间的整段 KV）
- 记忆按**群聊 / 好友隔离**（`g:群号` / `f:QQ号`），群与群、群与私聊之间互不可见
- 每个会话记忆条数上限默认 **200** 条，写满后 AI 需先清理或合并旧记忆
- 相同内容自动去重，不会重复写入

无需用户命令，直接对 AI 说「记住我喜欢喝美式」「我之前说过什么？」即可触发。配置见 [配置详解](/guide/configuration)。

### AI 子代理（subagent）

让 AI 拥有「分身」。AI 通过 `subagent_run` 工具把复杂/耗时的子任务（如深度调研、多轮搜索后总结）委派给一次性子代理执行，子代理跑完后只把最终结果带回给主对话：

- 子代理以**全新一次性上下文**运行（看不到当前对话历史），不持久化、执行完即丢弃，中间过程不占用主对话上下文
- 子代理拥有与主 AI **一致的工具能力**（搜索、网页浏览、消息历史、MCP 等；定时任务/长期记忆需对应功能开启），但**不能再委派子代理**（防止递归）
- 子代理执行过程中的中间消息不会发给用户，发送图片/文件等能力仍作用于当前会话；用户消息中的图片需由主 AI 加载（子代理的图片状态与主会话隔离）
- 默认超时 **300 秒**（单次调用可指定，上限 1800 秒；实际还会按框架单次消息处理预算自动收缩，为主请求预留收尾时间），超时后子代理中止、主 AI 继续回复；返回结果超过 **4000** 字符自动截断

无需用户命令，直接对 AI 说「帮我深入调研一下 XXX 再总结」这类复杂任务即可触发。配置见 [配置详解](/guide/configuration#ai-子代理-subagent)。


### AI 知识库（knowledge）

让 AI 拥有「资料库」。AI 通过 `kb_add` / `kb_search` / `kb_forget` 等工具把完整资料（教程、文章、URL 正文）存入知识库，并在对话中按需检索引用：

- 知识库按**会话隔离**（群聊 `g:群号` / 私聊 `f:用户ID`），另有一份**全局库**（`global`），所有会话都可检索、仅 Web 面板可管理
- 长文档自动按 600 字符一块、60 字符重叠切片，检索命中**块**而非整篇，避免无关内容占用上下文
- 默认每次对话前自动按关键词检索相关文档注入上下文（`kb.auto_inject`）；启用向量检索（`kb.embedding.enable`）后入库计算语义向量，检索与关键词混合打分，检索更准
- 每个作用域文档上限默认 **500** 篇，标题+内容相同自动去重

无需用户命令，对 AI 说「把这篇教程存进知识库」或问知识库里的内容即可触发。配置见 [配置详解](/guide/configuration#ai-知识库-knowledge)。

### Agent 团队（team）

让 AI 拥有「团队」。AI 通过 `team_run` 组建临时团队，把子任务拆给多个带角色描述的成员代理**并行执行**，再汇总各成员结果给出最终回答；团队定义可保存复用（`team_save` / `team_list` / `team_delete`），Web 面板提供「Agent 团队」管理页：

- 每个成员复用子代理执行路径：全新一次性上下文、执行完即丢弃；模型使用子代理模型配置（`plugin.ai_chat_bot.subagent.*`，留空跟随主模型）
- 单次最多并行 **5** 个成员（配置可调，硬上限 10），每个成员结果超出 **4000** 字符自动截断，防止汇总报告污染主对话上下文
- 成员**不能再组建团队**（防止递归）

默认关闭，配置见 [配置详解](/guide/configuration#agent-团队-team)。对 AI 说「组建一个调研团队，分别负责技术、市场、竞品分析，最后汇总报告」即可触发。

### 每日 Token 配额（quota）

在 Web 面板「配额管理」页可查看每个会话与全局的 AI token 用量；启用配额后可按「每会话每日」与「全局每日」两个维度限制消耗，超限后 AI 请求被拒绝（含子代理、定时任务消耗），未设置上限时仍记录用量。配置见 [配置详解](/guide/configuration#每日-token-配额-quota)。

### Query 日志（query_log）

Web 面板「Query 日志」页记录每次 AI 回复的完整执行过程：触发会话、发送者、用户输入、LLM 轮数、工具调用明细（名称/参数/结果/耗时）、token 用量与最终回复，状态区分执行中 / 成功 / 已停止 / 超时 / 出错，可条件筛选。配置见 [配置详解](/guide/configuration#query-日志-query-log)。

## 防撤回插件

`pluginantiwithdrawal` · 群聊为主 · **仅 QQ 平台**（`Meta.Platforms = ["qq"]`）

::: warning 平台限制
防撤回依赖 **QQ 专属能力**：合并转发（把缓存消息打包成聊天记录）与 **rkey**（图片/文件 URL 续期）。飞书没有合并转发与 rkey 机制，因此本插件只在 QQ 平台启用，飞书消息不会缓存。
:::

缓存每个群最近 **100 条** 消息，即使对方撤回也能回顾。

| 命令 | 场景 | 说明 |
| --- | --- | --- |
| `@机器人 /explore [n]` | 群聊 | 以合并转发形式发送最近 n 条缓存消息（n ≤ 100，默认 50） |
| `/explore <群号> [n]` | 私聊 | **仅管理员**，查看指定群的缓存消息 |

细节：

- 图片/文件消息通过 NapCat rkey 自动续期；无法续期时超过 3 分钟显示「已过期」占位
- 语音消息显示 `[语音消息]`，转发消息显示 `[转发消息，暂不支持查看]`

## 请求拦截插件

`plugininterceptor` · Order = 900（普通插件之后、AI 对话插件之前）· 群聊 + 私聊

按**白名单 / 黑名单**模式放行或屏蔽指定群聊、好友的消息：被拦截的消息不再向后续插件传播（AI 对话插件收不到，也就不会产生 AI 请求），而排在其前面的复读机、防撤回等插件不受影响。

无命令，全部在 Web 控制面板配置：

| 配置 | 说明 |
| --- | --- |
| 启用请求拦截 | 默认关闭，关闭时放行全部消息 |
| 名单模式 | `blacklist`：名单内的群/好友被屏蔽；`whitelist`：仅名单内的群/好友放行 |
| 群 ID 名单 | 每行一个群 ID（QQ 为群号，其他平台为带前缀的群 ID，如 `fs:oc_xxx`） |
| 用户 ID 名单 | 每行一个用户 ID（QQ 为 QQ 号，其他平台带前缀），对私聊及群聊消息发送者均生效 |

::: warning 白名单模式注意
`whitelist` 模式下，**群聊消息必须同时满足「群 ID 在群 ID 名单」且「发送者 ID 在用户 ID 名单」**才会放行。只填群 ID 名单不会放行该群内其他成员的消息；名单留空表示**拦截所有会话**——任何群和私聊都无法触发 AI 回复。
:::

## 复读机插件

`pluginrepeat` · 群聊

群内同一条消息出现 **3 次** 时，机器人自动跟读一次（之后重置计数）。

| 命令 | 说明 |
| --- | --- |
| `@机器人 /close repeat` | **仅管理员**，关闭复读 |
| `@机器人 /enable repeat` | **仅管理员**，开启复读 |

## 每日新闻插件

`pluginnews` · 群聊

按 cron 表达式定时向配置的群推送 [60s 读懂世界](https://60s.viki.moe/) 新闻图。

| 命令 | 说明 |
| --- | --- |
| `@机器人 /news` | 立即获取一张新闻图 |
| `/news force` | **仅管理员**（私聊），立即向所有配置群执行一次推送 |

配置见 [配置详解 · dailyNews](/guide/configuration#plugin-dailynews-——-每日新闻插件)。

## 日志插件

`pluginlog` · Order = -1000 · 群聊 + 私聊

在控制台打印每一条收发的消息内容，无命令、无配置，调试期建议保留，生产环境可从 `cmd/main.go` 移除。

