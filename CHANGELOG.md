# Changelog

本项目的所有重要变更都会记录在此文件中。

格式基于 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)，
版本号遵循 [语义化版本](https://semver.org/lang/zh-CN/)。

发布流程：打 `v*.*.*` 标签后，GitHub Release 的正文与 Docker 镜像描述
都会自动从本文件中对应版本段落提取，因此发版前请先更新对应版本的内容。

## [Unreleased]

### 新增

- **面板技能管理支持查看 SKILL 详情**：技能列表新增「详情」按钮，弹窗展示 SKILL.md 完整正文与附属文件清单；Markdown 附属文档可点击直接预览，脚本/二进制文件展示名称与大小。后端新增 `GET /api/skills/{name}` 详情接口，由 `SkillDetailSource` 可选接口提供，不影响现有 skill 列表 / 上传 / 删除行为
- **记忆与知识库自动注入支持向量检索 + 存量向量后台回填**：此前每轮对话的自动注入（`kb.auto_inject` / `memory.auto_inject`）刻意只走纯关键词检索，同义不同词的话题（如「饮品」vs「咖啡」）无法触发热注入。现启用 `kb.embedding` 后自动注入改为语义+关键词混合检索：每轮对用户消息 embed 一次（`embedder` 新增 128 条 FIFO 查询向量缓存与 10s 注入短超时），知识库与记忆注入复用同一向量；知识库注入在关键词分数门槛之外新增纯语义命中准入（余弦相似度 ≥ 0.35，`kbChunkResult` 新增 `Sim` 字段），记忆注入直接透传向量复用混合打分。未启用 embedding 或服务失败时行为与旧版完全一致（纯关键词兜底）。同时两个管理器在 embedder 可用时启动后台 goroutine，为启用向量之前写入、缺少语义向量的存量记忆/文档逐条补算 embedding 并落盘（200ms 间隔防限流，失败跳过待下次重启），老数据无需手工重建即可获得语义检索能力

## [v4.5.0] - 2026-08-07

### 新增

- **Anthropic 格式 Prompt 缓存（`cache_control` 断点）**：此前 `anthropic` 格式完全没有上游缓存（Anthropic 需显式声明断点才会缓存），现新增 `plugin.ai_chat_bot.prompt_cache.enable`（默认开启，面板「AI 对话 · 模型」组可配）与 `prompt_cache.ttl`（`5m` / `1h`，默认 5m）。启用后请求为 system 最后一个块与最后一条消息的最后一个可缓存块（文本/图片/tool_result）设置 `cache_control` 断点：system+历史前缀稳定命中，tool calling 多轮只对新增的工具往返内容付一次缓存写入，正常对话下一轮断点随历史前移；关闭后请求体与旧行为完全一致，兼容不支持 cache_control 的上游代理。`chat_completions` / `responses` 格式为提供方自动前缀缓存，不受此开关影响。`aichat` 组件新增 `PromptCacheConfig` 与 `WithPromptCache` 客户端选项（备用模型自动继承）；同时明确约定：保持 system 稳定是前缀缓存命中的前提，动态内容（如长期记忆注入）只能追加到消息尾部而非 system

- **AI 长期记忆主动注入（memory.auto_inject）**：pluginaichat 新增每轮自动注入开关（默认关闭，面板「AI 对话 · 定时与记忆」组可配 plugin.ai_chat_bot.memory.auto_inject 与 inject_max，默认每轮最多 3 条）。开启后按用户消息做纯关键词检索（复用知识库 queryTerms 中文切词，无 embedding 成本），命中记忆以「【长期记忆】…」块追加到用户消息前——与知识库自动注入同款尾部注入：system 保持不变、不影响上游前缀缓存；用户消息不落盘，注入内容不污染持久化历史；语义检索仍由 AI 按需调 memory_search 完成。memoryManager 新增 utoInject（条数上限 + 1500 字符预算）
- **meme 表情包工具改为可配置接口（默认切换 GIPHY）**：原内置表情包接口（api.suol.cc）已失效，且同类国内免 Key 小接口普遍不稳定。现 `meme` 工具不再绑定特定平台：`plugin.ai_chat_bot.meme.url` 为请求地址模板（`${msg}` 搜索词 / `${num}` 数量 / `${key}` API Key 占位符），`meme.list_path` / `meme.img_field` 为响应 JSON 的 gjson 提取路径（图片数组位置与元素内图片 URL 字段，支持 `images.fixed_width.url` 这类嵌套路径）——任何「返回一组图片 URL」的接口都能接入，接口再挂时在面板「AI 对话 · 工具」组改配置即可切换，无需改代码。默认接口切换为 **GIPHY stickers**（全球最大 GIF/表情包平台，`meme.key` 填免费 API Key，面板按敏感字段掩码；模板含 `${key}` 而未配置时工具返回明确配置指引而非上游 401 裸错误）。`functool` 新增 `MemeConfig`，`CreateDefaultTools` / `CreateToolsWithMCP` / `CreateToolsWithSkill` 签名相应增加 `memeConfig` 参数

- **AI MCP 管理工具（`mcp_list` / `mcp_add` / `mcp_remove` / `mcp_reconnect`）**：`pluginaichat` 新增 MCP 服务器自管理能力（`plugin.ai_chat_bot.mcp_tool.enable` 开启，默认关闭），AI 无需后台面板即可自行管理 MCP 服务器：支持 stdio 本地命令与 streamable/sse HTTP 端点两种模式（环境变量/请求头以 `KEY=VALUE` 列表传参，服务器名称校验满足 LLM 工具名规范）；添加/删除经 DI 注入的 `ConfigEditor` 写入 `files.mcp_json` 持久化，同时运行时热注册/注销立即生效（持久化失败自动回滚运行时注册，删除容忍运行时未注册的服务器以保证配置层面删除总可用）；`mcp_reconnect` 重建连接并刷新工具列表，对启动时连接失败从未注册的服务器自动从配置读取定义重新注册。`llmtool` 的 `ToolExecuter` 相应改为 RWMutex 保护共享工具表与 manager 列表，新增 `AddMCP` / `RemoveMCP` / `ReconnectMCP` / `MCPServerInfos` 运行时管理能力；操作记入操作日志
- **MCP 工具懒加载开关**：新增 `plugin.ai_chat_bot.mcp.lazy_load`（默认开启，面板「AI 对话 · 工具」组可直接切换）。开启时沿用两阶段懒加载（`mcp_discover` / `mcp_load` 按需加载，节省上下文）；关闭后启动时全量注册所有 MCP 工具——工具列表恒定、上游 prompt 缓存命中率更高，但工具较多时上下文开销大。`functool.CreateToolsWithMCP` / `CreateToolsWithSkill` 签名相应增加 `mcpLazyLoad` 参数
- **Skill 缓存刷新工具（`skill_reload`）**：常驻注册（无需开关），供 AI 经 `bash` 等工具直接编辑本地 skill 文件（SKILL.md / 附属文档）后重新从磁盘加载全部 skill 并刷新缓存——面板与 skill 管理工具的安装/删除本就会自动热重载，此工具面向「绕过管理器直接改文件」的场景。`SkillManager` 记录最近一次加载的目录与白名单，新增 `Refresh` 原地重载

## [v4.4.0] - 2026-08-06

### 新增

- **AI 对话支持三种 LLM API 格式**：`aichat` 的 `LLMClient` 重构为「重试/备用外壳 + 按格式后端（`llmBackend`）」结构，在原有 OpenAI Chat Completions 之外新增 **OpenAI Responses API** 与 **Anthropic Messages API（Claude）** 两种格式。主模型经 `plugin.ai_chat_bot.api_format`（`chat_completions` 默认 / `responses` / `anthropic`）选择，子代理、压缩器、备用模型各有独立 `api_format`（留空跟随主模型格式）；应用层重试、备用模型切换、流式增量回复、token 用量统计（含缓存命中）三种格式行为一致。Anthropic 格式完整支持深度思考（`thinking.mode` 映射 `budget_tokens`，思考块含签名随历史持久化并在 tool calling 多轮中原样回传），非流式调用内部走流式聚合以绕开 API 的大 max_tokens 流式强制
- **AI 配置管理工具（`config_get` / `config_set`）**：AI 对话插件新增配置管理工具，允许 AI 查看与修改框架配置（`plugin.ai_chat_bot.config_tool.enable` 开启，默认关闭）。`config_get` 列出全部已注册配置项或查看单个键，敏感字段（API Key/Token 等）依据配置注册表对 AI 掩码，不会泄露进对话上下文；`config_set` 写入数据库（重启后生效），仅允许修改注册表中已声明的键（防止拼写错误产生垃圾键、挡住 `meta.*` 内部键），值按 JSON 解析保留类型（bool/数字/数组），并与面板一致禁止置空面板监听地址。配置中心读写能力经新的插件 DI 项 `plugin.ConfigEditor`（由 `configstore.Store` 实现，持久化存储不可用时为 nil）注入，普通插件读配置仍走 viper / `ConfigSchema` 结构体绑定
- **AI 重启工具（`restart_bot`）**：允许 AI 重启 Bot 进程使配置修改生效（`plugin.ai_chat_bot.config_tool.restart_enable` 开启，默认关闭）。延迟数秒执行（默认 5s，可在 3-120s 内指定），先让当前回复走完发送流程再替换进程。进程自重启逻辑抽取为共享包 `bot/component/sysrestart`（面板「重启 Bot」按钮与自动更新同步改用，行为不变）
- **面板「操作日志」模块**：新增操作审计日志，记录面板登录成功/失败、密码修改、配置更新（含预设与扩展配置文件）、定时任务/skill/记忆/团队/知识库管理、配额清零、重启、自动更新、首次设置向导完成，以及 AI 工具发起的配置修改与重启（敏感值不落日志）。新组件 `bot/component/oplog`（包级单例，core 启动时 `Init` 注入 `__oplog` 命名空间存储）：SQL 后端走 `ania_op_log` 行级存储（过滤条件下推 WHERE、容量淘汰走范围删除），非 SQL 后端回退逐条 KV 记录；默认保留最近 500 条。面板新增「操作日志」页面（分类筛选、时间范围与关键词查询、滚动分页、自动刷新），接口 `GET /api/oplogs`

- **AI Skill 管理工具（`skill_list` / `skill_install` / `skill_remove`）**：`pluginaichat` 新增技能自管理能力（`plugin.ai_chat_bot.skill_tool.enable` 开启，默认关闭），AI 无需后台面板即可自行上网搜索并安装/卸载技能：技能资源由 AI 用已有的 `webSearch` / `webExplore` 上网搜索（GitHub API 搜索有频控，不做专用搜索工具）；`skill_install` 支持从 zip 压缩包直链、GitHub 仓库地址（自动转换为 codeload zip 下载，依次尝试 main/master 默认分支）或单个 `SKILL.md`/`.md` 文件直链安装，也可直接撰写 SKILL.md 全文（含 frontmatter）创建技能；下载/内容均有体积上限，安装复用面板同款 zip-slip / 校验 / 热重载逻辑，落盘后立即生效并记入操作日志（`skill_install` / `skill_remove`）。安装逻辑抽取为 `AIChatPlugin.installSkillZip` / `SkillInstallFromContent`（返回安装后的 skill 元信息，前端面板上传行为不变），`llmtool` 新增 `SkillNameFromContent` 供内容安装回退目录名

### 修复

- **定时任务执行日志重启中断收尾**：此前定时任务执行过程中若 Bot 重启，该次执行的日志记录会永远停留在「执行中」（running）状态（goroutine 随进程销毁，无法再更新）。现 `tasklog` 新增 `interrupted` 状态与 `MarkRunningInterrupted`：`clockManager` 启动时把所有遗留 running 日志统一标记为「中断」，回填中断原因、完成时间与耗时（SQL/KV 双后端一致，SQL 按状态列下推候选、payload 终判）；面板任务日志页新增「中断」状态筛选与样式，`GET /api/tasklogs` 的 status 参数同步支持 `interrupted`

## [v4.3.0] - 2026-08-05

⚠️注意：本次更新带来破坏性数据变更

### 新增

- **持久层关系表能力（可选探测）**：`common/storage` 新增 `SQLPersistentStorage` 可选接口（`SQLDB()`/`SQLDialect()`）、`SQLBackend()` 探测函数与 `TableDDL`/`EnsureTables()` 幂等建表（SQLite/MySQL 双方言 DDL）。框架内置 sqlite/mysql 持久后端已实现该接口（Clone 出的插件命名空间子存储共享同一连接，探测同样成功）；插件/组件按 `bot.QQ` 同款类型断言探测，探测或建表失败只记日志并自动回退纯 KV 方案，功能不缺失。插件自建关系表统一 `ania_` 前缀
- **AI 会话闲置回收**：`pluginaichat` 的会话缓存此前只增不减，ChatBot 实例永久驻留内存，占用随活跃会话数线性增长。现会话条目携带最近活跃时间，后台每分钟按 `plugin.ai_chat_bot.session.max_idle_minutes`（默认 120 分钟，0 禁用）闲置淘汰 + `plugin.ai_chat_bot.session.max_sessions`（默认 128 个，0 不限）LRU 容量淘汰；正在响应（持有会话锁）或有排队消息的会话跳过。淘汰只释放内存对象，历史已持久化，下次发言自动重建并回放；已知副作用：会话内 `mcp_load` 动态加载的工具随淘汰失效（等同重启，配置项说明已注明）
- **面板配置导出 JSON**：配置管理页新增「导出 JSON」按钮，将完整配置（含密钥等敏感字段的真实值、MCP / Prompt 覆盖）导出为 JSON 文件下载，便于备份与迁移；接口 `GET /api/config/export` 需要登录，响应标记 `no-store` 并携带带时间戳的文件名

### 变更

- **AI 对话历史改为行级存储**：SQL 后端下历史从 `ania_kv` 整段 JSON（每次变更全量重写）迁移为 `ania_chat_session`（会话表）+ `ania_chat_message`（消息表，`(session_id, seq)` 联合主键，无外键）一对多两表；追加只插入新消息行（增量落盘，会话表 `msg_count` 充当序号分配器，同事务读改保证崩溃一致性），压缩/截断走单事务重排覆盖。`aichat.HistoryStore` 接口相应改为 `Load`/`Append`/`Replace`/`Clear`；非 SQL 后端回退 KV 整段 JSON，行为不变
- **长期记忆行级化**：SQL 后端下 `memoryManager` 的持久化从整段 JSON 数组迁移为 `ania_memory` 表（每条记忆一行，`(scope, id)` 联合主键）；去重、上限、截断、embedding 时机等管理逻辑与公开行为不变，非 SQL 后端回退整段 KV
- **Query 日志 / 任务日志行级化**：SQL 后端下分别存于 `ania_query_log` / `ania_task_log`（每条记录一行，过滤条件字段冗余为列下推 WHERE，完整 Entry 仍存 payload 列）；过滤查询不再全量列键逐条加载，容量淘汰走范围删除。`querylog`/`tasklog` 公开 API（`Record`/`Update`/`Recent`/`Query`/`Filter` 等）一行未改，面板与工具无感知；非 SQL 后端保留逐条 KV 实现

### 移除

- **旧版数据格式迁移路径**：`pluginaichat` 旧版无前缀历史键迁移（`migrateLegacyHistory`）与 querylog/tasklog 旧版整体数组迁移（`migrateLegacy`）已删除（旧数据按测试数据处理，不迁移）
- **清理历史迁移/重构遗留的无效代码**：删除适配器注册表迁移后遗留的未使用字段（飞书适配器 `started`）、Telegram 渲染重构后遗留的未使用方法 `mdEnabled`、定时任务日志查询重构后遗留的未使用方法 `recentLogs`，以及核心路由测试中未使用的 `prefix` 字段
- **移除旧版单适配器 API 与旧数据兼容分支**（破坏性 API 变更）：
  - `core` 删除旧版单适配器接入：`WithAdapterFactory` 选项、`NewAniaBot` 的 adapter 参数（现为 `NewAniaBot(option ...Option)`）与 `legacy` 定义名一并移除，适配器统一走注册表（各平台包 `init()` 注册，按 `bot.platform.<name>.enable` 启用）；`cmd/main.go`、`custom/mvp` 示例与插件文档同步改为注册表接入（`custom/mvp` 包名由 `mvp` 修正为 `main`，恢复可运行）
  - `aichat` 删除旧版落盘数据的兼容处理（旧数据一律按测试数据处理，不再兼容）：`BuildChatMessages` 不再过滤历史中的 system 角色消息，历史回放不再对远程图片 URL 降级（`degradeImagesToText`/`isRemoteImageURL` 及其测试删除）；落盘侧 `degradeImagesForPersist` 保留，新写入历史仍剔除图片片段

### Fixed

- **流式回复跨工具轮重复发送中间文本**：`pluginaichat` 的流式缓冲在整个 `Chat` 会话期间只增不清，工具调用轮结束（`OnStreamRoundEnd`）未重置缓冲，导致下一轮新建的流式消息以「历史各轮全文 + 新文本」开头，逐轮累积重复（Discord/Telegram/飞书均受影响）。现工具边界清空缓冲，每条消息只携带本轮文本；同时群聊 @ 提及改为仅本次回复的首条流式消息携带，后续轮次不再重复 @（避免一次回复多次提醒）
- **长期记忆 SQL 测试时间戳不稳定**：`pluginaichat` 的 `TestSQLMemoryManagerScenarios` / `TestMemoryStoreConformance` 在 Windows 上偶发失败——`time.Now()` 精度可能为微秒，同 scope 内两条记忆的 `created_at` 落盘格式相同，SQL 后端按 `id ASC` 排随机 ID、KV 后端按插入序，两者顺序不一致。测试现于同 scope 二次写入前保证 `created_at` 严格递增，KV 与 SQL 序列断言稳定
- **Discord 撤回通知显示删除者**：此前删除事件不携带删除者，`OperatorId` 恒空，日志渲染为「被  撤回」。现经审计日志尽力解析（管理删除会落审计条目、本人自删不落，据「无匹配条目」推断自删；条目按作者+频道+时近匹配，需 View Audit Log 权限，无权限或作者未入缓存时操作者留空）；日志插件在操作者为空时降级渲染「被撤回」（飞书等平台同步受益）

## [v4.2.1] - 2026-08-03

### Fixed

- 修复配额管理统计的一些问题

## [v4.2.0] - 2026-08-03

### 新增

- **Discord 适配器**（`bot/adapter/discord`，平台标识 `discord`，基于 `bwmarrin/discordgo` v0.29，Gateway WebSocket 收事件，**无需公网地址、无需部署协议端**）：
  - **接入方式**：Bot Token 鉴权 + Gateway WebSocket 接收事件（心跳/断线重连/会话 resume 由 discordgo 内部维护，连接失败指数退避无限重试）；intents 订阅 Guilds/GuildMessages/DirectMessages/**MessageContent（特权意图，需在 Developer Portal 开启，否则 close 4014 拒绝连接）**/消息反应，`bot.discord.member_events` 可选追加 Server Members 特权意图（成员进出）；`bot.discord.proxy` 同时作用于 REST（含附件下载）与 WebSocket 网关拨号
  - **消息收发翻译**：`<@id>`/`<@!id>` 提及原位映射 at 段（@bot 提及产出 `qq=SelfId` 的 at 段，群聊 @ 触发 AI 对话开箱即用），@everyone、角色/频道/自定义表情标记降级字面文本，引用回复映射 reply 段；图片附件下载转 data URI 供 AI 插件加载（失败保留 CDN 链接，注意 Discord CDN 签名链接约 24h 过期），视频/语音/文件附件映射通用段，贴纸降级 `[贴纸]` 文本；出站文本（超 1990 字符分包）/@提及（`<@id>`）/@everyone/引用回复（MessageReference）/图片/文件/语音/视频（附件本地下载重传——Discord 不抓取外链；连续媒体合批 ≤10 文件/条；超约 25 MiB 附件跳过不拖累整条链），`AllowedMentions` 收敛 @ 权限防止 AI 文本字面 `@everyone` 误触全服通知；DM 经 `UserChannelCreate` 打开私聊频道后发送
  - **通知与平台事件**：消息删除映射公共撤回通知（删除者 Discord 不告知、作者从运行期缓存反查）、表情回应映射群消息表情回应通知；机器人进/出服务器（`discord.bot_added`/`discord.bot_removed`）与成员进出（`discord.guild_member_add`/`discord.guild_member_remove`）走平台事件——成员事件携带服务器 ID 而非频道 ID，不映射公共进出通知以保护 GroupId 频道可寻址不变量
  - **查询与流式**：历史消息经 `ChannelMessages` API 拉取（单次最多 100 条，内存缓存兜底），`GetMsgDetail` 缓存未命中走 `ChannelMessage` API 并回写缓存，群详情经 `Channel`+`GuildWithCounts`（无需特权意图）；流式回复经 `ChannelMessageEdit` 打字机（600ms 节流，Discord 原生渲染 Markdown 无降级路径）
  - ID 前缀 `dc:`（用户 `dc:<user_id>`、频道 `dc:<channel_id>`、消息 `dc:<channel_id>:<message_id>` 复合编码）；core 按 `EventKeyer`（消息 MessageKey + 撤回 NoticeKey）对网关 resume 重放去重；拦截插件 `群ID:用户ID` 规则解析支持 `dc:` 前缀
- **首次设置向导支持 Discord 接入**：Bot Token（敏感字段留空不修改）与代理配置

### 修复

- **Token 统计补齐派生 LLM 调用消耗**：此前面板 Token 统计（数据源为 Query 日志与定时任务日志）只统计主对话/任务主循环自身的 LLM 调用，`team_run` 团队成员、各类 `subagent` 子代理（会话异步/定时任务）、上下文压缩器与备用图片识别（OCR）的 token 消耗完全未计入（前两者仅进了每日配额，后两者配额也未计），启用这些功能后面板数字明显低估实际消耗且与配额口径不一致。现统一改为「并入父请求」口径：
  - 子代理执行（`runSubagentWithOptions`）返回完整 `TokenUsage`（含其工具回调派生的 OCR 消耗），会话路径（异步子代理、`team_run` 成员）计入会话级用量暂存，由该会话下一次 Query 日志收尾时并入当次条目（同步派生计入当次请求、异步派生计入下一条 Query，统计语义为「会话粒度的总成本」）；定时任务路径的子代理用量并入任务级累计器，随任务日志与配额一并结算
  - 上下文压缩器改为返回 token 用量（`CompressorFunc` 签名变更），`ChatBot.Chat` 把当次压缩消耗并入返回的 usage（只算 token 字段，不计工具循环轮数与 LastPromptTokens，避免污染压缩阈值判断），Query/任务日志与配额随之自动覆盖
  - 备用图片识别（`GetSingleImageDesc` 返回用量）经回调 usageSink 上报：主会话并入会话暂存 + 配额，子代理/定时任务并入各自的总用量
  - 配额计数相应调整：子代理配额由内部统一累加改为调用方各自累加（定时任务子代理随任务总用量结算，避免重复计数）
- `llmclient` 新增 `GenerateSingleWithUsage`（单次生成返回 token 用量），`GenerateSingle` 改为其包装

## [v4.1.0] - 2026-08-03

### 新增

- **QQ 官方机器人适配器**（`bot/adapter/qqofficial`，平台标识 `qqofficial`，resty + gorilla/websocket 手写 QQ 开放平台 API v2 客户端，零新增依赖）：
  - **接入方式**：AppID/AppSecret 经 `getAppAccessToken` 换取 access_token（自动到期刷新、401 自动失效重取；旧版 Token 鉴权已废弃不支持），WebSocket 网关（`GET /gateway` → wss）接收事件，REST OpenAPI 发送消息；**无需公网地址、无需部署协议端**；支持沙箱环境开关（`bot.qqofficial.sandbox`，机器人未上架前联调）与 API 地址自定义
  - **WebSocket 网关**：OpCode 2 Identify / OpCode 6 Resume 断线会话恢复（seq 补发遗漏事件）、周期心跳 + 双周期未 ACK 僵尸连接主动断开重连、服务端 OpCode 7/9 重连与会话失效指令处理、指数退避重连循环；intents 固定订阅 `GROUP_AND_C2C_EVENT (1<<25)`（群@消息/单聊消息/好友与群机器人事件，频道场景不在范围）
  - **消息收发翻译**：群@事件自动注入 `qq=SelfId` 的 at 段（官方群事件本就是 @机器人才推送，aichat 群聊 @ 触发开箱即用）；文本/图片/语音（优先 WAV 链接）/视频/文件附件映射通用段，mentions 映射 at 段，引用消息（message_type=103）正文为空时取引用内容兜底；出站文本/图片/语音/视频/文件/引用回复（`message_reference`），at 段按平台限制静默退化（官方群消息不支持 @ 成员），无显式 reply 段时自动隐式引用触发消息（被动回复凭证的 msg_id），引用气泡即群聊回复 UX 的主要表达
  - **被动回复机制**：入站事件 msg_id 记录为会话回复凭证（群聊 5 分钟内最多 5 次、单聊 60 分钟内最多 4 次，`msg_seq` 递增），凭证过期/次数耗尽或收到 304026/304027/304103/40034005/40034024/40034128 时自动降级主动消息重试一次
  - **媒体上传**：URL 直传（`/files`，平台下载转存）+ 本地字节**分片上传**（`upload_prepare` → 逐片 PUT 预签名 URL → `upload_part_finish` → `/files` 合并，携带整文件 md5/sha1/md5_10m 与分片 md5 校验），base64/data/file 源（meme、send_file 工具产物）均可发送
  - **Markdown 渲染**（`bot.qqofficial.markdown`，默认关）：AI 文本回复以 msg_type=2 Markdown 消息发送（标题/加粗/列表富文本渲染），被拒（22006/50055/50056/50057/304036/40034124/40034127）自动降级纯文本
  - **平台限制如实退化**：官方无消息历史/单条消息查询/群资料 API——`GetMsgDetail`/历史由内存缓存兜底（每会话最近 200 条，消息 ID 全局索引反查），`GetGroupDetail` 返回不支持；无消息编辑 API 不支持流式回复（自动退化一次性发送）；好友添加映射 `FriendAddNotice`，机器人被拉群/移出/删除、通知开关变更走 `OnPlatformEvent`（`qqofficial.*`）
  - ID 前缀 `qo:`（openid 为 per-AppID 身份：同一用户群聊 member_openid 与单聊 user_openid 不同，与 NapCat 数字 QQ 号无关）；core 按 `EventKeyer.MessageKey`（消息 ID）去重官方明确的「相同 msg_id 可能重复推送」
- **首次设置向导支持 QQ 官方接入**：平台步骤可勾选 QQ 官方机器人并填写 AppID/AppSecret（敏感字段留空不修改）与沙箱开关；平台配置字段经注册表自动渲染进「配置管理」页（QQ 官方适配器组）
- 文档：配置详解新增 `qqofficial` 适配器章节（接入步骤、配置键、平台能力/限制清单），快速开始/项目介绍/Web 面板指南同步补充 QQ 官方平台说明

### 修复

- **QQ 官方适配器支持群消息全量模式**：后台开启「接收所有消息」后平台推送 `GROUP_MESSAGE_CREATE`（群内每条消息）而非仅 `GROUP_AT_MESSAGE_CREATE`，此前该事件被忽略导致群内 @ 无响应；现两种事件共用群消息翻译：AT 事件必然注入 `qq=SelfId` 的 at 段，全量事件按 mentions 是否含机器人自身决定是否注入（非 @ 消息不带 at 段、正常流经插件链，与 NapCat 语义一致，AI 仍只在被 @ 时响应）；机器人自己发送的消息（author.id 等于自身）被过滤防止自我循环
- **QQ 官方全量模式 @ 识别修复**：READY 的 `user.id` 与群聊场景 `member_openid` 不同源，按 ID 比对 mentions 永远不匹配（@ 消息未注入 SelfId at 段，AI 不响应）；现按官方 User 结构的 `bot` 标志识别机器人条目并辅以用户名比对（同群多机器人场景，ID 全等兜底），@ 的是其他机器人时不误触发；全量模式 content 残留的 `<@openid>` 提及标记（文档称已剥离，实测未剥离）按 mentions 自动剥离，不再污染 AI 输入

## [v4.0.0] - 2026-08-02

🎉 v4.0.0版本正式发布，新增飞书、telegram平台接入

### 新增

- **多平台适配器框架**：`common/adapter` 抽象为公共契约（`Adapter`）+ 平台专属能力（`QQExt` 可选接口）+ 适配器注册表（`Definition`/`Register`/`RegisterBotWrapper`）。新增平台只需实现 `Adapter`、提供 `Definition` 并在 `cmd/main.go` 空白导入触发注册即可，框架核心零改动；支持 QQ + 飞书等多平台并存，按配置 `bot.platform.<name>.enable` 启用
- **飞书（Lark）适配器**（`bot/adapter/feishu`，基于 `larksuite/oapi-sdk-go/v3`）：WebSocket 长连接（默认，无需公网地址）/ Webhook 双模式事件订阅；消息收发翻译（文本/@提及/富文本/图片/文件/回复），图片与文件经 `im.messageResource.get` 下载为 data URI 供 AI 插件直接加载；撤回/表情回应/成员进出等通知映射到公共事件，机器人入群、卡片回调等平台特定事件走新增的 `OnPlatformEvent` 统一入口；`bot.QQ` 类型断言探测 QQ 专属能力
- **Telegram 适配器**（`bot/adapter/telegram`，resty 手写 Bot API 客户端，零新增 SDK 依赖）：**Bot API 长轮询**（getUpdates，无需公网地址/协议端），支持 `bot.telegram.proxy`（HTTP/SOCKS5 代理）与 `bot.telegram.api_base`（自建 Bot API 网关/反代）；消息收发翻译（文本/@提及/图片/文件/语音/视频/回复），图片经 `getFile` 下载为 data URI 供 AI 插件直接加载；@bot 提及映射为 at 段（@username 大小写不敏感匹配 getMe 自身，群聊 @ 触发 AI）；成员进出/表情回应映射公共通知（成员变动仅管理员或关闭隐私模式可见），机器人被拉群/移出走 `OnPlatformEvent`；**流式回复**（`editMessageText` 打字机，600ms 节流，@username 前缀在每次更新保留）；ID 前缀 `tg:`，消息 ID 编码为 `tg:<chat_id>:<message_id>`；长轮询 at-least-once 语义下适配器级 update_id 去重 + core 按消息 ID 兜底；Bot API 无消息历史/单条查询端点，`GetMsgDetail`/历史由内存缓存兜底（每会话最近 200 条）；文本超 4096 分包、媒体 caption 1024 上限截断
- **平台 ID 前缀体系**：QQ 历史数字 ID 无前缀（存量数据零迁移），其他平台 ID 统一加前缀（如飞书 `fs:`），core 按前缀路由到对应适配器；`QID` 放松为任意字符串（数字仍规范化）
- **插件平台声明**：`plugin.Meta.Platforms` 字段（空 = 支持全部平台，向后兼容），core 按事件来源平台过滤插件；防撤回插件声明 QQ-only（依赖合并转发与 rkey）
- 面板状态接口返回各平台适配器状态数组（`GET /api/status` → `adapters`），概览页按平台展示连接状态
- 飞书发消息改用 **post + markdown 渲染**：文本走 `msg_type=post` 的 `md` 元素，飞书客户端原生渲染标题/加粗/代码块/列表等；@提及拆为独立 at 元素（保证通知送达），图片元素追加到正文末尾
- 首次设置向导支持**多平台接入**：平台步骤可勾选 QQ(NapCat) 与/或飞书与/或 Telegram 并分别填写连接配置（飞书含 App ID/Secret 与 webhook 参数，Telegram 含 Bot Token/代理），管理员 ID 支持带平台前缀的字符串，至少启用一个平台
- **Telegram Markdown 渲染**（`bot.telegram.parse_mode`，可选 `off`/`html`/`markdown`/`markdownv2`）：配置后一次性文本发送与流式回复的最终编辑按所选模式渲染 AI 的 markdown 输出（标题/加粗/代码块/列表等）。`html`=推荐（新默认）：适配器内把 AI markdown 转换为 Telegram HTML 再发送，任意输入都不会解析失败；`markdown`=旧版 Telegram 原生解析（仅需转义 `_ * [ ]`，词中下划线不解析）；`markdownv2`=新版原生解析（转义严格）。流式中间编辑保持纯文本（流式过程标记不完整），原生模式遇未转义特殊字符或 4096 截断切断闭合标记时 Telegram 返回 400，自动降级纯文本发送/编辑（还原未转换原文），不影响现有纯文本行为


### 变更

- `common/adapter` 接口拆分：`Adapter` 仅保留公共能力（发群/私聊消息、查消息/群详情/历史），合并转发、戳一戳、群签到、rkey、AI 语音、表情回应、好友/群列表等 QQ 专属能力移入可选接口 `QQExt`；对应插件侧 `bot.Bot` 保留公共方法，QQ 专属能力移入 `bot.QQ` 可选接口，插件在事件回调中类型断言探测（`if qb, ok := b.(bot.QQ); ok`）
- 插件事件回调不再收到裸 `*core.AniaBot`，而是平台能力包装后的 `bot.Bot`（`adapter.WrapBot`）：事件来源适配器实现 `QQExt` 时断言 `bot.QQ` 成功，否则失败（其他平台插件无感退化）
- `bot.admin_id` 由 int 改为 string（支持带平台前缀的 ID）；请求拦截/每日新闻插件的群号/QQ号名单由 `[]int` 改为 `[]string`（支持 `fs:` 前缀 ID）
- AI 对话插件定时任务与 Prompt 覆盖的目标 ID 解析支持多平台：纯数字（QQ）规范化为 QID，带前缀（如 `fs:oc_xxx`）原样保留
- `plugin.ai_chat_bot.stream.enable` 帮助文案由「飞书卡片实时更新」扩为「飞书卡片/Telegram 消息实时更新」（Telegram 适配器同样实现了流式回复，见上；原文案只提及飞书）
- **核心事件幂等去重服务**（消息体系重构 Phase A）：core 在事件进入插件链前统一去重，新增可选能力接口 `adapter.EventKeyer`（`MessageKey`/`NoticeKey`），适配器可提供稳定去重键；未实现时消息按「平台 + MessageId」兜底（NapCat/OneBot 由此获得去重），通知不做组合兜底避免误伤。存储复用 `storage.Storage`（内存后端原子 check-and-set，redis 后端 SET NX EX，多实例共享），TTL 10 分钟有界；飞书适配器实现 `MessageKey`（按 message_id，勿用 event_id 的原则写进接口注释），其早期适配器级去重保留
- **消息模型字段修复**（消息体系重构 Phase D）：`GroupRecallNotice.MessageId`/`FriendRecallNotice.MessageId` 由 `uint` 改为 `QID`，飞书撤回通知不再丢失字符串消息 ID（此前仅记日志）；新增可选能力接口 `adapter.SelfIDProvider`，事件未携带 self_id 时（飞书首次被 @ 前的空窗期）core 用适配器兜底填充，修复自消息过滤与 @ 提及检测失效；飞书私聊消息 `MessageType` 对齐 OneBot v11（`private` + `sub_type=friend`）；飞书适配器实现 `EventKeyer.NoticeKey`（撤回通知按 message_id 去重）
- **段能力声明 + 类型化段数据**（消息体系重构 Phase B）：typed 消息（TextMessage/MentionMessage/ImageMessage/ReplyMessage/FileMessage/RecordMessage/VideoMessage/FaceMessage/MusicMessage）新增 `Marshal()` 成为段数据构造的单一事实来源，msgchain 构造器与飞书适配器入站翻译统一迁移；修复图片/视频段同时写 `file` 与 `url`（ParseImage/ParseVideo 依赖 url，此前 Bot 发送的图片在 FriendlyText/历史回放中丢 URL）、`ParseFile` 补读 `name` 键；新增可选能力接口 `adapter.SegmentSupport`（`SupportedSegments()`），core 发送前对平台不支持的段类型计数告警（第 1 次与每 100 次），替代飞书出站静默丢弃（NapCat 声明 OneBot v11 全量段，飞书声明 text/at/image/reply/file/record）
- **飞书发送者昵称解析**（消息体系重构 Phase E）：飞书消息事件本身不含发送者昵称（此前日志与 AI 消息前缀显示空昵称 `[nickname: id:fs:ou_...]`），适配器在异步分发时经通讯录 `contact/v3/user/get` 解析昵称（带缓存、2s 超时、需 `contact:user.base:readonly` 权限；权限缺失/查询失败静默降级）；`FriendlyText` 昵称不可得时兜底显示「用户」
- **流式回复（打字机）**（消息体系重构 Phase C，`plugin.ai_chat_bot.stream.enable` 默认开启）：
  - LLM 层：`LLMClient.GenerateStream` 走 openai-go `NewStreaming`，流式累积内容/reasoning_content/按 Index 组装的工具调用/末块 token 用量；重试与备用模型语义与一次性一致，**首字节后失败不重试**（避免重复输出）
  - 工具循环：`ChatOptions` 新增 `OnStreamDelta`/`OnStreamRoundEnd`（nil 保持一次性，定时任务/子代理/团队调用方零改动）；工具边界结束当前流式消息，下一轮首块创建新消息；流式模式下 inter-round `SendText` 不再重复发送
  - 发送层：新增可选能力接口 `bot.StreamSender`/`adapter.StreamSenderExt`（`SendGroupStream`/`SendFriendStream` 返回可更新的 `StreamHandle{Patch/End}`）；core 按 ID 前缀路由委托；**QQ/OneBot v11 无消息编辑 API，qqBot 显式覆盖返回不支持，断言失败自动退化一次性**
  - 飞书实现：以 schema 2.0 interactive 卡片创建消息，`im.message.patch` 节流更新（600ms），28KB 内容上限字节安全截断；@ 提及前缀在每次 Patch 保留不消失；支持回复目标
  - 主对话接线：首个增量懒创建消息（群聊携带本批全部 @ 提及），流式创建失败/平台不支持自动回退一次性路径；`/stop` 取消先收尾流式消息再发停止提示
- **面板多平台适配**：管理面板全面去除 QQ 框架残留假设
  - 定时任务页：目标 ID 校验由「纯数字」放宽为任意非空 ID（支持 `fs:oc_xxx` / `fs:ou_xxx`），「群号 / QQ 号」文案改为「群 ID / 用户 ID」，创建者不再硬编码 QQ 前缀；编辑弹窗支持修改触发对象类型/目标 ID/单次标记（后端 `ClockTaskUpdate` 新增 `target_type`/`target_id`/`run_once` 字段）
  - 记忆/知识库/团队/配额页的会话 scope 校验由 `^[gf]:\d+$` 放宽为 `^[gf]:.+$`（如 `g:fs:oc_xxx`），此前飞书会话的数据在面板上完全无法查看/管理；`ClockTaskInfo.CreatedBy` 由 uint64 改为 string，飞书创建者不再显示为 0
  - 日志插件（pluginlog）通知文本改用 `QID.String()` 渲染，飞书撤回/表情回应/成员进出不再记录为 0，并修复通知文本中消息 ID 的格式串错误
  - AI 场景提示词与工具描述去除「QQ 群聊 / 群号 / QQ号」措辞，改为平台中性的「会话 ID / 用户 ID」（多平台下 AI 不再误以为自己在 QQ 上）
  - Prompt 覆盖编辑器移除 `inputmode="numeric"`，联系人页表头、日志筛选标签、Quota reset 错误文案等同步改为平台中性表述；文档示例（cron 插件教程）同步更新为 `[]string` 群 ID

### 修复

- **Telegram markdown 渲染实际从未生效（流式与一次性发送同因）**：此前把 AI 原始 markdown 直接交给 Telegram 原生 Markdown/MarkdownV2 解析器——MarkdownV2 要求转义 `.` `!` `(` `)` `-` `+` `#` 等十余个字符、旧版 Markdown 遇未配对 `` _ * [ ` `` 也会失败，AI 输出几乎必然触发 400 而静默降级纯文本，消息恒为纯文本展示。新增 `html` 渲染模式并设为新默认：适配器内把 AI markdown 转换为 Telegram HTML（新增 `markdown.go`，支持标题/加粗/斜体/删除线/行内代码/围栏代码块/链接/引用；未配对标记原样保留为文本、词中下划线不解析），对任意输入都产出合法 HTML、不会触发解析 400；html 模式下 400 兜底降级还原未转换的原文重发（避免把 HTML 标签当纯文本发出）。存量安装需在面板「Telegram 适配器 → 消息渲染模式」手动切换为 `html`（配置默认值不覆盖已存配置）
- **流式平台（Telegram/飞书）query 的 token 用量丢失、未计入 Token 统计**：流式 usage 提取仅识别「空 Choices 的独立末块」形态（OpenAI 标准）；DeepSeek 部分响应、智谱等提供方把 usage 附带在 finish_reason 块（Choices 非空），或网关忽略 `stream_options` 直接附带在最后一个内容块——这些形态下 usage 被全部丢弃，走流式路径的 TG/飞书 query 的 token 恒为 0（QQ 走一次性请求不受影响）；改为任何块只要 usage 字段非零即提取，并补充回归测试
- **流式最终 Markdown 渲染被内容去重误杀**：`End` 时内容与最后一条纯文本编辑一致时直接跳过最终编辑，Markdown 渲染从未真正尝试——消息恒为纯文本、日志无任何输出；现带 parse_mode 的最终编辑不受内容去重限制（渲染生成的实体使消息内容变化，Telegram 接受编辑，参考 aiogram 流式写法「流式 ParseMode.NONE、最终无条件应用 Markdown」）；最终渲染失败且内容已展示时跳过降级重发，消除 `message is not modified` 噪音
- **Telegram 错误码解析丢失**（此前「流式回复没发完」的真实根因）：resty 只把 2xx 响应的 body 解析进 `SetResult`，**4xx/5xx 的错误 JSON 不会填充**——官方 API 的 `error_code`/`description` 全部丢失，一切 400 错误（如 MarkdownV2 解析失败）都显示为 `telegram api error 0 (http 400)`，导致 400 降级纯文本重发从不触发，流式最终内容停留在节流窗口内未发出的旧版本；`unpack` 改为自行解析响应 body（429 识别与 MarkdownV2 纯文本降级恢复），错误日志附带 HTTP 状态码与响应体片段（截断 200 字符）便于诊断；响应体不可解析（错误页/空/截断）时 5xx/未知仍按瞬时故障保留 parse_mode 原样重试一次，4xx 按确定性拒绝直接纯文本降级；流式最终内容与已展示内容一致时跳过无意义的最终编辑（消除 `message is not modified` 400 噪音）
- 飞书**图文（post 富文本）消息被静默丢弃**：接收事件的 post content 是顶层 `title`/`content` 结构（无 `zh_cn` 包装，与发送/API 侧不同），`parsePostContent` 强制要求 `zh_cn` 导致翻译为空段，`onReceive` 静默跳过——面板无日志、AI 不响应；现兼容两种结构（zh_cn 包装 + 顶层，顶层 content 为空时回退 content_v2），并修复 post 消息 at 元素占位符（`@_user_N`）未反查 open_id、直接输出 `fs:@_user_N` 的问题，新增回归测试
- 飞书流式回复卡片 JSON 结构错误：schema 2.0 卡片中 `elements` 必须位于 `body` 下，此前放在根层级导致发消息/更新消息被 API 拒绝（code 230099 / 200621，`unknown property: elements`），已修正并更新测试
- **Telegram `splitEntities` 字节偏移与 UTF-16 偏移混用**：字符串切片游标（字节）被拿去和 Telegram 实体的 UTF-16 code unit 偏移比较做重叠/越界检查，含 CJK/emoji 的实体后方实体被误判越界丢弃——CJK 加粗后跟 @提及 时 at 段丢失（用户收不到通知）；改为字节游标与 UTF-16 游标分离，并补充 CJK/text_mention 回归测试
- **AI 工具执行器并发崩溃与同名工具重复定义**：同轮并行工具调用中若包含 `mcp_load_*`（写 session 工具表）与其它工具（读工具表），触发 `concurrent map read and map write` 致命错误（recover 无法捕获，进程直接退出）；`SessionToolExecutor` 增加互斥锁与快照拷贝。同时修复 `toolsWithSession` 简单拼接不去重：会话动态工具与全局工具同名时（如 MCP 工具与内置工具撞名）向 LLM 发送重复 function 定义导致提供方 400、且定义与执行解析不一致；现按名去重、会话层优先（与 `resolveTool` 一致），并补充并发回归测试
- **上下文压缩失败降级截断切开 tool_call 配对**：`truncateOldestHalf` 切点落在 tool 结果消息上时，保留区首条成为孤立 tool 消息（其 assistant tool_calls 在被丢弃的一半里），OpenAI 兼容 API 拒绝（400）且截断已落盘、此后每轮请求都失败；现切点后移越过孤立 tool 消息，并补充边界对齐回归测试。同文件修复 `Chat` 落盘起点按 `1+len(history)` 反推：历史中含旧版遗留 system 消息被 `BuildChatMessages` 过滤时索引错位，本轮用户消息/回复被跳过、不写入持久化历史；现直接记录构建后的真实长度
- **定时任务防叠加失效与超时可溢出**：cron 的 `SkipIfStillRunning` 包装的是任务闭包，而闭包仅经 `bot.Go` 异步派发后微秒级返回，锁立即释放——长任务（默认 120s+）到期再次触发时并发叠加执行；改为按任务 ID 的 running 标志（覆盖 cron 与立即执行两条路径），执行期间再次触发记日志跳过。同时 `TimeoutSec` 未设上限，面板/AI 可写入超大值致 `int→Duration` 溢出为负、context 立即过期任务秒败；现钳制到 1800s 上限（插件默认超时同），并补充回归测试
- **去重存储故障时 fail-closed 静默丢消息**：`tryClaimEvent` 把 `SetString` 的 false 一律当重复投递丢弃，redis 断连期间所有消息被静默吞掉；现失败后用 `GetString` 复核——键存在→真重复丢弃，键不存在→存储故障 fail-open 放行并告警（at-least-once 语义下宁可重复处理也不丢消息），补充故障注入回归测试。同时修复 redis `ScanKeys` 的 `count` 语义：此前仅作为 SCAN 每批数量提示透传、返回全部匹配键，与内存后端（count 为结果数量上限）不一致；现达到上限即截断返回
- **NapCat HTTP 适配器 `httpClient` 空指针**：仅在 `Serve` 中初始化，首次启动等待设置向导期间插件调用发送接口即 nil panic；改为构造时就绪（与 ws 适配器预建 `ackMng` 同理），并补充回归测试。同时修复 WS 适配器 token 直接拼接 URL query：token 含 `+` `/` `=` `&` 等字符时破坏 query 解析、NapCat 侧鉴权拿到错误 token（握手 401 且排查困难），现经 `url.QueryEscape` 编码
- **配额汇总 nil 解引用顺序错误**：`Summary` 先执行 `q.now()` 再判 `q == nil`，配额未启用（管理器为 nil）时面板配额页直接 panic；nil 检查移至最前，并补充 nil 接收者回归测试

## [v3.7.0] - 2026-08-01

### 新增

- Agent 团队 `team_run` 支持 leader 分工：主 AI 可为不同成员填写专属任务（members[].task），把不同任务派发给不同成员（如后端程序员审查后端代码、测试工程师执行测试）；成员未填专属任务时执行顶层总体任务，填写后先收到总体任务作为背景再执行专属任务
- 新增每日 Token 配额限制（`plugin.ai_chat_bot.quota.*`，默认关闭）：按「每会话每日」与「全局每日」两个维度限制 AI 消耗，超限后 AI 请求被拒绝（群聊/私聊收到提示）；主对话、子代理、Agent 团队成员、AI 定时任务全部计入所属会话的配额；计数持久化到 `quota:` 命名空间（重启不丢、按日期键惰性清理），被拒请求不产生 Query 日志记录；面板新增「配额管理」页（全局用量概览 + 会话用量明细 + 单会话/全部清零，`GET /api/quota` / `POST /api/quota/reset`）
- 新增 LLM 请求重试（`plugin.ai_chat_bot.retry.max_attempts` 默认 3 / `retry.base_delay_sec` 默认 2）：429/5xx/网络错误时指数退避重试（带随机抖动、尊重请求超时剩余预算），主对话、子代理、定时任务、OCR 统一生效；openai-go SDK 已内置 408/409/429/5xx 重试（默认 2 次），此配置为应用层补充（主要覆盖 SDK 不重试的网络错误）
- 新增备用模型自动切换（`plugin.ai_chat_bot.fallback.model`，空表示不启用；base_url/api_key 留空回退主模型）：主对话与上下文压缩在主模型重试耗尽或遇到不可重试错误时自动改用备用模型重试一次，主模型 API 故障时对话不再整轮失败
- 子代理与 AI 定时任务支持独立模型配置（`plugin.ai_chat_bot.subagent.base_url/api_key/model`，留空回退主模型；Agent 团队成员复用子代理配置）：可用更便宜的模型跑子任务
- 上下文压缩支持独立模型配置（`plugin.ai_chat_bot.compressor.base_url/api_key/model`，留空回退主模型）：可用更便宜的模型做历史摘要，降低压缩成本
- 面板登录防爆破：按来源 IP 统计登录失败次数，10 分钟窗口内连续失败 5 次即锁定 10 分钟（返回 HTTP 429 与 `Retry-After` 头），登录成功自动清零计数；失败响应附加固定 500ms 延迟拖慢在线爆破；记录数超阈值时惰性清理过期记录防止内存无界增长；纯内存计数，重启后清零。来源 IP 提取仅在直连对端为回环地址时（本机反代场景）才信任 `X-Forwarded-For` / `X-Real-IP`，防止外部伪造头部变换身份绕过锁定
- 面板登录/锁定/登录成功均记录 slog 日志（含来源 IP），爆破尝试可在控制台日志页直接观测

### 优化

- 同一轮 LLM 返回的多个工具调用并行执行（此前串行逐个执行）：结果按原顺序回填保证与 assistant 消息的 tool_calls 配对，工具观察者回调与 QQ 发送等回调经互斥串行化（无数据竞争），单个工具 panic 转为错误文本不中断整轮
- 面板密码校验改用 `subtle.ConstantTimeCompare` 常数时间比较，消除计时侧信道；存储值哈希段长度不符 SHA-256 时直接拒绝
- AI 对话 / 每日新闻插件的初始化失败改为返回 `fmt.Errorf("%w: 具体原因（含配置键名）", aniaerror.ParameterInitializeError)` 包装错误并交由框架统一记录，消除插件侧重复日志与裸哨兵错误丢失上下文的问题；插件开发文档（patterns.md）的初始化示例同步更新为新范式

### 修复

- 修复面板登录页密码错误时提示「未登录或会话已过期」的问题：前端请求封装对 401 统一吞掉服务端错误体，现优先展示服务端返回的具体错误（如「密码错误」「失败次数过多」）
- 修复插件生命周期错误被静默丢弃的问题：`Start` / `StartCron` / `Awake` 返回的错误此前被框架直接忽略（如 AI 插件未配置 API KEY、每日新闻 cron 注册失败时启动日志无任何报错），现统一经 `logError` 记录；`logError` 新增 `context.Canceled` 分支，用户 `/stop` 主动取消不再误记为「执行错误」

### 变更

- 长期记忆检索支持语义向量混合打分：`plugin.ai_chat_bot.kb.embedding` 启用时，记忆入库自动计算向量，`memory_search` 在关键词打分基础上叠加语义相似度加分（权重与知识库一致），同义不同词（如「喜爱」vs「喜欢」）也能命中；未启用 embedding 时保持纯关键词（与旧行为一致），旧数据（无向量字段）自动跳过语义加分
- `common/aniaerror` 移除未被使用的 `UnknownError`、`NetworkError`、`JsonSeralizeError`（原名有拼写错误）与 `Timeout`（`context.DeadlineExceeded` 别名），仅保留实际使用的 `ParameterInitializeError`

## [v3.6.0] - 2026-07-31

### 新增

- 新增 Agent 团队功能：主 AI 可通过 `team_run` 工具组建多代理团队，把同一任务并行派发给多个成员代理（每个成员以全新上下文运行、互不可见），全部完成后汇总各成员结果返回主 AI 做最终综合——成员即带角色系统提示词的一次性子代理，复用子代理执行引擎（独立超时、工具轮数上限、结果截断、回调隔离），无法再组建团队或委派子代理；成员支持三种指定方式：内联自定义角色描述（优先级最高）、预置角色（规划师/研究员/程序员/代码审查员/分析师/编辑）、当前会话已保存团队或全局团队中的成员名（未识别的名称降级为普通子代理并在报告中标注）；自定义团队按作用域持久化到 `team:` 命名空间——群聊/私聊 scope 由 AI 通过 `team_create` / `team_list` / `team_delete` 工具管理，全局团队（`global`，所有会话共享）由 Web 面板管理，均跨重启保留；面板新增「Agent 团队」管理页（按作用域查看团队定义、增删改团队与成员，成员行支持预置角色下拉一键填充），改动即时生效；随 `plugin.ai_chat_bot.team.enable` 门控（默认关闭），成员默认超时/工具轮数/结果长度/并行成员数均可配置（默认 5 个、硬上限 10）
- 新增知识库功能：文档按作用域（全局 `global` / 群聊 `g:群号` / 私聊 `f:QQ号`）管理，持久化到 `kb:` 命名空间；长文档入库时自动切片（约 600 字符/块、块间重叠），检索命中块而非整篇；AI 对话通过 `kb_search` / `kb_add` 工具按需检索或记录资料，并支持每次对话前自动关键词检索注入相关片段（`plugin.ai_chat_bot.kb.auto_inject`，默认开启，走纯关键词不产生额外 API 成本）；检索默认基于中文二元组切分 + 局部 IDF 加权打分，零外部依赖；可选开启向量检索（`plugin.ai_chat_bot.kb.embedding.enable`）用 OpenAI 兼容 `/embeddings` 混合打分，provider 不支持时自动退回纯关键词；面板新增「知识库」管理页（增删改查 + Jina Reader URL 导入），改动即时生效无需重启
- 面板新增「控制台日志」页：实时查看 Bot 运行时的控制台输出——捕获 slog 结构化日志（核心/插件/面板）与标准库 `log` 输出（适配器/工具类），终端风格按级别着色展示（debug/info/warn/error/log），支持级别筛选、自动刷新、滚动加载更早记录与清空显示；日志保存在内存环形缓冲（最多保留最近 2000 条），重启后清空；新增 `GET /api/consolelogs` 分页接口（`limit` / `before` 游标，`{"items": [...], "has_more": bool}`），捕获层放在核心 logger，原控制台输出行为不变

### 修复

- 修复 MCP 工具定义 `required` 数组顺序随机打失前缀缓存的问题：MCP inputSchema 缺少顶层 `required` 键时，`extractRequiredFromProperties` 遍历 map 生成 required 切片，每次请求顺序不同，直接破坏上游 prompt 前缀缓存（与 v3.5.0 修复同类）；现按名称排序后输出，并给 `getToolNames`、`SkillManager.List`、`skill_read` 附属文件清单等工具结果文本的 map 遍历统一补排序，彻底消除非确定性输出
- 修复 HTTP 适配器默认零认证的问题：未配置 `bot.adapter.token`（或配置为空串）时拒绝全部上报并提示配置方式（fail-closed），防止伪造事件注入冒充管理员；token 比较由大小写不敏感改为精确匹配；`SendPokeMsg` 补充 OneBot status 校验，不再对 `status=failed` 报假成功
- 修复 AI 定时任务越权：`/clock` 命令的 `del` / `on` / `off` / `info` / `timeout` 与 AI 工具的 `clock_update` / `clock_delete` / `clock_log` 此前按任务 ID 直接操作、无归属校验（ID 为自增序号可枚举），任意群成员或任意会话的 AI 可删除/查看其他会话的任务；现统一校验任务归属（只能操作当前会话的任务，管理员豁免），`clock_create` / `clock_list` 的显式跨会话目标同样拒绝，`clock_log` 未指定任务时只返回本会话日志；`tasklog` 新增按触发对象过滤的 `RecentForTarget`
- 修复上下文压缩失败导致整轮对话失败、用户消息丢失的问题：`MaybeCompress` 压缩请求失败（网络抖动/限流）时不再返回错误，改为丢弃最旧一半历史降级截断，本轮用户消息正常处理与落盘；同时压缩器输出的摘要消息由 system 角色改为 user 角色、不再拼接 basePrompt，消除压缩后请求中 system prompt 出现两份的问题（basePrompt 此前被重复注入且会被反复带进二次摘要）
- 修复 bash 工具忽略调用方 ctx 的问题：`/stop` 或请求超时后命令继续跑满 2 分钟、占满会话锁与并发槽；现基于调用方 ctx 派生超时，取消请求立即终止命令；输出截断改为按 rune 计算，避免切坏多字节 UTF-8
- 修复 Jina 搜索/浏览与面板 URL 导入不检查 HTTP 状态码且无请求超时的问题：401/402/429/5xx 的错误页文本此前会被当搜索结果返回给模型、当知识文档入库；现均校验状态码并设置请求超时（搜索 30s、导入 60s）
- 修复 MCP 工具错误详情被丢弃与结果无截断的问题：`IsError` 时保留服务器在 Content 中回传的具体错误文本供模型纠正参数；MCP 工具结果按 8000 字符截断、`skill_read` 按 16000 字符截断、`get_msg_history` count 上限 30，防止超大结果直接撑爆下一轮 LLM 上下文并拖垮后续压缩
- 修复 `top_k` 配置项完全无效的问题：此前 `ChatOptions.TopK` 从未下发到 LLM 请求，现经 openai-go 的 `SetExtraFields` 原样下发（`top_k` 为非标准参数，DeepSeek 等兼容 API 支持）
- 修复 embedding 调用无超时与不校验返回数量的问题：`kb_add` / `kb_search` 在 embedding 服务无响应时永久挂起，现内部强制 30s 超时；返回向量数量与输入不一致时整体退回关键词检索，避免向量错配到错误文本块
- 修复定时任务无防重入的问题：cron 实例改用 `SkipIfStillRunning` 链，上一次执行未结束时跳过一次触发，防止短周期任务（如 `@every 30s`）执行超时后并发叠加重复推送、重复消耗 API 额度
- 修复 goroutine panic 恢复回调无二次恢复的问题：插件 `OnPanic` 实现再 panic 会传播出 goroutine 直接终止整个进程，现逐个插件包一层 recover，并立即释放 per-plugin 的 context 定时器
- 修复复读机对纯图片/表情消息误判的问题：`RawMessage` 为空串时所有此类消息被判定为"相同"，连续 3 条不同图片消息即触发复读；现空消息不参与复读比较
- 修复每日新闻插件 cron 表达式非法时静默不注册的问题：`StartCron` 现在检查 `AddFunc` 错误并返回，避免"看似启动成功但从不播报"
- 修复未 @ 消息计数非原子与清理被跳过仍清零的问题：并发消息可能丢计数；AI 长响应中拿不到会话锁时保留计数继续累计，响应结束后才触发历史自动清理

## [v3.5.0] - 2026-07-30

### 新增

- 面板 Token 统计页重构为「总览 + 细节」两区并支持时间维度筛选：总览区固定为全量口径（历史累计 / 今日 / 缓存命中 / 单次平均，不随筛选变化）；细节区可按 全部 / 今日 / 昨日 / 近 7 天 / 近 30 天 / 本月 / 自定义日期范围（起止日期，最长 62 天）聚合统计卡、分来源序列、来源 / 会话类型 / 状态拆分、24 小时分布与目标排行，单天窗口（今日 / 昨日 / 单日自定义）主图自动切换为当日 24 小时分来源序列；`GET /api/tokenstats/detail` 相应新增 `range`（及 custom 用的 `start` / `end`）查询参数，服务端直接映射为日志的 Start/End 过滤，`hourly` 序列改为与 daily 同构的分来源结构，响应新增 `range` 字段

### 变更

- 给 AI 的图片统一引入短哈希标识：消息文本（含回复引用、合并转发、msg_history 历史）中的图片标记由 `[图片:<url>]` 改为 `[图片 <hash>]`（SHA-256 前 8 位 hex，按图片 URL 计算，不再把冗长的临时签名链接塞进 prompt）；`load_images` 加载结果与多模态上下文消息中每张图片前均附带同一 `[图片 <hash>]` 标签，备用图片识别模型（OCR 兜底）的描述也以 `<图片 hash>` 标注替代原序号，`local_image` 工具结果同样带哈希——AI 可凭哈希在多张图片间准确区分与引用；历史落盘/回放的图片降级标记同步保留哈希（`[图片 <hash>]` / `[图片 <hash>，链接已失效]`）
- 面板消息日志 / Query 日志 / 定时任务执行日志改为服务端分页 + 前端滚动加载：`GET /api/msglogs` `/api/querylogs` `/api/tasklogs` 新增 `limit`（默认 50、最大 200）与 `before` 游标参数，响应结构由裸数组改为 `{"items": [...], "has_more": bool}`（调用方需同步适配）；消息日志利用「列表新在前 + ID 连续自增」把游标直接换算为列表偏移定位，任务/Query 日志按序号跳过游标之后记录，均不再全量读取；前端三个页面刷新只拉最新一页合并头部（已有条目原地更新状态），消息日志滚动到顶部、Query/任务日志滚动到底部时自动加载更早分页

### 修复

- 修复面板消息日志中群表情回应通知的操作者 QQ 恒为 0 的问题：`GroupMsgEmojiLikeNotice` 按错误的 `operator_id` 字段解析，而 NapCat 实际上报的是 `user_id`；现更正为 `UserId`（同时修正 `likes` 的表情 ID 字段为 `emoji_id` 字符串）
- 修复 LLM 请求 prompt 前缀缓存命中率极低的问题：工具定义列表（`ToolExecuter.toolsWithSession`）与注入 system prompt 的 skill 注册表（`SkillManager.BuildAvailableSkillsPrompt`）此前直接遍历 Go map 序列化，输出顺序每次请求随机变化，导致上游 context cache（如 DeepSeek）从前缀第 0 个 token 起即不匹配、命中率接近 0；现两处均按名称排序输出，保证请求前缀完全确定，同一会话内历史部分可稳定命中缓存

## [v3.4.0] - 2026-07-29

### 新增

- 定时任务支持委派子代理：任务 AI 可通过 `subagent_run` 把复杂子任务交给一次性子代理在后台并行执行（`subagent_list` / `subagent_cancel` 可查看与取消）；由于子代理是异步的，任务收尾时会自动等待全部子代理返回，把结果回喂给任务 AI 合成最终回复——只有这最后一轮输出才推送给目标，子代理返回前的中间回复不推送；子代理超时按任务剩余预算自动压缩，随 `plugin.ai_chat_bot.subagent.enable` 门控

### 变更

- 面板概览页 CPU 负载曲线改由服务端缓存：面板启动后后台每 5 秒采样一次 CPU 占用率并保留最近约 10 分钟历史（120 点），`/api/host` 快照新增 `cpu_history` 字段；前端打开页面直接渲染完整曲线，不再从单个数据点开始重新积累，CPU 采样窗口也不再受请求频率影响
- 面板登录会话改为滑动过期：剩余有效期不足 12 小时时，任意一次请求自动顺延至 24 小时并刷新 Cookie，活跃用户不再被固定 24 小时到期强制下线；闲置满 24 小时仍会过期，改密/重置密码吊销全部会话的行为不变
- 面板修改密码不再要求输入原密码：已登录会话本身即为凭据，弹窗只保留新密码输入框；`PUT /api/password` 相应移除 `old_password` 字段
- 修正 `plugin.ai_chat_bot.rate_limit` 的面板标签与文档描述：该配置实际是 AI 请求并发上限（信号量语义），原"速率限制（次/秒）"描述有误，现改为"并发限制"并补充说明
- AI 对话历史落盘前将图片片段统一降级为文本标记：此前 `local_image` 等工具载入的 base64 内联图片（单张可达 MB 级）会随历史整体反复全量重写，撑大持久化单 key 并造成写放大（MySQL `MEDIUMTEXT` 超限还会导致落盘静默失败）；现仅内存中的当前会话保留图片，落盘副本一律存 `[图片]` 文本标记，重启回放不受影响
- AI 历史压缩增加 usage 缺失时的兜底触发：上游不报 prompt tokens 时改用字符数粗估（约 2 字符折 1 token、图片按固定值估算）判断是否超过 80% 上下文阈值，避免压缩永不触发导致历史无限增长
- 长期记忆单条内容增加 2000 字符上限（`memory_save` / 面板编辑统一截断，`memory_save` 工具描述已注明），避免单条超长内容撑大按 scope 整体存储的记忆 key
- 任务日志 / Query 日志的面板查询改为逆序逐条加载、凑够 limit 即停，不再每次把全部记录（数百条）逐键读出后再过滤
- MySQL 持久化存储的 UPSERT 由 `REPLACE INTO`（delete+insert 两次行变更）改为 `INSERT ... ON DUPLICATE KEY UPDATE` 原地更新，降低写放大；`VALUES(col)` 写法在 MySQL 5.7/8.x 与 MariaDB 均兼容（8.0.20+ 仅为弃用告警）

## [v3.3.0] - 2026-07-27

### 新增

- 面板配置管理新增配置预设功能：可将当前全部配置（含密钥、MCP / Prompt 覆盖）保存为命名快照，支持一键应用切换、同名覆盖更新与删除；应用预设仅覆盖快照中包含的配置键，不影响其他键，重启后生效
- 定时任务执行日志记录完整执行过程：任务内容、LLM 轮数、工具调用明细（名称/参数/结果/耗时）与最终回复，面板「定时任务」页参考 Query 日志展示
- 面板新增「Token 统计」独立页面：按来源（对话/定时任务）、会话类型（群聊/私聊）、执行状态、消耗目标排行（Top 10）、24 小时分布与最近 30 天分来源每日序列等多维度统计 token 用量
- 主对话最大工具调用轮数支持面板配置：新增配置项 `plugin.ai_chat_bot.max_iterations`（默认 20），主对话与定时任务统一生效
- 每日新闻插件新增启用/禁用开关：新增配置项 `plugin.dailynews.enable`（默认 true），关闭后停止定时播报并忽略 `/news` 命令
- 面板概览页新增 AI API 余额卡片：启用配置项 `bot.balance.enable` 后，按声明式配置请求余额接口——`bot.balance.url` / `bot.balance.headers` / `bot.balance.body`（支持 `${base_url}` `${api_key}` `${model}` 占位符，取自 AI 对话插件配置）、`bot.balance.method`（GET/POST），显示模板 `bot.balance.format` 中的 `{gjson 路径}` 会被替换为响应 JSON 中对应字段的值（默认适配 DeepSeek 风格 `/user/balance` 接口）；结果按 `bot.balance.cache_sec`（默认 300 秒）缓存，支持手动强制刷新，配置改动即时生效无需重启

### 变更

- 面板概览页的定时任务管理（新建/编辑/删除/启停）合并到「任务日志」页，侧边栏该项更名为「定时任务」
- 移除 `MAX_ITERATIONS` 环境变量，工具调用轮数统一由 `plugin.ai_chat_bot.max_iterations` 配置项控制
- 面板 NapCat 适配器 HTTP 配置项标注更明确：「HTTP 监听端口」更名「本地监听端口」、「HTTP 目标地址」更名「NapCat HTTP 地址」，帮助文本注明与 NapCat 侧「HTTP 客户端」（上报）和「HTTP 服务器」（调用）配置的对应关系

### 修复

- HTTP 适配器接收 NapCat 事件上报时校验 token：配置 `bot.adapter.token` 后，未携带正确 `Authorization: Bearer <token>` 头（或 `access_token` 查询参数）的上报请求将被拒绝（401），防止伪造事件注入
- HTTP 适配器本地服务器启动失败（如端口被占用）时明确输出错误日志，不再静默失效
- HTTP 适配器上报接口仅接受 POST 请求，其他方法返回 405

## [v3.2.0] - 2026-07-27

### 新增

- Token 消耗监控功能：支持总量、今日及最近 14 天的每日用量统计
- 定时任务执行日志查询功能：支持条件筛选与展示

### 变更

- 移除不必要的 `EstimateTokens` 和 `countRunes` 函数，优化代码结构

## [v3.1.1] - 2026-07-26

### 新增

- 异步子代理功能：支持复杂任务的委派与管理
- 跳过无效用户的子代理消息，优化消息处理逻辑
- 面板首页与插件卡片新增功能展示与图标

### 文档

- 添加容器部署指南并链接至快速开始文档
- 更新 docker.md 中关于源码目录挂载的说明

## [v3.1.0] - 2026-07-26

### 新增

- AI 子代理（subagent）功能：主 AI 可将复杂/耗时任务委派给临时子代理执行

### 文档

- 移除 .gitignore 中对 custom/plugins 的忽略规则

## [v3.0.0] - 2026-07-26

### 新增

- 消息日志改用缓存存储，Redis 驱动下重启后保留
- 请求拦截插件配置更新，添加群号和 QQ 号名单说明
- Web 面板配置说明添加环境变量覆盖提示

### 修复

- OnPanic 用互斥锁保护 lastPanicTime，消除并发 panic 上报时的数据竞争
- HasMention 对 at 段 qq 字段改用 comma-ok 断言，避免异常数据触发 panic
- 更新检查改用完整 commit 哈希比较，避免短哈希长度不一致误报新版本
- get_msg_history 参数改用 desc 标签并将 count 设为可选
- bash 工具自定义环境变量改为追加到继承环境，不再整体替换
- 工具重复注册改为跳过并记录日志，不再 panic
- convertMessage 非图片用户消息拼接全部文本片段，不再只取第一段
- AI 会话状态统一按 g:/f: 前缀键索引，修复同号群聊与私聊互串
- 命令行重置面板密码时同步吊销所有已签发会话
- 内存缓存 matchPattern 支持中间通配段，与 Redis 后端匹配语义对齐
- meme 工具请求传入 ctx 并设置 30s 超时，避免接口挂起时永久阻塞
- bash 工具非零退出码时不再丢弃 stdout/stderr
- 上下文压缩判断改用最后一次调用的 prompt token，避免累加值虚高触发过早压缩
- HTTP 适配器消息过滤规则与 WS 适配器对齐，行为不再随传输方式变化
- HTTP 适配器补齐 API 层状态检查，失败响应不再返回 success=true
- WS 请求 echo 追加原子序号，避免同纳秒并发请求的 echo 碰撞串音
- MCPToolManager.toolCache 加互斥锁，修复并发 AI 会话下的致命 map 并发写
- 内存缓存 ScanKeys 改持写锁，修复 RLock 下删除过期键导致的致命并发冲突
- WS 适配器 ACK 帧改为 readLoop 直接投递，避免 worker 池自饿死
- 字体改为本地打包，移除 Google Fonts CDN 依赖

### 文档

- 修正内置插件文档、架构图、插件开发文档与配置文档中的多处过时内容
- 拦截插件文档补充 whitelist 群聊的 AND 语义说明
- cron.md 示例改用 message.FromUint64 构造 QID
- 修正 README 快速开始中的构建命令

[Unreleased]: https://github.com/jeanhua/AniaBot/compare/v3.1.1...HEAD
[v3.1.1]: https://github.com/jeanhua/AniaBot/compare/v3.1.0...v3.1.1
[v3.1.0]: https://github.com/jeanhua/AniaBot/compare/v3.0.0...v3.1.0
[v3.0.0]: https://github.com/jeanhua/AniaBot/compare/v2.2.2...v3.0.0
