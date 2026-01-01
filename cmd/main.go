package main

import (
	"github.com/jeanhua/AniaBot/ania/aniaadapter"
	"github.com/jeanhua/AniaBot/ania/aniabot"
	"github.com/jeanhua/AniaBot/ania/plugins/pluginlog"
	"github.com/jeanhua/AniaBot/ania/plugins/pluginrepeat"
)

func main() {
	// adapter := aniaadapter.NewNapcatHttpAdapter()
	adapter := aniaadapter.NewNapcatWebSocketAdapter()
	bot := aniabot.NewAniaBot(adapter)
	// 插件注册
	bot.AddPlugin(pluginlog.NewPlugin())
	bot.AddPlugin(pluginrepeat.NewPlugin())

	bot.Run()
}
