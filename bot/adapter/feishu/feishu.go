package feishu

import (
	"github.com/jeanhua/AniaBot/common/adapter"
	"github.com/jeanhua/AniaBot/common/pluginconfig"
	"github.com/spf13/viper"
)

// idPrefix 飞书平台 ID 的框架统一前缀：所有飞书 ID（chat_id/消息 ID/用户 open_id）
// 在框架内表示为 "fs:" 前缀的字符串，core 按前缀路由到本适配器。
const idPrefix = "fs:"

// Platform 平台标识。
const Platform = "feishu"

// feishuConfigFields 飞书平台配置字段（面板动态渲染）。
// 默认 WebSocket 长连接模式：无需公网地址，与 QQ 的 ws 体验一致。
var feishuConfigFields = []pluginconfig.Field{
	{Key: "bot.platform.feishu.enable", Label: "启用飞书平台", Type: "bool", Group: "平台适配器", Help: "是否启用飞书平台；关闭后 Bot 不连接飞书", Default: false},
	{Key: "bot.feishu.app_id", Label: "App ID", Type: "string", Group: "飞书适配器", Help: "飞书开放平台「凭证与基础信息」中的应用 App ID"},
	{Key: "bot.feishu.app_secret", Label: "App Secret", Type: "password", Group: "飞书适配器", Sensitive: true, Help: "飞书开放平台「凭证与基础信息」中的应用 App Secret"},
	{Key: "bot.feishu.mode", Label: "事件订阅方式", Type: "select", Options: []string{"ws", "webhook"}, Group: "飞书适配器", Help: "ws（WebSocket 长连接，推荐，无需公网地址）或 webhook（需公网可访问的 HTTPS 回调地址）", Default: "ws"},
	{Key: "bot.feishu.webhook.listen", Label: "Webhook 监听地址", Type: "string", Group: "飞书适配器", Help: "webhook 模式下的本地监听地址", Default: "127.0.0.1:7777"},
	{Key: "bot.feishu.webhook.path", Label: "Webhook 回调路径", Type: "string", Group: "飞书适配器", Help: "webhook 模式下的回调路径，飞书后台请求地址需填 https://<公网地址><此路径>", Default: "/webhook/event"},
	{Key: "bot.feishu.webhook.verification_token", Label: "Verification Token", Type: "string", Group: "飞书适配器", Help: "webhook 模式下在飞书「事件订阅」页配置；ws 模式无需填写"},
	{Key: "bot.feishu.webhook.encrypt_key", Label: "Encrypt Key", Type: "password", Group: "飞书适配器", Sensitive: true, Help: "webhook 模式下可选的事件加密密钥；ws 模式无需填写"},
}

// init 注册飞书适配器定义。
func init() {
	adapter.Register(adapter.Definition{
		Name:         "feishu",
		Platform:     Platform,
		IDPrefix:     idPrefix,
		ConfigFields: feishuConfigFields,
		New: func(cfg *viper.Viper) (adapter.Adapter, error) {
			return NewAdapter(cfg), nil
		},
	})
}
