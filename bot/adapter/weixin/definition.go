package weixin

import (
	"github.com/jeanhua/AniaBot/common/adapter"
	"github.com/jeanhua/AniaBot/common/pluginconfig"
	"github.com/spf13/viper"
)

// weixinConfigFields 微信平台配置字段（面板动态渲染）。
// bot token 由扫码登录获得并保存在状态目录（./data/weixin/），不在面板填写；
// token 失效（errcode -14）时适配器会在控制台重新展示登录二维码。
var weixinConfigFields = []pluginconfig.Field{
	{Key: "bot.platform.weixin.enable", Label: "启用微信平台", Type: "bool", Group: "平台适配器", Help: "是否启用微信（iLink bot）平台；首次启动时会在控制台展示扫码登录二维码", Default: false},
	{Key: "bot.weixin.token", Label: "Bot Token", Type: "password", Group: "微信适配器", Sensitive: true, Help: "扫码登录凭据由控制台登录后自动保存在状态目录，一般无需填写；手动置入则跳过扫码（优先于状态文件）"},
	{Key: "bot.weixin.api_base", Label: "API Base URL", Type: "string", Group: "微信适配器", Help: "iLink Bot API 地址，默认官方地址；登录成功后会自动记录服务端下发的专属地址", Default: DefaultAPIBase},
	{Key: "bot.weixin.cdn_base", Label: "CDN Base URL", Type: "string", Group: "微信适配器", Help: "微信媒体 CDN 地址（图片/文件上传下载）", Default: DefaultCDNBase},
	{Key: "bot.weixin.state_dir", Label: "状态目录", Type: "string", Group: "微信适配器", Help: "登录凭据与长轮询游标的保存目录（建议放在 data 卷，容器重建后不丢登录）", Default: "./data/weixin"},
	{Key: "bot.weixin.bot_type", Label: "Bot 类型", Type: "string", Group: "微信适配器", Help: "扫码登录的 ilink bot 类型，默认 3（官方微信 bot）", Default: DefaultBotType},
}

// init 注册微信适配器定义。
func init() {
	adapter.Register(adapter.Definition{
		Name:         "weixin",
		Platform:     Platform,
		IDPrefix:     idPrefix,
		ConfigFields: weixinConfigFields,
		New: func(cfg *viper.Viper) (adapter.Adapter, error) {
			return NewAdapter(cfg), nil
		},
	})
}
