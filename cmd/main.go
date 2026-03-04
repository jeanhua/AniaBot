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

	"github.com/jeanhua/AniaBot/custom/plugins/acgwallpaper"
	"github.com/jeanhua/AniaBot/custom/plugins/activeman"
	"github.com/jeanhua/AniaBot/custom/plugins/douyinparser"
	"github.com/jeanhua/AniaBot/custom/plugins/gdmusicplugin"
	"github.com/jeanhua/AniaBot/custom/plugins/githubrepoer"
	"github.com/jeanhua/AniaBot/custom/plugins/groupnewsletter"
	"github.com/jeanhua/AniaBot/custom/plugins/interceptor"
	"github.com/jeanhua/AniaBot/custom/plugins/waifupics"
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

	// 自定义插件
	bot.AddPlugin(douyinparser.Newplugin())
	bot.AddPlugin(acgwallpaper.NewAcgWallpaperPlugin(5))
	bot.AddPlugin(waifupics.NewWaifuPlugin(5))
	bot.AddPlugin(githubrepoer.NewGithubRepoer(5))
	bot.AddPlugin(interceptor.NewInterceptorPlugin())
	bot.AddPlugin(groupnewsletter.NewGroupNewsletterPlugin())
	bot.AddPlugin(gdmusicplugin.NewMusicPlugin())
	bot.AddPlugin(activeman.NewActiveMan(0.1, 0.1, 0.1))

	bot.Run()
}
