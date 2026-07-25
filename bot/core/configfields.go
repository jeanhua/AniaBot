package core

import "github.com/jeanhua/AniaBot/common/pluginconfig"

// frameworkConfigFields 框架自身的配置字段元信息（bot.*）。
// 插件的字段由插件各自通过 plugin.ConfigRegistrar 注册，面板动态展示。
var frameworkConfigFields = []pluginconfig.Field{
	// Bot 基础
	{Key: "bot.admin_id", Label: "管理员 QQ", Type: "int", Group: "Bot 基础", Help: "接收启动/异常通知的管理员 QQ 号"},

	// Web 面板
	{Key: "bot.admin_panel.enable", Label: "启用面板", Type: "bool", Group: "Web 面板", Help: "是否启用 Web 控制面板", Default: true},
	{Key: "bot.admin_panel.listen", Label: "监听地址", Type: "string", Group: "Web 面板", Help: "如 127.0.0.1:7700；改为 0.0.0.0:7700 可局域网访问（面板有密码保护）", Default: "127.0.0.1:7700"},

	// 适配器
	{Key: "bot.adapter.token", Label: "Token", Type: "password", Group: "NapCat 适配器", Sensitive: true, Help: "NapCat 侧设置了 token 时填写"},
	{Key: "bot.adapter.ws.address", Label: "WS 地址", Type: "string", Group: "NapCat 适配器", Help: "NapCat WebSocket Server 地址", Default: "ws://localhost:4455"},
	{Key: "bot.adapter.ws.worker_count", Label: "处理线程数", Type: "int", Group: "NapCat 适配器", Help: "0 为自动调整", Default: 0},
	{Key: "bot.adapter.ws.worker_queue_size", Label: "消息队列大小", Type: "int", Group: "NapCat 适配器", Help: "超出限制的消息将被丢弃", Default: 1024},
	{Key: "bot.adapter.http.listen_port", Label: "HTTP 监听端口", Type: "int", Group: "NapCat 适配器", Help: "HTTP 模式下 NapCat 上报事件的本地端口", Default: 6679},
	{Key: "bot.adapter.http.target_url", Label: "HTTP 目标地址", Type: "string", Group: "NapCat 适配器", Help: "HTTP 模式下 NapCat 开放的调用地址", Default: "http://localhost:6680"},

	// 缓存存储
	{Key: "bot.store.cache.driver", Label: "缓存驱动", Type: "select", Options: []string{"memory", "redis"}, Group: "缓存存储", Help: "memory（进程内内存，重启清空）或 redis（需 Redis 服务，支持多实例共享）", Default: "memory"},
	{Key: "bot.store.cache.redis.address", Label: "Redis 地址", Type: "string", Group: "缓存存储", Default: "localhost:6379"},
	{Key: "bot.store.cache.redis.password", Label: "Redis 密码", Type: "password", Group: "缓存存储", Sensitive: true},
	{Key: "bot.store.cache.redis.db", Label: "Redis DB", Type: "int", Group: "缓存存储", Default: 0},
}
