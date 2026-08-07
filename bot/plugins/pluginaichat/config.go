package pluginaichat

// AI 对话插件的配置结构体。实现 plugin.ConfigSchemaProvider 后，框架启动时
// 自动注册字段（面板渲染 + 默认值补齐），并在 Start 前填充完成——Start 里
// 直接读 p.cfg，无需再手写 cfg.Get* 逐个读取。
//
// 框架级共享键（files.mcp_json / files.prompt_json）不属于本插件配置，
// 仍在 Start 中通过 viper 读取。

// 默认系统提示词（与 Prompt 字段 default 标签保持一致；Start 中作为空值兜底）
const defaultPrompt = `你是一个ai对话机器人，在即时通讯平台上与用户聊天，说话不要长篇大论

## 注意
- 当你不理解用户的问题时，要先获取用户最近的历史消息，再根据历史消息回答用户的问题`

type thinkingConfig struct {
	Enable bool   `cfg:"enable" label:"启用深度思考" group:"AI 对话 · 模型" default:"false"`
	Mode   string `cfg:"mode" label:"思考模式" type:"select" options:"none,low,medium,high,auto" group:"AI 对话 · 模型" default:"auto"`
}

type searchConfig struct {
	Token string `cfg:"token" label:"Jina AI Token" type:"password" sensitive:"true" group:"AI 对话 · 模型" help:"网页浏览与搜索功能"`
}

type bashToolConfig struct {
	Enable    bool     `cfg:"enable" label:"启用 Bash 工具" group:"AI 对话 · 工具" help:"直接在宿主机执行 shell 命令，注意安全风险" default:"false"`
	Shell     string   `cfg:"shell" label:"Shell 路径" group:"AI 对话 · 工具" help:"留空使用系统默认（Linux/macOS 为 sh，Windows 为 cmd），可填 /bin/bash、/bin/ash 等"`
	Env       []string `cfg:"env" label:"环境变量" group:"AI 对话 · 工具" help:"KEY=VALUE，每行一个"`
	Whitelist []string `cfg:"whitelist" label:"命令白名单(正则)" group:"AI 对话 · 工具" help:"非空时仅允许匹配的命令，每行一个"`
	Blacklist []string `cfg:"blacklist" label:"命令黑名单(正则)" group:"AI 对话 · 工具" help:"匹配的命令被禁止，每行一个" default:"config(\\.dev)?\\.(yaml|yml|json),^mkfs,^shutdown,^reboot"`
}

type fileToolConfig struct {
	Enable bool `cfg:"enable" label:"启用文件发送工具" group:"AI 对话 · 工具" help:"可读取宿主机任意文件并发送给用户，默认关闭" default:"false"`
}

type localImageToolConfig struct {
	Enable bool `cfg:"enable" label:"启用本地图片工具" group:"AI 对话 · 工具" help:"可读取宿主机本地图片，默认关闭" default:"false"`
}

// memeToolConfig meme 表情包工具配置：请求地址模板 + gjson 解析路径，
// 任何「返回一组图片 URL」的接口都能接入，接口失效时改配置即可切换。
// 默认 GIPHY stickers（全球最大 GIF/表情包平台，需免费 API Key）
type memeToolConfig struct {
	URL      string `cfg:"url" label:"表情包接口地址" group:"AI 对话 · 工具" help:"请求地址模板，支持 ${msg}（搜索词）、${num}（数量）、${key}（API Key）占位符；默认 GIPHY，国内服务器访问需代理，也可换成任意返回图片数组的自定义接口" default:"https://api.giphy.com/v1/stickers/search?api_key=${key}&q=${msg}&limit=${num}"`
	Key      string `cfg:"key" label:"表情包 API Key" type:"password" sensitive:"true" group:"AI 对话 · 工具" help:"替换地址中的 ${key}；默认接口为 GIPHY，免费 Key 在 developers.giphy.com 申请；接口地址不含 ${key} 时留空即可"`
	ListPath string `cfg:"list_path" label:"结果数组路径" group:"AI 对话 · 工具" help:"gjson 路径，指向响应 JSON 中的图片数组" default:"data"`
	ImgField string `cfg:"img_field" label:"图片字段路径" group:"AI 对话 · 工具" help:"数组元素中图片 URL 的 gjson 路径，如 img_url 或 images.fixed_width.url" default:"images.fixed_width.url"`
	Num      int    `cfg:"num" label:"请求数量" group:"AI 对话 · 工具" help:"每次请求的结果数量，随机从中挑一张发送" default:"50"`
}

// configToolConfig AI 配置管理 / 重启工具配置：允许 AI 查看与修改框架配置
// （config_get / config_set，敏感字段对 AI 掩码，修改重启后生效），
// 以及通过 restart_bot 重启 Bot 使配置修改生效。
type configToolConfig struct {
	Enable        bool `cfg:"enable" label:"启用配置管理工具" group:"AI 对话 · 工具" help:"允许 AI 查看与修改框架配置（config_get/config_set，敏感字段对 AI 掩码，修改重启后生效），默认关闭" default:"false"`
	RestartEnable bool `cfg:"restart_enable" label:"启用重启工具" group:"AI 对话 · 工具" help:"允许 AI 重启 Bot 使配置修改生效（restart_bot），默认关闭" default:"false"`
}

// skillToolConfig AI Skill 管理工具配置：允许 AI 自行安装/卸载/查看技能
// （skill_list / skill_install / skill_remove），无需后台面板手动上传。
// 安装支持 zip 链接下载（含 GitHub 仓库自动转换）或直接撰写 SKILL.md 内容，
// 落盘后热重载立即生效；默认关闭以控制任意代码/提示注入风险。
type skillToolConfig struct {
	Enable bool `cfg:"enable" label:"启用 Skill 管理工具" group:"AI 对话 · 工具" help:"允许 AI 安装/卸载/查看技能（skill_list/skill_install/skill_remove）：AI 可先用 webSearch/webExplore 上网搜索技能资源，再从 zip 链接或 GitHub 仓库下载安装，也可直接撰写 SKILL.md 内容创建；安装后热重载立即生效，默认关闭" default:"false"`
}

// mcpConfig MCP 服务器加载策略配置
type mcpConfig struct {
	LazyLoad bool `cfg:"lazy_load" label:"MCP 工具懒加载" group:"AI 对话 · 工具" help:"开启后 MCP 工具按需发现/加载（mcp_discover/mcp_load），工具定义不进初始上下文、节省 token；但会话内动态加载会改变 tools 列表，可能降低上游 prompt 缓存命中率。关闭后启动时全量注册所有 MCP 工具（工具列表恒定、缓存友好，但工具较多时上下文开销大）" default:"true"`
}

// mcpToolConfig AI MCP 管理工具配置：允许 AI 自行添加/删除/重连/查看
// MCP 服务器（mcp_list / mcp_add / mcp_remove / mcp_reconnect），配置写入
// files.mcp_json 持久化并即时热注册生效；默认关闭以控制远程代码/工具注入风险。
type mcpToolConfig struct {
	Enable bool `cfg:"enable" label:"启用 MCP 管理工具" group:"AI 对话 · 工具" help:"允许 AI 添加/删除/重连/查看 MCP 服务器（mcp_list/mcp_add/mcp_remove/mcp_reconnect）：支持 stdio 命令与 streamable/sse HTTP 端点，配置写入 files.mcp_json 持久化并即时生效，默认关闭" default:"false"`
}

type ocrConfig struct {
	Enable      bool     `cfg:"enable" label:"启用备用识图模型" group:"AI 对话 · OCR" help:"主模型不支持多模态时将图片转文字" default:"false"`
	BaseURL     string   `cfg:"base_url" label:"Base URL" group:"AI 对话 · OCR" default:"https://api.siliconflow.cn/v1"`
	APIKey      string   `cfg:"api_key" label:"API Key" type:"password" sensitive:"true" group:"AI 对话 · OCR"`
	Model       string   `cfg:"model" label:"模型" group:"AI 对话 · OCR" default:"Qwen/Qwen3-VL-8B-Instruct"`
	MaxToken    *int     `cfg:"max_token" label:"最大输出 Token" group:"AI 对话 · OCR" default:"600"`
	Temperature *float64 `cfg:"temperature" label:"Temperature" group:"AI 对话 · OCR" default:"0.6"`
	TopP        *float64 `cfg:"top_p" label:"Top P" group:"AI 对话 · OCR" default:"0.95"`
	TopK        *int     `cfg:"top_k" label:"Top K" group:"AI 对话 · OCR" default:"20"`
	Prompt      string   `cfg:"prompt" label:"识图提示词" type:"text" group:"AI 对话 · OCR" default:"你负责将看到的图片用markdown格式描述出来，不要有无关的其他对话"`
}

type clockConfig struct {
	Enable            bool `cfg:"enable" label:"启用 AI 定时任务" group:"AI 对话 · 定时与记忆" default:"true"`
	DefaultTimeoutSec int  `cfg:"default_timeout_sec" label:"默认超时(秒)" group:"AI 对话 · 定时与记忆" default:"120"`
	MaxLogEntries     int  `cfg:"max_log_entries" label:"日志保留条数" group:"AI 对话 · 定时与记忆" default:"500"`
}

// promptCacheConfig 上游 prompt 缓存配置：仅 anthropic 格式需要显式声明
// cache_control 断点（chat_completions / responses 由提供方自动前缀缓存）。
// 断点打在 system 与最后一条消息上，system 内容保持稳定即可稳定命中；
// 动态内容（如未来要做的长期记忆注入）必须追加到消息尾部而非 system。
type promptCacheConfig struct {
	Enable bool   `cfg:"enable" label:"启用 Prompt 缓存" group:"AI 对话 · 模型" help:"anthropic 格式下为 system 与对话历史设置 cache_control 断点（需模型与上游支持）；chat_completions / responses 为自动前缀缓存，不受此开关影响" default:"true"`
	TTL    string `cfg:"ttl" label:"缓存保留时长" type:"select" options:"5m,1h" group:"AI 对话 · 模型" help:"仅 anthropic 格式有效：5m 写入成本 1.25x、1h 为 2x，读取均为 0.1x；会话间隔短留 5m，长时间闲置用 1h" default:"5m"`
}

type memoryConfig struct {
	Enable     bool `cfg:"enable" label:"启用长期记忆" group:"AI 对话 · 定时与记忆" default:"true"`
	MaxEntries int  `cfg:"max_entries" label:"单会话记忆上限" group:"AI 对话 · 定时与记忆" default:"200"`
	// AutoInject 每轮按用户消息纯关键词检索相关记忆并注入到用户消息前
	// （尾部注入：system 不变，不影响上游前缀缓存；用户消息不落盘，历史无污染）
	AutoInject bool `cfg:"auto_inject" label:"主动注入相关记忆" group:"AI 对话 · 定时与记忆" default:"false" help:"每轮按用户消息检索相关记忆并追加到用户消息前（system 保持不变，不影响上游前缀缓存；用户消息不落盘，历史无污染）。启用向量检索后按语义+关键词混合检索，否则纯关键词"`
	InjectMax  int  `cfg:"inject_max" label:"每轮注入条数上限" group:"AI 对话 · 定时与记忆" default:"3" help:"主动注入的最大记忆条数，0 表示不限制"`
}

type kbEmbeddingConfig struct {
	Enable  bool   `cfg:"enable" label:"启用向量检索" group:"AI 对话 · 知识库" default:"false" help:"入库时计算语义向量，检索与自动注入按语义+关键词混合打分（自动注入每轮一次 embed，带缓存）；启动时自动回填存量数据的向量。需要 embedding 服务支持，provider 不支持时自动退回纯关键词"`
	BaseURL string `cfg:"base_url" label:"Embedding Base URL" group:"AI 对话 · 知识库" help:"留空使用主模型的 Base URL；如主模型无 embedding 接口（如 DeepSeek），可填 https://api.jina.ai/v1 或其它 OpenAI 兼容服务"`
	APIKey  string `cfg:"api_key" label:"Embedding API Key" type:"password" sensitive:"true" group:"AI 对话 · 知识库" help:"留空使用主模型的 API Key（用 Jina 时可填 Jina AI Token）"`
	Model   string `cfg:"model" label:"Embedding 模型" group:"AI 对话 · 知识库" default:"jina-embeddings-v3" help:"如 jina-embeddings-v3、text-embedding-3-small、BAAI/bge-large-zh-v1.5"`
}

type kbConfig struct {
	Enable     bool `cfg:"enable" label:"启用知识库" group:"AI 对话 · 知识库" default:"true"`
	MaxDocs    int  `cfg:"max_docs" label:"单作用域文档上限" group:"AI 对话 · 知识库" default:"500"`
	AutoInject bool `cfg:"auto_inject" label:"自动注入上下文" group:"AI 对话 · 知识库" default:"true" help:"每次对话前自动检索相关文档并注入上下文。启用向量检索后按语义+关键词混合检索（每轮一次 embed，带缓存），未启用则纯关键词"`

	Embedding kbEmbeddingConfig `cfg:"embedding"`
}

type subagentConfig struct {
	Enable        bool   `cfg:"enable" label:"启用子代理" group:"AI 对话 · 子代理" help:"允许主 AI 把复杂子任务委派给一次性子代理执行，子代理拥有全部工具能力且上下文独立" default:"true"`
	TimeoutSec    int    `cfg:"timeout_sec" label:"默认超时(秒)" group:"AI 对话 · 子代理" default:"300"`
	MaxIterations int    `cfg:"max_iterations" label:"最大工具迭代轮数" group:"AI 对话 · 子代理" default:"10"`
	MaxResultLen  int    `cfg:"max_result_len" label:"结果最大字符数" group:"AI 对话 · 子代理" help:"子代理返回结果超出该长度时截断，防止污染主对话上下文" default:"4000"`
	BaseURL       string `cfg:"base_url" label:"子代理 Base URL" group:"AI 对话 · 子代理" help:"留空使用主模型配置；可填更便宜的模型以降低子任务成本"`
	APIKey        string `cfg:"api_key" label:"子代理 API Key" type:"password" sensitive:"true" group:"AI 对话 · 子代理" help:"留空使用主模型配置"`
	Model         string `cfg:"model" label:"子代理模型" group:"AI 对话 · 子代理" help:"留空使用主模型；子代理与 AI 定时任务共用该模型"`
	APIFormat     string `cfg:"api_format" label:"子代理 API 格式" type:"select" options:"chat_completions,responses,anthropic" group:"AI 对话 · 子代理" help:"留空跟随主模型格式"`
}

type teamConfig struct {
	Enable        bool `cfg:"enable" label:"启用 Agent 团队" group:"AI 对话 · Agent 团队" help:"允许主 AI 组建多代理团队，把子任务派发给多个成员代理并行执行" default:"false"`
	TimeoutSec    int  `cfg:"timeout_sec" label:"成员默认超时(秒)" group:"AI 对话 · Agent 团队" default:"300"`
	MaxIterations int  `cfg:"max_iterations" label:"成员最大工具迭代轮数" group:"AI 对话 · Agent 团队" default:"10"`
	MaxResultLen  int  `cfg:"max_result_len" label:"单成员结果最大字符数" group:"AI 对话 · Agent 团队" help:"每个成员返回的结果超出该长度时截断，防止汇总报告污染主对话上下文" default:"4000"`
	MaxMembers    int  `cfg:"max_members" label:"单次最多并行成员数" group:"AI 对话 · Agent 团队" default:"5"`
}

type queryLogConfig struct {
	Enable     bool `cfg:"enable" label:"启用 Query 日志" group:"AI 对话 · 查询日志" help:"在面板记录每次 AI 回复的完整执行过程（耗时、token、工具调用详情）" default:"true"`
	MaxEntries int  `cfg:"max_entries" label:"日志保留条数" group:"AI 对话 · 查询日志" default:"200"`
}

// compressorConfig 上下文压缩专用模型配置：留空回退主模型配置。
type compressorConfig struct {
	BaseURL   string `cfg:"base_url" label:"压缩器 Base URL" group:"AI 对话 · 模型" help:"留空使用主模型配置"`
	APIKey    string `cfg:"api_key" label:"压缩器 API Key" type:"password" sensitive:"true" group:"AI 对话 · 模型" help:"留空使用主模型配置"`
	Model     string `cfg:"model" label:"压缩器模型" group:"AI 对话 · 模型" help:"留空使用主模型；建议填更便宜的模型降低历史压缩成本"`
	APIFormat string `cfg:"api_format" label:"压缩器 API 格式" type:"select" options:"chat_completions,responses,anthropic" group:"AI 对话 · 模型" help:"留空跟随主模型格式"`
}

// retryConfig 应用层重试配置：429/5xx/网络错误时指数退避重试。
type retryConfig struct {
	MaxAttempts  int `cfg:"max_attempts" label:"最大尝试次数" group:"AI 对话 · 模型" help:"0 或 1 表示不重试；429/5xx/网络错误时指数退避重试（SDK 已内置 429/5xx 重试，此值为应用层补充）" default:"3"`
	BaseDelaySec int `cfg:"base_delay_sec" label:"退避基准(秒)" group:"AI 对话 · 模型" help:"每次重试等待 基准×2^n 秒（带随机抖动）" default:"2"`
}

// fallbackConfig 备用模型配置：主模型重试耗尽或遇到不可重试错误时自动切换重试一次。
type fallbackConfig struct {
	BaseURL   string `cfg:"base_url" label:"备用模型 Base URL" group:"AI 对话 · 模型" help:"留空使用主模型配置"`
	APIKey    string `cfg:"api_key" label:"备用模型 API Key" type:"password" sensitive:"true" group:"AI 对话 · 模型" help:"留空使用主模型配置"`
	Model     string `cfg:"model" label:"备用模型" group:"AI 对话 · 模型" help:"留空表示不启用备用模型；主模型重试耗尽后自动切换重试一次（仅主对话与上下文压缩）"`
	APIFormat string `cfg:"api_format" label:"备用模型 API 格式" type:"select" options:"chat_completions,responses,anthropic" group:"AI 对话 · 模型" help:"留空跟随主模型格式"`
}

// streamConfig 流式回复配置：平台支持「先发后改」时逐字展示回复
// （如飞书卡片/Telegram 消息实时更新）；平台不支持或出错时自动退化为一次性回复。
type streamConfig struct {
	Enable bool `cfg:"enable" label:"启用流式回复" group:"AI 对话 · 回复" help:"平台支持时逐字展示回复（飞书卡片/Telegram/Discord 消息实时更新）；不支持或出错时自动退化为一次性回复" default:"true"`
}

// quotaConfig 每日 Token 配额限制配置：按会话与全局两个维度限制每日消耗。
type quotaConfig struct {
	Enable            bool `cfg:"enable" label:"启用每日配额限制" group:"AI 对话 · 配额" help:"按会话与全局两个维度限制每日 Token 消耗，超限后 AI 请求被拒绝" default:"false"`
	DailyTokens       int  `cfg:"daily_tokens" label:"每会话每日 Token 上限" group:"AI 对话 · 配额" help:"0 表示不限制；超出后该会话当日 AI 请求将被拒绝（含子代理、定时任务消耗）" default:"0"`
	GlobalDailyTokens int  `cfg:"global_daily_tokens" label:"全局每日 Token 上限" group:"AI 对话 · 配额" help:"0 表示不限制；所有会话合计消耗超限后 AI 请求全部拒绝" default:"0"`
}

// sessionConfig 会话内存回收配置：限制内存中驻留的 ChatBot 会话数量与时长，
// 防止活跃会话增多导致内存线性增长。淘汰只丢弃内存对象，对话历史已持久化，
// 下次发言时自动重建会话并回放历史。
type sessionConfig struct {
	MaxIdleMinutes int `cfg:"max_idle_minutes" label:"闲置会话回收（分钟）" group:"AI 对话 · 会话" default:"120" help:"超过该时长无 AI 交互的会话从内存淘汰（历史在数据库，下次发言自动恢复；会话内 mcp_load 动态加载的工具会随淘汰失效，等同重启）；0 表示不按闲置淘汰"`
	MaxSessions    int `cfg:"max_sessions" label:"最大驻留会话数" group:"AI 对话 · 会话" default:"128" help:"内存中最多驻留的会话数，超出时淘汰最久未活跃的；0 表示不限制"`
}

type aiChatConfig struct {
	BaseURL          string `cfg:"plugin.ai_chat_bot.base_url" label:"Base URL" group:"AI 对话 · 模型" help:"API 地址；anthropic 格式填 https://api.anthropic.com，其余填 OpenAI 兼容地址" default:"https://api.deepseek.com"`
	APIKey           string `cfg:"plugin.ai_chat_bot.api_key" label:"API Key" type:"password" sensitive:"true" group:"AI 对话 · 模型"`
	Model            string `cfg:"plugin.ai_chat_bot.model" label:"模型" group:"AI 对话 · 模型" default:"deepseek-chat"`
	APIFormat        string `cfg:"plugin.ai_chat_bot.api_format" label:"API 格式" type:"select" options:"chat_completions,responses,anthropic" group:"AI 对话 · 模型" help:"chat_completions：OpenAI 兼容（DeepSeek 等，默认）；responses：OpenAI Responses API；anthropic：Anthropic Messages API（Claude）" default:"chat_completions"`
	Multimodal       bool   `cfg:"plugin.ai_chat_bot.multimodal" label:"多模态" group:"AI 对话 · 模型" help:"主模型是否支持图片输入" default:"false"`
	RateLimit        int    `cfg:"plugin.ai_chat_bot.rate_limit" label:"并发限制" group:"AI 对话 · 模型" help:"同时处理的 AI 请求数上限，超出后直接拒绝" default:"2"`
	MaxContextTokens int    `cfg:"plugin.ai_chat_bot.max_context_tokens" label:"上下文 Token 上限" group:"AI 对话 · 模型" default:"128000"`
	MaxIterations    int    `cfg:"plugin.ai_chat_bot.max_iterations" label:"最大工具调用轮数" group:"AI 对话 · 模型" help:"单次回复中 AI 最多连续调用工具的轮数，超出后强制结束" default:"20"`
	// 指针字段：nil 表示不向下游 LLM 传该参数（保持未设置语义）
	MaxToken    *int     `cfg:"plugin.ai_chat_bot.max_token" label:"最大输出 Token" group:"AI 对话 · 模型" default:"8192"`
	Temperature *float64 `cfg:"plugin.ai_chat_bot.temperature" label:"Temperature" group:"AI 对话 · 模型" default:"1.2"`
	TopP        *float64 `cfg:"plugin.ai_chat_bot.top_p" label:"Top P" group:"AI 对话 · 模型" default:"0.9"`
	TopK        *int     `cfg:"plugin.ai_chat_bot.top_k" label:"Top K" group:"AI 对话 · 模型" default:"100"`
	// 与 defaultPrompt 常量保持一致（标签内 \n 会被解析为换行）
	Prompt   string         `cfg:"plugin.ai_chat_bot.prompt" label:"系统提示词" type:"text" group:"AI 对话 · 模型" default:"你是一个ai对话机器人，在即时通讯平台上与用户聊天，说话不要长篇大论\n\n## 注意\n- 当你不理解用户的问题时，要先获取用户最近的历史消息，再根据历史消息回答用户的问题"`
	Thinking thinkingConfig `cfg:"plugin.ai_chat_bot.thinking"`
	Search   searchConfig   `cfg:"plugin.ai_chat_bot.search"`

	SkillsDir  string               `cfg:"plugin.ai_chat_bot.skills_dir" label:"Skills 目录" group:"AI 对话 · 工具" default:"./skills"`
	Skills     []string             `cfg:"plugin.ai_chat_bot.skills" label:"Skills 白名单" group:"AI 对话 · 工具" help:"为空则加载全部，每行一个"`
	Bash       bashToolConfig       `cfg:"plugin.ai_chat_bot.bash"`
	File       fileToolConfig       `cfg:"plugin.ai_chat_bot.file"`
	LocalImage localImageToolConfig `cfg:"plugin.ai_chat_bot.local_image"`
	Meme       memeToolConfig       `cfg:"plugin.ai_chat_bot.meme"`
	ConfigTool configToolConfig     `cfg:"plugin.ai_chat_bot.config_tool"`
	SkillTool  skillToolConfig      `cfg:"plugin.ai_chat_bot.skill_tool"`
	MCP        mcpConfig            `cfg:"plugin.ai_chat_bot.mcp"`
	MCPTool    mcpToolConfig        `cfg:"plugin.ai_chat_bot.mcp_tool"`

	OCR ocrConfig `cfg:"plugin.ai_chat_bot.ocr"`

	Clock       clockConfig       `cfg:"plugin.ai_chat_bot.clock"`
	PromptCache promptCacheConfig `cfg:"plugin.ai_chat_bot.prompt_cache"`
	Memory      memoryConfig      `cfg:"plugin.ai_chat_bot.memory"`
	Kb          kbConfig          `cfg:"plugin.ai_chat_bot.kb"`
	Subagent    subagentConfig    `cfg:"plugin.ai_chat_bot.subagent"`
	Team        teamConfig        `cfg:"plugin.ai_chat_bot.team"`
	QueryLog    queryLogConfig    `cfg:"plugin.ai_chat_bot.query_log"`
	Compressor  compressorConfig  `cfg:"plugin.ai_chat_bot.compressor"`
	Retry       retryConfig       `cfg:"plugin.ai_chat_bot.retry"`
	Fallback    fallbackConfig    `cfg:"plugin.ai_chat_bot.fallback"`
	Stream      streamConfig      `cfg:"plugin.ai_chat_bot.stream"`
	Quota       quotaConfig       `cfg:"plugin.ai_chat_bot.quota"`
	Session     sessionConfig     `cfg:"plugin.ai_chat_bot.session"`
}

// ConfigSchema 实现 plugin.ConfigSchemaProvider：返回配置结构体指针，
// 框架在 Start 前自动完成字段注册、默认值补齐与配置填充。
func (p *AIChatPlugin) ConfigSchema() any {
	return &p.cfg
}
