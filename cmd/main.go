package main

import (
	"github.com/jeanhua/AniaBot/ania/adapter/napcat"
	"github.com/jeanhua/AniaBot/ania/aniabot"
	"github.com/jeanhua/AniaBot/ania/plugins/pluginlog"
	"github.com/jeanhua/AniaBot/ania/plugins/pluginrepeat"
)

func main() {
	// adapter := napcat.NewNapcatHttpAdapter()
	adapter := napcat.NewNapcatWebSocketAdapter()
	bot := aniabot.NewAniaBot(adapter)
	// 插件注册
	bot.AddPlugin(pluginlog.NewPlugin())
	bot.AddPlugin(pluginrepeat.NewPlugin())

	bot.Run()
}
