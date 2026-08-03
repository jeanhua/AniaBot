package discord

import (
	"github.com/jeanhua/AniaBot/common/adapter"
	"github.com/jeanhua/AniaBot/common/pluginconfig"
	"github.com/spf13/viper"
)

// idPrefix Discord 平台 ID 的框架统一前缀：所有 Discord ID（用户/频道/消息）
// 在框架内表示为 "dc:" 前缀的字符串，core 按前缀路由到本适配器。
// 消息 ID 因 GetMsgDetail/历史查询需要频道上下文，统一编码为 "dc:<channel_id>:<message_id>"。
const idPrefix = "dc:"

// Platform 平台标识。
const Platform = "discord"

// discordConfigFields Discord 平台配置字段（面板动态渲染）。
// discordgo Gateway WebSocket 收事件：无需公网地址，与 Telegram 长轮询体验一致。
var discordConfigFields = []pluginconfig.Field{
	{Key: "bot.platform.discord.enable", Label: "启用 Discord 平台", Type: "bool", Group: "平台适配器", Help: "是否启用 Discord 平台；关闭后 Bot 不连接 Discord", Default: false},
	{Key: "bot.discord.token", Label: "Bot Token", Type: "password", Group: "Discord 适配器", Sensitive: true, Help: "Discord Developer Portal → Applications → Bot 页面的 Token（重置后旧 Token 立即失效）；必须在该页面开启 Message Content Intent，否则网关拒绝连接"},
	{Key: "bot.discord.proxy", Label: "HTTP/SOCKS5 代理", Type: "string", Group: "Discord 适配器", Help: "格式 http://host:port 或 socks5://host:port；留空直连（REST 与 WebSocket 网关都走代理）"},
	{Key: "bot.discord.member_events", Label: "接收成员进出事件", Type: "bool", Group: "Discord 适配器", Help: "开启后订阅 Server Members 特权意图（需在 Developer Portal 同步开启 Server Members Intent），成员进出以平台事件（discord.guild_member_add/remove）投递", Default: false},
}

// init 注册 Discord 适配器定义。
func init() {
	adapter.Register(adapter.Definition{
		Name:         "discord",
		Platform:     Platform,
		IDPrefix:     idPrefix,
		ConfigFields: discordConfigFields,
		New: func(cfg *viper.Viper) (adapter.Adapter, error) {
			return NewAdapter(cfg), nil
		},
	})
}
