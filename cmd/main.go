package main

import (
	"flag"
	"fmt"
	"os"

	// 空白导入各平台适配器包以触发其 init() 注册（新增平台在此追加导入即可）
	_ "github.com/jeanhua/AniaBot/bot/adapter/discord"
	_ "github.com/jeanhua/AniaBot/bot/adapter/feishu"
	_ "github.com/jeanhua/AniaBot/bot/adapter/napcat"
	_ "github.com/jeanhua/AniaBot/bot/adapter/qqofficial"
	_ "github.com/jeanhua/AniaBot/bot/adapter/telegram"
	"github.com/jeanhua/AniaBot/bot/core"
	"github.com/jeanhua/AniaBot/bot/plugins/pluginaichat"
	"github.com/jeanhua/AniaBot/bot/plugins/plugininterceptor"
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
	// 平台适配器由注册表按配置 bot.platform.<name>.enable 在配置加载后创建
	bot := core.NewAniaBot()
	// 插件注册
	bot.AddPlugin(pluginsys.NewPluginSys())
	bot.AddPlugin(pluginlog.NewPlugin())
	// 白名单管理（/wl）不再是内置插件：需要命令行管理名单时从插件市场安装 whitelist，
	// 其 Meta.Order 在日志之后、其余功能插件之前，保证「拦住全部插件」时最先拦截
	bot.AddPlugin(plugininterceptor.NewPlugin())

	bot.AddPlugin(pluginrepeat.NewPlugin())
	bot.AddPlugin(pluginnews.NewNewsPlugin())

	bot.AddPlugin(pluginaichat.NewAIChatPlugin())

	// 插件市场安装的第三方插件（由 tools/plugingen 生成注册代码）
	registerMarketplacePlugins(bot)

	bot.Run()
}
