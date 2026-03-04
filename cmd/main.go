package main

import (
	"github.com/jeanhua/AniaBot/bot/adapter/napcat"
	"github.com/jeanhua/AniaBot/bot/core"
	"github.com/jeanhua/AniaBot/bot/plugins/pluginaichat"
	"github.com/jeanhua/AniaBot/bot/plugins/pluginantiwithdrawal"
	"github.com/jeanhua/AniaBot/bot/plugins/pluginlog"
	"github.com/jeanhua/AniaBot/bot/plugins/pluginnews"
	"github.com/jeanhua/AniaBot/bot/plugins/pluginrepeat"
	"github.com/jeanhua/AniaBot/bot/plugins/pluginsys"
)

func main() {
	// adapter := napcat.NewNapcatHttpAdapter()
	adapter := napcat.NewNapcatWebSocketAdapter()
	bot := core.NewAniaBot(adapter)
	// 插件注册
	bot.AddPlugin(pluginsys.NewPluginSys())
	bot.AddPlugin(pluginlog.NewPlugin())
	bot.AddPlugin(pluginrepeat.NewPlugin())
	bot.AddPlugin(pluginantiwithdrawal.NewPlugin())
	bot.AddPlugin(pluginaichat.NewAIChatPlugin())
	bot.AddPlugin(pluginnews.NewNewsPlugin())

	bot.Run()
}
