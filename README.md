<div align="center">
  <img src="./README/logo.jpg" width="200" alt="AniaBot Logo"/>
  <h1>AniaBot</h1>
  <p>一个 QQ 机器人框架喵~</p>
</div>

> 正在重构 [PinBot](https://github.com/jeanhua/PinBot) 中，有这些优势(对比PinBot)：
>
> - 更加灵活，可适配多种上层协议(目前已实现napcat http/websocket协议)
> - 性能提升(减少重复反序列化消耗，优化结构)
> - 结构更加清晰，支持插件动态实现接口自动触发对应事件

## todo

1. RAG支持
2. 完善内置插件系统
3. 完善AI交互模板框架

## 插件系统

### 一、插件指南

在AniaBot - custom - plugins目录下创建一个文件夹，编写go插件

实现插件：

```go
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
```

**插件注册**

```go
func main() {
	// adapter := napcat.NewNapcatHttpAdapter()
	adapter := napcat.NewNapcatWebSocketAdapter()
	bot := aniabot.NewAniaBot(adapter)
	// 插件注册
	bot.AddPlugin(pluginlog.NewPlugin())
	bot.AddPlugin(pluginrepeat.NewPlugin())

	bot.Run()
}
```

### 二、插件定义

**插件元数据**：

```go
type Meta struct {
	Name      string // 插件名字
	HelpWords string // 插件帮助字段，发送 /help 指令显示
	AdminOnly bool   // 插件是否为管理员触发(对其他人隐藏)
	Order     int    // 插件执行顺序，从小到大
}
```

插件实现接口后自动触发，有如下接口

- `OnGroupMsg(bot.Bot, *command.Command, message.Message) bool`
  收到群消息触发，返回值代表是否执行后续插件
- `OnFriendMsg(bot.Bot, *command.Command, message.Message) bool`
  收到私聊消息触发，返回值代表是否执行后续插件

---

- `Start(cfg *viper.Viper)`
  Bot初始化时触发

**插件示例**(日志打印插件)

```go
package pluginlog

import (
	"log"
	"strings"

	"github.com/jeanhua/AniaBot/common/bot"
	"github.com/jeanhua/AniaBot/common/model/command"
	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/jeanhua/AniaBot/common/plugin"
)

type LogPlugin struct {
	plugin.Meta
}

func NewPlugin() *LogPlugin {
	p := &LogPlugin{}
	p.Name = "日志打印插件"
	p.HelpWords = "用于在控制台打印日志信息"
	p.AdminOnly = true
	return p
}

func (p *LogPlugin) OnGroupMsg(bot bot.Bot, cmd *command.Command, msg message.Message) bool {
	var rawStrMsg strings.Builder
	for _, m := range msg.Message {
		rawStrMsg.WriteString(m.FriendlyText())
	}
	name := msg.Sender.Card
	if name == "" {
		name = msg.Sender.Nickname
	}
	log.Printf("收到群聊消息[%d %s]: %s", msg.GroupId, name, rawStrMsg.String())
	return true
}

func (p *LogPlugin) OnFriendMsg(bot bot.Bot, cmd *command.Command, msg message.Message) bool {
	var rawStrMsg strings.Builder
	for _, m := range msg.Message {
		rawStrMsg.WriteString(m.FriendlyText())
	}
	name := msg.Sender.Card
	if name == "" {
		name = msg.Sender.Nickname
	}
	log.Printf("收到好友消息[%d %s]: %s", msg.Sender.UserId, name, rawStrMsg.String())
	return true
}
```

