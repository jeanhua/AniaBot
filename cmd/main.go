package main

import (
	"github.com/jeanhua/AniaBot/ania/aniaadapter"
	"github.com/jeanhua/AniaBot/ania/aniabot"
	"github.com/jeanhua/AniaBot/ania/plugins/logplugin"
)

func main() {
	adapter := aniaadapter.NewNapcatHttpAdapter()
	bot := aniabot.NewAniaBot(adapter)
	bot.AddPlugin(logplugin.NewPlugin())
	bot.Run()
}
