# Changelog

本项目的所有重要变更都会记录在此文件中。

格式基于 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)，
版本号遵循 [语义化版本](https://semver.org/lang/zh-CN/)。

发布流程：打 `v*.*.*` 标签后，GitHub Release 的正文与 Docker 镜像描述
都会自动从本文件中对应版本段落提取，因此发版前请先更新对应版本的内容。

## [Unreleased]

### 新增

- **多平台适配器框架**：`common/adapter` 抽象为公共契约（`Adapter`）+ 平台专属能力（`QQExt` 可选接口）+ 适配器注册表（`Definition`/`Register`/`RegisterBotWrapper`）。新增平台只需实现 `Adapter`、提供 `Definition` 并在 `cmd/main.go` 空白导入触发注册即可，框架核心零改动；支持 QQ + 飞书等多平台并存，按配置 `bot.platform.<name>.enable` 启用
- **飞书（Lark）适配器**（`bot/adapter/feishu`，基于 `larksuite/oapi-sdk-go/v3`）：WebSocket 长连接（默认，无需公网地址）/ Webhook 双模式事件订阅；消息收发翻译（文本/@提及/富文本/图片/文件/回复），图片与文件经 `im.messageResource.get` 下载为 data URI 供 AI 插件直接加载；撤回/表情回应/成员进出等通知映射到公共事件，机器人入群、卡片回调等平台特定事件走新增的 `OnPlatformEvent` 统一入口；`bot.QQ` 类型断言探测 QQ 专属能力
- **平台 ID 前缀体系**：QQ 历史数字 ID 无前缀（存量数据零迁移），其他平台 ID 统一加前缀（如飞书 `fs:`），core 按前缀路由到对应适配器；`QID` 放松为任意字符串（数字仍规范化）
- **插件平台声明**：`plugin.Meta.Platforms` 字段（空 = 支持全部平台，向后兼容），core 按事件来源平台过滤插件；防撤回插件声明 QQ-only（依赖合并转发与 rkey）
- 面板状态接口返回各平台适配器状态数组（`GET /api/status` → `adapters`），概览页按平台展示连接状态
- 飞书发消息改用 **post + markdown 渲染**：文本走 `msg_type=post` 的 `md` 元素，飞书客户端原生渲染标题/加粗/代码块/列表等；@提及拆为独立 at 元素（保证通知送达），图片元素追加到正文末尾

### 优化

- 面板密码校验改用 `subtle.ConstantTimeCompare` 常数时间比较，消除计时侧信道；存储值哈希段长度不符 SHA-256 时直接拒绝
- AI 对话 / 每日新闻插件的初始化失败改为返回 `fmt.Errorf("%w: 具体原因（含配置键名）", aniaerror.ParameterInitializeError)` 包装错误并交由框架统一记录，消除插件侧重复日志与裸哨兵错误丢失上下文的问题；插件开发文档（patterns.md）的初始化示例同步更新为新范式

### 修复

- 修复面板登录页密码错误时提示「未登录或会话已过期」的问题：前端请求封装对 401 统一吞掉服务端错误体，现优先展示服务端返回的具体错误（如「密码错误」「失败次数过多」）
- 修复插件生命周期错误被静默丢弃的问题：`Start` / `StartCron` / `Awake` 返回的错误此前被框架直接忽略（如 AI 插件未配置 API KEY、每日新闻 cron 注册失败时启动日志无任何报错），现统一经 `logError` 记录；`logError` 新增 `context.Canceled` 分支，用户 `/stop` 主动取消不再误记为「执行错误」

### 变更

- `common/adapter` 接口拆分：`Adapter` 仅保留公共能力（发群/私聊消息、查消息/群详情/历史），合并转发、戳一戳、群签到、rkey、AI 语音、表情回应、好友/群列表等 QQ 专属能力移入可选接口 `QQExt`；对应插件侧 `bot.Bot` 保留公共方法，QQ 专属能力移入 `bot.QQ` 可选接口，插件在事件回调中类型断言探测（`if qb, ok := b.(bot.QQ); ok`）
- 插件事件回调不再收到裸 `*core.AniaBot`，而是平台能力包装后的 `bot.Bot`（`adapter.WrapBot`）：事件来源适配器实现 `QQExt` 时断言 `bot.QQ` 成功，否则失败（其他平台插件无感退化）
- `bot.admin_id` 由 int 改为 string（支持带平台前缀的 ID）；请求拦截/每日新闻插件的群号/QQ号名单由 `[]int` 改为 `[]string`（支持 `fs:` 前缀 ID）
- AI 对话插件定时任务与 Prompt 覆盖的目标 ID 解析支持多平台：纯数字（QQ）规范化为 QID，带前缀（如 `fs:oc_xxx`）原样保留
- `common/aniaerror` 移除未被使用的 `UnknownError`、`NetworkError`、`JsonSeralizeError`（原名有拼写错误）与 `Timeout`（`context.DeadlineExceeded` 别名），仅保留实际使用的 `ParameterInitializeError`

## [v3.7.0] - 2026-08-01

### 新增

- Agent 团队 `team_run` 支持 leader 分工：主 AI 可为不同成员填写专属任务（members[].task），把不同任务派发给不同成员（如后端程序员审查后端代码、测试工程师执行测试）；成员未填专属任务时执行顶层总体任务，填写后先收到总体任务作为背景再执行专属任务
- 新增每日 Token 配额限制（`plugin.ai_chat_bot.quota.*`，默认关闭）：按「每会话每日」与「全局每日」两个维度限制 AI 消耗，超限后 AI 请求被拒绝（群聊/私聊收到提示）；主对话、子代理、Agent 团队成员、AI 定时任务全部计入所属会话的配额；计数持久化到 `quota:` 命名空间（重启不丢、按日期键惰性清理），被拒请求不产生 Query 日志记录；面板新增「配额管理」页（全局用量概览 + 会话用量明细 + 单会话/全部清零，`GET /api/quota` / `POST /api/quota/reset`）
- 新增 LLM 请求重试（`plugin.ai_chat_bot.retry.max_attempts` 默认 3 / `retry.base_delay_sec` 默认 2）：429/5xx/网络错误时指数退避重试（带随机抖动、尊重请求超时剩余预算），主对话、子代理、定时任务、OCR 统一生效；openai-go SDK 已内置 408/409/429/5xx 重试（默认 2 次），此配置为应用层补充（主要覆盖 SDK 不重试的网络错误）
- 新增备用模型自动切换（`plugin.ai_chat_bot.fallback.model`，空表示不启用；base_url/api_key 留空回退主模型）：主对话与上下文压缩在主模型重试耗尽或遇到不可重试错误时自动改用备用模型重试一次，主模型 API 故障时对话不再整轮失败
- 子代理与 AI 定时任务支持独立模型配置（`plugin.ai_chat_bot.subagent.base_url/api_key/model`，留空回退主模型；Agent 团队成员复用子代理配置）：可用更便宜的模型跑子任务
- 上下文压缩支持独立模型配置（`plugin.ai_chat_bot.compressor.base_url/api_key/model`，留空回退主模型）：可用更便宜的模型做历史摘要，降低压缩成本

### 优化

- 同一轮 LLM 返回的多个工具调用并行执行（此前串行逐个执行）：结果按原顺序回填保证与 assistant 消息的 tool_calls 配对，工具观察者回调与 QQ 发送等回调经互斥串行化（无数据竞争），单个工具 panic 转为错误文本不中断整轮

### 变更

- 长期记忆检索支持语义向量混合打分：`plugin.ai_chat_bot.kb.embedding` 启用时，记忆入库自动计算向量，`memory_search` 在关键词打分基础上叠加语义相似度加分（权重与知识库一致），同义不同词（如「喜爱」vs「喜欢」）也能命中；未启用 embedding 时保持纯关键词（与旧行为一致），旧数据（无向量字段）自动跳过语义加分

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
