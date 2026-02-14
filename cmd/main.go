package main

import (
	"github.com/jeanhua/AniaBot/ania/adapter/napcat"
	"github.com/jeanhua/AniaBot/ania/aniabot"
	"github.com/jeanhua/AniaBot/ania/plugins/pluginaichat"
	"github.com/jeanhua/AniaBot/ania/plugins/pluginantiwithdrawal"
	"github.com/jeanhua/AniaBot/ania/plugins/pluginlog"
	"github.com/jeanhua/AniaBot/ania/plugins/pluginnews"
	"github.com/jeanhua/AniaBot/ania/plugins/pluginrepeat"
	"github.com/jeanhua/AniaBot/custom/plugins/acgwallpaper"
	"github.com/jeanhua/AniaBot/custom/plugins/douyinparser"
	"github.com/jeanhua/AniaBot/custom/plugins/waifupics"
)

func main() {
	// adapter := napcat.NewNapcatHttpAdapter()
	adapter := napcat.NewNapcatWebSocketAdapter()
	bot := aniabot.NewAniaBot(adapter)
	// 插件注册
	bot.AddPlugin(pluginlog.NewPlugin())
	bot.AddPlugin(pluginrepeat.NewPlugin())
	bot.AddPlugin(pluginantiwithdrawal.NewPlugin())
	bot.AddPlugin(pluginaichat.NewAIChatPlugin())
	bot.AddPlugin(pluginnews.NewNewsPlugin())

	// 自定义插件
	bot.AddPlugin(douyinparser.Newplugin())
	bot.AddPlugin(acgwallpaper.NewAcgWallpaperPlugin(5))
	bot.AddPlugin(waifupics.NewWaifuPlugin(5))

	bot.Run()
}
