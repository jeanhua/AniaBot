package pluginaichat

// AI 对话插件的配置结构体。实现 plugin.ConfigSchemaProvider 后，框架启动时
// 自动注册字段（面板渲染 + 默认值补齐），并在 Start 前填充完成——Start 里
// 直接读 p.cfg，无需再手写 cfg.Get* 逐个读取。
//
// 框架级共享键（files.mcp_json / files.prompt_json）不属于本插件配置，
// 仍在 Start 中通过 viper 读取。

// 默认系统提示词（与 Prompt 字段 default 标签保持一致；Start 中作为空值兜底）
const defaultPrompt = `你是一个ai对话机器人，在QQ上和别人聊天，说话不要长篇大论

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
	Enable    bool     `cfg:"enable" label:"启用 Bash 工具" group:"AI 对话 · 工具" help:"直接在宿主机执行 bash 命令，注意安全风险" default:"false"`
	Shell     string   `cfg:"shell" label:"Shell 路径" group:"AI 对话 · 工具" default:"/bin/bash"`
	Env       []string `cfg:"env" label:"环境变量" group:"AI 对话 · 工具" help:"KEY=VALUE，每行一个"`
	Whitelist []string `cfg:"whitelist" label:"命令白名单(正则)" group:"AI 对话 · 工具" help:"非空时仅允许匹配的命令，每行一个"`
	Blacklist []string `cfg:"blacklist" label:"命令黑名单(正则)" group:"AI 对话 · 工具" help:"匹配的命令被禁止，每行一个" default:"config(\\.dev)?\\.(yaml|yml|json),^mkfs,^shutdown,^reboot"`
}

type localImageToolConfig struct {
	Enable bool `cfg:"enable" label:"启用本地图片工具" group:"AI 对话 · 工具" help:"可读取宿主机本地图片，默认关闭" default:"false"`
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

type memoryConfig struct {
	Enable     bool `cfg:"enable" label:"启用长期记忆" group:"AI 对话 · 定时与记忆" default:"true"`
	MaxEntries int  `cfg:"max_entries" label:"单会话记忆上限" group:"AI 对话 · 定时与记忆" default:"200"`
}

type queryLogConfig struct {
	Enable     bool `cfg:"enable" label:"启用 Query 日志" group:"AI 对话 · 查询日志" help:"在面板记录每次 AI 回复的完整执行过程（耗时、token、工具调用详情）" default:"true"`
	MaxEntries int  `cfg:"max_entries" label:"日志保留条数" group:"AI 对话 · 查询日志" default:"200"`
}

type aiChatConfig struct {
	BaseURL          string `cfg:"plugin.ai_chat_bot.base_url" label:"Base URL" group:"AI 对话 · 模型" help:"兼容 OpenAI 规范的 API 地址" default:"https://api.deepseek.com"`
	APIKey           string `cfg:"plugin.ai_chat_bot.api_key" label:"API Key" type:"password" sensitive:"true" group:"AI 对话 · 模型"`
	Model            string `cfg:"plugin.ai_chat_bot.model" label:"模型" group:"AI 对话 · 模型" default:"deepseek-chat"`
	Multimodal       bool   `cfg:"plugin.ai_chat_bot.multimodal" label:"多模态" group:"AI 对话 · 模型" help:"主模型是否支持图片输入" default:"false"`
	RateLimit        int    `cfg:"plugin.ai_chat_bot.rate_limit" label:"速率限制(次/秒)" group:"AI 对话 · 模型" default:"2"`
	MaxContextTokens int    `cfg:"plugin.ai_chat_bot.max_context_tokens" label:"上下文 Token 上限" group:"AI 对话 · 模型" default:"128000"`
	// 指针字段：nil 表示不向下游 LLM 传该参数（保持未设置语义）
	MaxToken    *int     `cfg:"plugin.ai_chat_bot.max_token" label:"最大输出 Token" group:"AI 对话 · 模型" default:"8192"`
	Temperature *float64 `cfg:"plugin.ai_chat_bot.temperature" label:"Temperature" group:"AI 对话 · 模型" default:"1.2"`
	TopP        *float64 `cfg:"plugin.ai_chat_bot.top_p" label:"Top P" group:"AI 对话 · 模型" default:"0.9"`
	TopK        *int     `cfg:"plugin.ai_chat_bot.top_k" label:"Top K" group:"AI 对话 · 模型" default:"100"`
	// 与 defaultPrompt 常量保持一致（标签内 \n 会被解析为换行）
	Prompt   string         `cfg:"plugin.ai_chat_bot.prompt" label:"系统提示词" type:"text" group:"AI 对话 · 模型" default:"你是一个ai对话机器人，在QQ上和别人聊天，说话不要长篇大论\n\n## 注意\n- 当你不理解用户的问题时，要先获取用户最近的历史消息，再根据历史消息回答用户的问题"`
	Thinking thinkingConfig `cfg:"plugin.ai_chat_bot.thinking"`
	Search   searchConfig   `cfg:"plugin.ai_chat_bot.search"`

	SkillsDir  string               `cfg:"plugin.ai_chat_bot.skills_dir" label:"Skills 目录" group:"AI 对话 · 工具" default:"./skills"`
	Skills     []string             `cfg:"plugin.ai_chat_bot.skills" label:"Skills 白名单" group:"AI 对话 · 工具" help:"为空则加载全部，每行一个"`
	Bash       bashToolConfig       `cfg:"plugin.ai_chat_bot.bash"`
	LocalImage localImageToolConfig `cfg:"plugin.ai_chat_bot.local_image"`

	OCR ocrConfig `cfg:"plugin.ai_chat_bot.ocr"`

	Clock    clockConfig    `cfg:"plugin.ai_chat_bot.clock"`
	Memory   memoryConfig   `cfg:"plugin.ai_chat_bot.memory"`
	QueryLog queryLogConfig `cfg:"plugin.ai_chat_bot.query_log"`
}

// ConfigSchema 实现 plugin.ConfigSchemaProvider：返回配置结构体指针，
// 框架在 Start 前自动完成字段注册、默认值补齐与配置填充。
func (p *AIChatPlugin) ConfigSchema() any {
	return &p.cfg
}
