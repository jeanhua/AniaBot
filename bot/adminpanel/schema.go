package adminpanel

// ConfigField 描述一个配置键在面板表单中的展示方式。
type ConfigField struct {
	Key       string `json:"key"`
	Label     string `json:"label"`
	Type      string `json:"type"` // string | password | int | float | bool | text | strings | ints
	Group     string `json:"group"`
	Help      string `json:"help,omitempty"`
	Sensitive bool   `json:"sensitive,omitempty"`
}

// configSchema 已知配置键的表单元数据。未列出的键仍可在「高级模式」
// （原始 JSON 编辑）中查看与修改。
var configSchema = []ConfigField{
	// Bot 基础
	{Key: "bot.admin_id", Label: "管理员 QQ", Type: "int", Group: "Bot 基础", Help: "接收启动/异常通知的管理员 QQ 号"},

	// Web 面板
	{Key: "bot.admin_panel.enable", Label: "启用面板", Type: "bool", Group: "Web 面板", Help: "是否启用 Web 控制面板"},
	{Key: "bot.admin_panel.listen", Label: "监听地址", Type: "string", Group: "Web 面板", Help: "如 127.0.0.1:7700；改为 0.0.0.0:7700 可局域网访问（面板有密码保护）"},

	// 适配器
	{Key: "bot.adapter.token", Label: "Token", Type: "password", Group: "NapCat 适配器", Sensitive: true, Help: "NapCat 侧设置了 token 时填写"},
	{Key: "bot.adapter.ws.address", Label: "WS 地址", Type: "string", Group: "NapCat 适配器", Help: "NapCat WebSocket Server 地址"},
	{Key: "bot.adapter.ws.max_retries", Label: "最大重连次数", Type: "int", Group: "NapCat 适配器"},
	{Key: "bot.adapter.ws.worker_count", Label: "处理线程数", Type: "int", Group: "NapCat 适配器", Help: "0 为自动调整"},
	{Key: "bot.adapter.ws.worker_queue_size", Label: "消息队列大小", Type: "int", Group: "NapCat 适配器", Help: "超出限制的消息将被丢弃"},
	{Key: "bot.adapter.http.listen_port", Label: "HTTP 监听端口", Type: "int", Group: "NapCat 适配器", Help: "HTTP 模式下 NapCat 上报事件的本地端口"},
	{Key: "bot.adapter.http.target_url", Label: "HTTP 目标地址", Type: "string", Group: "NapCat 适配器", Help: "HTTP 模式下 NapCat 开放的调用地址"},

	// 缓存存储
	{Key: "bot.store.cache.driver", Label: "缓存驱动", Type: "string", Group: "缓存存储", Help: "memory（进程内内存，重启清空）或 redis（需 Redis 服务，支持多实例共享）"},
	{Key: "bot.store.cache.redis.address", Label: "Redis 地址", Type: "string", Group: "缓存存储"},
	{Key: "bot.store.cache.redis.password", Label: "Redis 密码", Type: "password", Group: "缓存存储", Sensitive: true},
	{Key: "bot.store.cache.redis.db", Label: "Redis DB", Type: "int", Group: "缓存存储"},

	// AI 对话 - 模型
	{Key: "plugin.ai_chat_bot.base_url", Label: "Base URL", Type: "string", Group: "AI 对话 · 模型", Help: "兼容 OpenAI 规范的 API 地址"},
	{Key: "plugin.ai_chat_bot.api_key", Label: "API Key", Type: "password", Group: "AI 对话 · 模型", Sensitive: true},
	{Key: "plugin.ai_chat_bot.model", Label: "模型", Type: "string", Group: "AI 对话 · 模型"},
	{Key: "plugin.ai_chat_bot.multimodal", Label: "多模态", Type: "bool", Group: "AI 对话 · 模型", Help: "主模型是否支持图片输入"},
	{Key: "plugin.ai_chat_bot.rate_limit", Label: "速率限制(次/秒)", Type: "int", Group: "AI 对话 · 模型"},
	{Key: "plugin.ai_chat_bot.max_context_tokens", Label: "上下文 Token 上限", Type: "int", Group: "AI 对话 · 模型"},
	{Key: "plugin.ai_chat_bot.max_token", Label: "最大输出 Token", Type: "int", Group: "AI 对话 · 模型"},
	{Key: "plugin.ai_chat_bot.temperature", Label: "Temperature", Type: "float", Group: "AI 对话 · 模型"},
	{Key: "plugin.ai_chat_bot.top_p", Label: "Top P", Type: "float", Group: "AI 对话 · 模型"},
	{Key: "plugin.ai_chat_bot.top_k", Label: "Top K", Type: "int", Group: "AI 对话 · 模型"},
	{Key: "plugin.ai_chat_bot.thinking.enable", Label: "启用深度思考", Type: "bool", Group: "AI 对话 · 模型"},
	{Key: "plugin.ai_chat_bot.thinking.mode", Label: "思考模式", Type: "string", Group: "AI 对话 · 模型", Help: "none / low / medium / high / auto"},
	{Key: "plugin.ai_chat_bot.prompt", Label: "系统提示词", Type: "text", Group: "AI 对话 · 模型"},
	{Key: "plugin.ai_chat_bot.search.token", Label: "Jina AI Token", Type: "password", Group: "AI 对话 · 模型", Sensitive: true, Help: "网页浏览与搜索功能"},

	// AI 对话 - 工具
	{Key: "plugin.ai_chat_bot.skills_dir", Label: "Skills 目录", Type: "string", Group: "AI 对话 · 工具"},
	{Key: "plugin.ai_chat_bot.skills", Label: "Skills 白名单", Type: "strings", Group: "AI 对话 · 工具", Help: "为空则加载全部，每行一个"},
	{Key: "plugin.ai_chat_bot.bash.enable", Label: "启用 Bash 工具", Type: "bool", Group: "AI 对话 · 工具", Help: "直接在宿主机执行 bash 命令，注意安全风险"},
	{Key: "plugin.ai_chat_bot.bash.shell", Label: "Shell 路径", Type: "string", Group: "AI 对话 · 工具"},
	{Key: "plugin.ai_chat_bot.bash.env", Label: "环境变量", Type: "strings", Group: "AI 对话 · 工具", Help: "KEY=VALUE，每行一个"},
	{Key: "plugin.ai_chat_bot.bash.whitelist", Label: "命令白名单(正则)", Type: "strings", Group: "AI 对话 · 工具", Help: "非空时仅允许匹配的命令，每行一个"},
	{Key: "plugin.ai_chat_bot.bash.blacklist", Label: "命令黑名单(正则)", Type: "strings", Group: "AI 对话 · 工具", Help: "匹配的命令被禁止，每行一个"},
	{Key: "plugin.ai_chat_bot.local_image.enable", Label: "启用本地图片工具", Type: "bool", Group: "AI 对话 · 工具", Help: "可读取宿主机本地图片，默认关闭"},

	// AI 对话 - OCR
	{Key: "plugin.ai_chat_bot.ocr.enable", Label: "启用备用识图模型", Type: "bool", Group: "AI 对话 · OCR", Help: "主模型不支持多模态时将图片转文字"},
	{Key: "plugin.ai_chat_bot.ocr.base_url", Label: "Base URL", Type: "string", Group: "AI 对话 · OCR"},
	{Key: "plugin.ai_chat_bot.ocr.api_key", Label: "API Key", Type: "password", Group: "AI 对话 · OCR", Sensitive: true},
	{Key: "plugin.ai_chat_bot.ocr.model", Label: "模型", Type: "string", Group: "AI 对话 · OCR"},
	{Key: "plugin.ai_chat_bot.ocr.max_token", Label: "最大输出 Token", Type: "int", Group: "AI 对话 · OCR"},
	{Key: "plugin.ai_chat_bot.ocr.temperature", Label: "Temperature", Type: "float", Group: "AI 对话 · OCR"},
	{Key: "plugin.ai_chat_bot.ocr.top_p", Label: "Top P", Type: "float", Group: "AI 对话 · OCR"},
	{Key: "plugin.ai_chat_bot.ocr.top_k", Label: "Top K", Type: "int", Group: "AI 对话 · OCR"},
	{Key: "plugin.ai_chat_bot.ocr.prompt", Label: "识图提示词", Type: "text", Group: "AI 对话 · OCR"},

	// AI 对话 - 定时与记忆
	{Key: "plugin.ai_chat_bot.clock.enable", Label: "启用 AI 定时任务", Type: "bool", Group: "AI 对话 · 定时与记忆"},
	{Key: "plugin.ai_chat_bot.clock.default_timeout_sec", Label: "默认超时(秒)", Type: "int", Group: "AI 对话 · 定时与记忆"},
	{Key: "plugin.ai_chat_bot.clock.max_log_entries", Label: "日志保留条数", Type: "int", Group: "AI 对话 · 定时与记忆"},
	{Key: "plugin.ai_chat_bot.memory.enable", Label: "启用长期记忆", Type: "bool", Group: "AI 对话 · 定时与记忆"},
	{Key: "plugin.ai_chat_bot.memory.max_entries", Label: "单会话记忆上限", Type: "int", Group: "AI 对话 · 定时与记忆"},

	// 每日新闻
	{Key: "plugin.dailyNews.api", Label: "API 端点", Type: "string", Group: "每日新闻插件"},
	{Key: "plugin.dailyNews.cron", Label: "Cron 表达式", Type: "string", Group: "每日新闻插件", Help: "如 0 18 * * * 表示每天 18 点"},
	{Key: "plugin.dailyNews.groups", Label: "播报群列表", Type: "ints", Group: "每日新闻插件", Help: "每行一个群号"},

	// 拦截器
	{Key: "plugin.interceptor.blacklist.users", Label: "用户黑名单", Type: "strings", Group: "拦截器插件", Help: "优先级: 黑名单 > 白名单，每行一个"},
	{Key: "plugin.interceptor.blacklist.groups", Label: "群黑名单", Type: "strings", Group: "拦截器插件"},
	{Key: "plugin.interceptor.whitelist.users", Label: "用户白名单", Type: "strings", Group: "拦截器插件", Help: "all 表示全部放行"},
	{Key: "plugin.interceptor.whitelist.groups", Label: "群白名单", Type: "strings", Group: "拦截器插件"},

	// 群刊
	{Key: "plugin.group_newsletter.fmt", Label: "输出格式", Type: "string", Group: "群刊插件", Help: "md 或 jpg（jpg 需 md2img-api 容器）"},
	{Key: "plugin.group_newsletter.model.base_url", Label: "Base URL", Type: "string", Group: "群刊插件"},
	{Key: "plugin.group_newsletter.model.api_key", Label: "API Key", Type: "password", Group: "群刊插件", Sensitive: true},
	{Key: "plugin.group_newsletter.model.model", Label: "模型", Type: "string", Group: "群刊插件"},
	{Key: "plugin.group_newsletter.model.prompt", Label: "提示词", Type: "text", Group: "群刊插件", Help: "留空使用默认"},
	{Key: "plugin.group_newsletter.msg_threshold", Label: "消息阈值", Type: "int", Group: "群刊插件"},
	{Key: "plugin.group_newsletter.max_messages", Label: "最大保存消息数", Type: "int", Group: "群刊插件"},
	{Key: "plugin.group_newsletter.enabled_groups", Label: "启用群列表", Type: "ints", Group: "群刊插件", Help: "留空表示所有群启用"},

	// GithubRepoer
	{Key: "plugin.github_repoer.max_token", Label: "上下文限制", Type: "int", Group: "GithubRepoer 插件"},
	{Key: "plugin.github_repoer.fmt", Label: "输出格式", Type: "string", Group: "GithubRepoer 插件", Help: "md 或 jpg"},
	{Key: "plugin.github_repoer.model.base_url", Label: "Base URL", Type: "string", Group: "GithubRepoer 插件"},
	{Key: "plugin.github_repoer.model.api_key", Label: "API Key", Type: "password", Group: "GithubRepoer 插件", Sensitive: true},
	{Key: "plugin.github_repoer.model.model", Label: "模型", Type: "string", Group: "GithubRepoer 插件"},
	{Key: "plugin.github_repoer.model.prompt", Label: "提示词", Type: "text", Group: "GithubRepoer 插件"},

	// gdmusic
	{Key: "plugin.gdmusic.base_url", Label: "API 地址", Type: "string", Group: "gdmusic 插件", Help: "留空使用默认"},

	// url_parser
	{Key: "plugin.url_parser.cache_ttl", Label: "缓存时间(分钟)", Type: "int", Group: "URL 解析插件"},
	{Key: "plugin.url_parser.token", Label: "Jina AI Token", Type: "password", Group: "URL 解析插件", Sensitive: true},
	{Key: "plugin.url_parser.llm.base_url", Label: "Base URL", Type: "string", Group: "URL 解析插件"},
	{Key: "plugin.url_parser.llm.api_key", Label: "API Key", Type: "password", Group: "URL 解析插件", Sensitive: true},
	{Key: "plugin.url_parser.llm.model", Label: "模型", Type: "string", Group: "URL 解析插件"},

	// 组件
	{Key: "component.md2img.apipoint", Label: "md2img 地址", Type: "string", Group: "组件", Help: "md2img-api 容器地址"},
}
