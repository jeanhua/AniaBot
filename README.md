<div align="center">
  <img src="./README/logo.jpg" width="200" alt="AniaBot Logo"/>
  <h1>AniaBot</h1>
  <p>一个 QQ 机器人框架喵~</p>
</div>

> 一个QQ机器人喵~ ，正在重构 [PinBot](https://github.com/jeanhua/PinBot) 中，有这些优势(对比PinBot)：
>
> - 更加灵活，可适配多种交换(http/websocket)
> - 性能提升(减少重复反序列化消耗，优化结构)
> - 结构更加清晰，支持插件动态实现接口自动触发对应事件

## todo

1. RAG支持
2. 完善内置插件系统

## 插件系统

**插件示例**

```go
package logplugin

import (
	"log"

	"github.com/jeanhua/AniaBot/common/bot"
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

func (p *LogPlugin) OnGroupMsg(bot bot.Bot, msg message.Message) bool {
	name := msg.Sender.Card
	if name == "" {
		name = msg.Sender.Nickname
	}
	log.Printf("收到群聊消息[%d %s]: %s", msg.GroupId, name, msg.RawMessage)
	return true
}

func (p *LogPlugin) OnFriendMsg(bot bot.Bot, msg message.Message) bool {
	name := msg.Sender.Card
	if name == "" {
		name = msg.Sender.Nickname
	}
	log.Printf("收到好友消息[%d %s]: %s", msg.Sender.UserId, name, msg.RawMessage)
	return true
}
```

>插件元数据：
>
>```go
>type Meta struct {
>	Name      string // 插件名字
>	HelpWords string // 插件帮助字段，发送 /help 指令显示
>	AdminOnly bool   // 插件是否为管理员触发(对其他人隐藏)
>	Order     int    // 插件执行顺序，从小到大
>}
>```
>
>插件实现接口后自动触发，有如下接口
>
>```go
>type Plugin interface {
>	GetMeta() *Meta // 嵌入结构体后自动实现
>}
>
>type BasicEvent interface {
>	OnGroupMsg(bot.Bot, message.Message) bool // 返回真继续执行后续插件，假则停止执行
>	OnFriendMsg(bot.Bot, message.Message) bool
>}
>
>type InitialEvent interface {
>	Init(cfg *viper.Viper) // 初始化事件，在系统开始时触发，适用于需要初始化的插件
>}
>```

**插件注册**

```go
func main() {
	// adapter := aniaadapter.NewNapcatHttpAdapter()
	adapter := aniaadapter.NewNapcatWebSocketAdapter()
	bot := aniabot.NewAniaBot(adapter)
    // 注册插件
	bot.AddPlugin(logplugin.NewPlugin())

	bot.Run()
}
```

