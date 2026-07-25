package pluginaichat

import "github.com/jeanhua/AniaBot/common/pluginconfig"

// 默认系统提示词（与历史默认配置一致）
const defaultPrompt = `你是一个ai对话机器人，在QQ上和别人聊天，说话不要长篇大论

## 注意
- 当你不理解用户的问题时，要先获取用户最近的历史消息，再根据历史消息回答用户的问题`

// 默认 OCR 识图提示词
const defaultOCRPrompt = `你负责将看到的图片用markdown格式描述出来，不要有无关的其他对话`

// ConfigFields 实现 plugin.ConfigRegistrar：声明 AI 对话插件的全部配置字段，
// 框架启动时补齐缺失的默认值，Web 面板据此动态渲染表单。
func (p *AIChatPlugin) ConfigFields() []pluginconfig.Field {
	return configFields
}

var configFields = []pluginconfig.Field{
	// 模型
	{Key: "plugin.ai_chat_bot.base_url", Label: "Base URL", Type: "string", Group: "AI 对话 · 模型", Help: "兼容 OpenAI 规范的 API 地址", Default: "https://api.deepseek.com"},
	{Key: "plugin.ai_chat_bot.api_key", Label: "API Key", Type: "password", Group: "AI 对话 · 模型", Sensitive: true},
	{Key: "plugin.ai_chat_bot.model", Label: "模型", Type: "string", Group: "AI 对话 · 模型", Default: "deepseek-chat"},
	{Key: "plugin.ai_chat_bot.multimodal", Label: "多模态", Type: "bool", Group: "AI 对话 · 模型", Help: "主模型是否支持图片输入", Default: false},
	{Key: "plugin.ai_chat_bot.rate_limit", Label: "速率限制(次/秒)", Type: "int", Group: "AI 对话 · 模型", Default: 2},
	{Key: "plugin.ai_chat_bot.max_context_tokens", Label: "上下文 Token 上限", Type: "int", Group: "AI 对话 · 模型", Default: 128000},
	{Key: "plugin.ai_chat_bot.max_token", Label: "最大输出 Token", Type: "int", Group: "AI 对话 · 模型", Default: 8192},
	{Key: "plugin.ai_chat_bot.temperature", Label: "Temperature", Type: "float", Group: "AI 对话 · 模型", Default: 1.2},
	{Key: "plugin.ai_chat_bot.top_p", Label: "Top P", Type: "float", Group: "AI 对话 · 模型", Default: 0.9},
	{Key: "plugin.ai_chat_bot.top_k", Label: "Top K", Type: "int", Group: "AI 对话 · 模型", Default: 100},
	{Key: "plugin.ai_chat_bot.thinking.enable", Label: "启用深度思考", Type: "bool", Group: "AI 对话 · 模型", Default: false},
	{Key: "plugin.ai_chat_bot.thinking.mode", Label: "思考模式", Type: "string", Group: "AI 对话 · 模型", Help: "none / low / medium / high / auto", Default: "auto"},
	{Key: "plugin.ai_chat_bot.prompt", Label: "系统提示词", Type: "text", Group: "AI 对话 · 模型", Default: defaultPrompt},
	{Key: "plugin.ai_chat_bot.search.token", Label: "Jina AI Token", Type: "password", Group: "AI 对话 · 模型", Sensitive: true, Help: "网页浏览与搜索功能"},

	// 工具
	{Key: "plugin.ai_chat_bot.skills_dir", Label: "Skills 目录", Type: "string", Group: "AI 对话 · 工具", Default: "./skills"},
	{Key: "plugin.ai_chat_bot.skills", Label: "Skills 白名单", Type: "strings", Group: "AI 对话 · 工具", Help: "为空则加载全部，每行一个", Default: []string{}},
	{Key: "plugin.ai_chat_bot.bash.enable", Label: "启用 Bash 工具", Type: "bool", Group: "AI 对话 · 工具", Help: "直接在宿主机执行 bash 命令，注意安全风险", Default: false},
	{Key: "plugin.ai_chat_bot.bash.shell", Label: "Shell 路径", Type: "string", Group: "AI 对话 · 工具", Default: "/bin/bash"},
	{Key: "plugin.ai_chat_bot.bash.env", Label: "环境变量", Type: "strings", Group: "AI 对话 · 工具", Help: "KEY=VALUE，每行一个", Default: []string{}},
	{Key: "plugin.ai_chat_bot.bash.whitelist", Label: "命令白名单(正则)", Type: "strings", Group: "AI 对话 · 工具", Help: "非空时仅允许匹配的命令，每行一个", Default: []string{}},
	{Key: "plugin.ai_chat_bot.bash.blacklist", Label: "命令黑名单(正则)", Type: "strings", Group: "AI 对话 · 工具", Help: "匹配的命令被禁止，每行一个", Default: []string{`config(\.dev)?\.(yaml|yml|json)`, `^mkfs`, `^shutdown`, `^reboot`}},
	{Key: "plugin.ai_chat_bot.local_image.enable", Label: "启用本地图片工具", Type: "bool", Group: "AI 对话 · 工具", Help: "可读取宿主机本地图片，默认关闭", Default: false},

	// OCR
	{Key: "plugin.ai_chat_bot.ocr.enable", Label: "启用备用识图模型", Type: "bool", Group: "AI 对话 · OCR", Help: "主模型不支持多模态时将图片转文字", Default: false},
	{Key: "plugin.ai_chat_bot.ocr.base_url", Label: "Base URL", Type: "string", Group: "AI 对话 · OCR", Default: "https://api.siliconflow.cn/v1"},
	{Key: "plugin.ai_chat_bot.ocr.api_key", Label: "API Key", Type: "password", Group: "AI 对话 · OCR", Sensitive: true},
	{Key: "plugin.ai_chat_bot.ocr.model", Label: "模型", Type: "string", Group: "AI 对话 · OCR", Default: "Qwen/Qwen3-VL-8B-Instruct"},
	{Key: "plugin.ai_chat_bot.ocr.max_token", Label: "最大输出 Token", Type: "int", Group: "AI 对话 · OCR", Default: 600},
	{Key: "plugin.ai_chat_bot.ocr.temperature", Label: "Temperature", Type: "float", Group: "AI 对话 · OCR", Default: 0.6},
	{Key: "plugin.ai_chat_bot.ocr.top_p", Label: "Top P", Type: "float", Group: "AI 对话 · OCR", Default: 0.95},
	{Key: "plugin.ai_chat_bot.ocr.top_k", Label: "Top K", Type: "int", Group: "AI 对话 · OCR", Default: 20},
	{Key: "plugin.ai_chat_bot.ocr.prompt", Label: "识图提示词", Type: "text", Group: "AI 对话 · OCR", Default: defaultOCRPrompt},

	// 定时与记忆
	{Key: "plugin.ai_chat_bot.clock.enable", Label: "启用 AI 定时任务", Type: "bool", Group: "AI 对话 · 定时与记忆", Default: true},
	{Key: "plugin.ai_chat_bot.clock.default_timeout_sec", Label: "默认超时(秒)", Type: "int", Group: "AI 对话 · 定时与记忆", Default: 120},
	{Key: "plugin.ai_chat_bot.clock.max_log_entries", Label: "日志保留条数", Type: "int", Group: "AI 对话 · 定时与记忆", Default: 500},
	{Key: "plugin.ai_chat_bot.memory.enable", Label: "启用长期记忆", Type: "bool", Group: "AI 对话 · 定时与记忆", Default: true},
	{Key: "plugin.ai_chat_bot.memory.max_entries", Label: "单会话记忆上限", Type: "int", Group: "AI 对话 · 定时与记忆", Default: 200},
}
