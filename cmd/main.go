package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/jeanhua/AniaBot/bot/adapter/napcat"
	"github.com/jeanhua/AniaBot/bot/core"
	"github.com/jeanhua/AniaBot/bot/plugins/pluginaichat"
	"github.com/jeanhua/AniaBot/bot/plugins/pluginantiwithdrawal"
	"github.com/jeanhua/AniaBot/bot/plugins/pluginlog"
	"github.com/jeanhua/AniaBot/bot/plugins/pluginnews"
	"github.com/jeanhua/AniaBot/bot/plugins/pluginrepeat"
	"github.com/jeanhua/AniaBot/bot/plugins/pluginsys"
)

var setPassword = flag.String("set-password", "", "重置 Web 控制面板密码后退出（忘记密码时使用），如：-set-password 新密码")

func main() {
	flag.Parse()

	// 忘记面板密码时：仅重置密码并退出，不启动 Bot
	if *setPassword != "" {
		if err := core.ResetPanelPassword(*setPassword); err != nil {
			fmt.Println("重置面板密码失败:", err)
			os.Exit(1)
		}
		fmt.Println("Web 控制面板密码已重置，请重新启动 Bot")
		return
	}
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
