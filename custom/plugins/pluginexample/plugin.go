package pluginexample

import "github.com/jeanhua/AniaBot/common/plugin"

type ExamplePlugin struct {
	plugin.Meta
}

func NewPlugin() *ExamplePlugin {
	return &ExamplePlugin{
		Meta: plugin.Meta{
			Name:      "示例插件",     // 这是插件名字
			HelpWords: "这是一个示例插件", // 这是插件描述，发送 /help 显示的内容
			AdminOnly: false,      // 为真则只有管理员发送 /help 才显示
			Order:     0,          // 插件执行顺序，从小到大
		},
	}
}
