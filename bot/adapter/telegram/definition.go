package telegram

import (
	"github.com/jeanhua/AniaBot/common/adapter"
	"github.com/jeanhua/AniaBot/common/pluginconfig"
	"github.com/spf13/viper"
)

// idPrefix Telegram 平台 ID 的框架统一前缀：所有 Telegram ID（chat_id/用户 ID/消息 ID）
// 在框架内表示为 "tg:" 前缀的字符串，core 按前缀路由到本适配器。
// 消息 ID 因 Telegram 的 message_id 仅在同一会话内唯一，统一编码为 "tg:<chat_id>:<message_id>"。
const idPrefix = "tg:"

// Platform 平台标识。
const Platform = "telegram"

// telegramConfigFields Telegram 平台配置字段（面板动态渲染）。
// 默认长轮询模式：无需公网地址，与飞书 ws 模式体验一致。
var telegramConfigFields = []pluginconfig.Field{
	{Key: "bot.platform.telegram.enable", Label: "启用 Telegram 平台", Type: "bool", Group: "平台适配器", Help: "是否启用 Telegram 平台；关闭后 Bot 不连接 Telegram", Default: false},
	{Key: "bot.telegram.token", Label: "Bot Token", Type: "password", Group: "Telegram 适配器", Sensitive: true, Help: "向 @BotFather 创建机器人后获取的 Bot Token"},
	{Key: "bot.telegram.api_base", Label: "API Base URL", Type: "string", Group: "Telegram 适配器", Help: "官方为 https://api.telegram.org；国内部署可填自建 Bot API 网关/反代地址", Default: "https://api.telegram.org"},
	{Key: "bot.telegram.proxy", Label: "HTTP/SOCKS5 代理", Type: "string", Group: "Telegram 适配器", Help: "格式 http://host:port 或 socks5://host:port；留空直连"},
	{Key: "bot.telegram.polling.timeout", Label: "长轮询超时（秒）", Type: "int", Group: "Telegram 适配器", Help: "getUpdates 长轮询等待秒数，默认 30（建议 10-50）", Default: 30},
	{Key: "bot.telegram.parse_mode", Label: "消息渲染模式", Type: "select", Options: []string{"off", "markdown", "markdownv2"}, Group: "Telegram 适配器", Help: "off=纯文本（默认，最稳定）；markdown=旧版 Markdown（仅需转义 _ * [ ]，词中下划线不解析，对 AI 输出最宽容）；markdownv2=新版（转义严格，需转义 _ * [ ] ( ) ~ ` > # + - = | { } . !）；流式结束/发送时渲染，解析失败自动降级纯文本", Default: "off"},
}

// init 注册 Telegram 适配器定义。
func init() {
	adapter.Register(adapter.Definition{
		Name:         "telegram",
		Platform:     Platform,
		IDPrefix:     idPrefix,
		ConfigFields: telegramConfigFields,
		New: func(cfg *viper.Viper) (adapter.Adapter, error) {
			return NewAdapter(cfg), nil
		},
	})
}
