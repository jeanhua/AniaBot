package core

import (
	"github.com/jeanhua/AniaBot/bot/adminpanel"
	"github.com/jeanhua/AniaBot/common/pluginconfig"
)

// frameworkConfigFields 框架自身的配置字段元信息（bot.*）。
// 插件的字段由插件各自通过 plugin.ConfigRegistrar 注册，面板动态展示。
var frameworkConfigFields = []pluginconfig.Field{
	// Bot 基础
	{Key: "bot.admin_id", Label: "管理员 QQ", Type: "int", Group: "Bot 基础", Help: "接收启动/异常通知的管理员 QQ 号"},

	// Web 面板
	{Key: "bot.admin_panel.enable", Label: "启用面板", Type: "bool", Group: "Web 面板", Help: "是否启用 Web 控制面板；关闭后可用环境变量 ANIA_BOT_ADMIN_PANEL_ENABLE=true 覆盖重新开启", Default: true},
	{Key: "bot.admin_panel.listen", Label: "监听地址", Type: "string", Group: "Web 面板", Help: "如 127.0.0.1:7700；改为 0.0.0.0:7700 可局域网访问（面板有密码保护）", Default: "127.0.0.1:7700"},

	// 适配器
	{Key: "bot.adapter.mode", Label: "连接模式", Type: "select", Options: []string{"ws", "http"}, Group: "NapCat 适配器", Help: "ws（WebSocket，推荐）或 http（需同时配置下方「本地监听端口」和「NapCat HTTP 地址」），重启后生效", Default: "ws"},
	{Key: "bot.adapter.token", Label: "Token", Type: "password", Group: "NapCat 适配器", Sensitive: true, Help: "NapCat 侧设置了 token 时填写"},
	{Key: "bot.adapter.ws.address", Label: "WS 地址", Type: "string", Group: "NapCat 适配器", Help: "NapCat WebSocket Server 地址", Default: "ws://localhost:4455"},
	{Key: "bot.adapter.ws.worker_count", Label: "处理线程数", Type: "int", Group: "NapCat 适配器", Help: "0 为自动调整", Default: 0},
	{Key: "bot.adapter.ws.worker_queue_size", Label: "消息队列大小", Type: "int", Group: "NapCat 适配器", Help: "超出限制的消息将被丢弃", Default: 1024},
	{Key: "bot.adapter.http.listen_port", Label: "本地监听端口", Type: "int", Group: "NapCat 适配器", Help: "HTTP 模式下 Bot 接收 NapCat 事件上报的本地端口；NapCat 侧需添加「HTTP 客户端」，上报地址填 http://<Bot 主机 IP>:<此端口>", Default: 6679},
	{Key: "bot.adapter.http.target_url", Label: "NapCat HTTP 地址", Type: "string", Group: "NapCat 适配器", Help: "HTTP 模式下 NapCat 的「HTTP 服务器」地址，Bot 通过它调用 NapCat 接口（发消息等）", Default: "http://localhost:6680"},

	// 缓存存储
	{Key: "bot.store.cache.driver", Label: "缓存驱动", Type: "select", Options: []string{"memory", "redis"}, Group: "缓存存储", Help: "memory（进程内内存，重启清空）或 redis（需 Redis 服务，支持多实例共享）", Default: "memory"},
	{Key: "bot.store.cache.redis.address", Label: "Redis 地址", Type: "string", Group: "缓存存储", Default: "localhost:6379"},
	{Key: "bot.store.cache.redis.password", Label: "Redis 密码", Type: "password", Group: "缓存存储", Sensitive: true},
	{Key: "bot.store.cache.redis.db", Label: "Redis DB", Type: "int", Group: "缓存存储", Default: 0},

	// 自动更新
	{Key: "bot.update.source_dir", Label: "源码目录", Type: "string", Group: "自动更新", Help: "AniaBot 仓库的克隆路径（独立目录），自动更新在此拉取并编译；留空则禁用自动更新"},
	{Key: "bot.update.git_url", Label: "Git 地址", Type: "string", Group: "自动更新", Help: "非空时更新前覆盖源码目录的 origin 地址"},
	{Key: "bot.update.branch", Label: "跟踪分支", Type: "string", Group: "自动更新", Default: "main"},

	// API 余额查询（面板概览页展示，修改后即时生效，无需重启）
	{Key: "bot.balance.enable", Label: "启用余额查询", Type: "bool", Group: "API 余额查询", Help: "在面板概览页显示 AI API 余额；通过下方自定义 JS 代码请求余额接口", Default: false},
	{Key: "bot.balance.cache_sec", Label: "缓存时间(秒)", Type: "int", Group: "API 余额查询", Help: "余额查询结果的缓存时长，避免频繁请求余额接口", Default: 300},
	{Key: "bot.balance.js", Label: "查询脚本(JS)", Type: "text", Group: "API 余额查询", Help: "自定义 JS 代码，由后端执行。可用全局变量：cfg.base_url / cfg.api_key / cfg.model（AI 对话插件的 API 配置）、fetch(url, {method, headers, body})（同步 HTTP 请求，返回 {status, body, json()}）、console.log。最后一条表达式的值作为余额显示", Default: adminpanel.DefaultBalanceJS},
}
