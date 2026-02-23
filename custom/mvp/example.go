package mvp

import (
	"context"

	"github.com/jeanhua/AniaBot/bot/adapter/napcat"
	"github.com/jeanhua/AniaBot/bot/core"
	"github.com/jeanhua/AniaBot/common/bot"
	"github.com/jeanhua/AniaBot/common/model/command"
	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/jeanhua/AniaBot/common/msgchain"
	"github.com/jeanhua/AniaBot/common/plugin"
	// "github.com/jeanhua/AniaBot/bot/plugins/pluginlog"
	// "github.com/jeanhua/AniaBot/bot/plugins/pluginrepeat"
)

func main() {
	// adapter := napcat.NewNapcatHttpAdapter() // HTTP 适配器
	adapter := napcat.NewNapcatWebSocketAdapter() // Websocket适配器
	bot := core.NewAniaBot(adapter)
	// 插件注册
	// 系统内部插件
	// bot.AddPlugin(pluginlog.NewPlugin())    // 控制台日志打印插件
	// bot.AddPlugin(pluginrepeat.NewPlugin()) // 群复读机插件

	// 自定义插件
	bot.AddPlugin(NewCustomPlugin())

	bot.Run()
}

type CustomPlugin struct {
	plugin.Meta
}

func NewCustomPlugin() *CustomPlugin {
	return &CustomPlugin{
		Meta: plugin.Meta{
			Name:      "自定义插件",
			HelpWords: "这是一个自定义插件",
			AdminOnly: false,
			Order:     0,
		},
	}
}

// 接收群聊消息事件
func (p *CustomPlugin) OnGroupMsg(ctx context.Context, bot bot.Bot, cmd command.Command, msg message.Message) (bool, error) {
	if cmd.Mention && cmd.Name == "test" { // 判断条件：@[bot] /test 有效
		builder := msgchain.Builder().Group()          // 群消息构造器
		builder.Text("测试成功")                           // 构造消息
		bot.SendGroupMsg(msg.GroupId, builder.Build()) // 发送消息
	}
	return true, nil // 表示继续执行下一个插件
}
