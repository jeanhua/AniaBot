<div align="center">
  <img src="./README/logo.png" width="200" alt="AniaBot Logo"/>
  <h1>AniaBot</h1>
  <p>一个插件驱动型 QQ 机器人框架</p>
</div>



> 正在重构 [PinBot](https://github.com/jeanhua/PinBot) 中，有这些优势(对比PinBot)：
>
> - 更加灵活，可适配多种上层协议(目前已实现napcat http/websocket协议)
> - 性能提升(减少重复反序列化消耗，优化结构)
> - 结构更加清晰，支持插件动态实现接口自动触发对应事件

![framework](./README/framework.png)

## 一、消息构造器

1. 构造器
   ```go
   // 群聊消息构造器
   builder := msgchain.Builder.Group()
   // 私聊消息构造器
   builder := msgchain.Builder.Friend()
   ```

2. 消息构造
   ```go
   builder.Mention(msg.Sender.UserId) // AT某人
   builder.Text("你没有权限哦") // 添加文本消息
   // ......
   ```

   有如下API

   ```go
   Text(text string)
   Face(faceId uint) // 参考 https://bot.q.qq.com/wiki/develop/api-v2/openapi/emoji/model.html#EmojiType
   ImageUrl(url string)
   ImageBase64(bs64code string)
   ImageLocal(path string)
   Reply(msgId uint)
   RecordUrl(url string)
   RecordLocal(path string)
   Raw(rawMsg []message.OB11Segment)
   Mention(userId uint) // group only
   ```

3. 发送消息

   ```go
   // 发送群聊消息
   bot.SendGroupMsg(msg.GroupId, builder.Build())
   // 发送私聊消息
   bot.SendFriendMsg(msg.Sender.UserId, builder.Build())
   ```

## 二、插件指南

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

## 三、插件定义

**插件元数据**：

```go
type Meta struct {
	Name      string // 插件名字
	HelpWords string // 插件帮助字段，发送 /help 指令显示
	AdminOnly bool   // 插件是否为管理员触发(对其他人隐藏)
	Order     int    // 插件执行顺序，从小到大
}
```

插件重写如下接口后自动触发，有如下接口

- `OnGroupMsg(bot.Bot, *command.Command, message.Message) bool`
  收到群消息触发，返回值代表是否执行后续插件
- `OnFriendMsg(bot.Bot, *command.Command, message.Message) bool`
  收到私聊消息触发，返回值代表是否执行后续插件

- `Start(cfg *viper.Viper)`
  Bot初始化时触发
  
- 消息通知接口，[详情查看定义](./common/plugin/metainfo.go)

## 四、最小可行方案

```go
package main

import (
	"github.com/jeanhua/AniaBot/ania/adapter/napcat"
	"github.com/jeanhua/AniaBot/ania/aniabot"

	"github.com/jeanhua/AniaBot/common/bot"
	"github.com/jeanhua/AniaBot/common/model/command"
	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/jeanhua/AniaBot/common/msgchain"
	"github.com/jeanhua/AniaBot/common/plugin"
	// "github.com/jeanhua/AniaBot/ania/plugins/pluginlog"
	// "github.com/jeanhua/AniaBot/ania/plugins/pluginrepeat"
)

func main() {
	// adapter := napcat.NewNapcatHttpAdapter() // HTTP 适配器
	adapter := napcat.NewNapcatWebSocketAdapter() // Websocket适配器
	bot := aniabot.NewAniaBot(adapter)
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
func (p *CustomPlugin) OnGroupMsg(bot bot.Bot, cmd *command.Command, msg message.Message) bool {
	if cmd != nil && cmd.Mention && cmd.Name == "test" { // 判断条件：@[bot] /test 有效
		builder := msgchain.Builder.Group()            // 群消息构造器
		builder.Text("测试成功")                           // 构造消息
		bot.SendGroupMsg(msg.GroupId, builder.Build()) // 发送消息
	}
	return true // 表示继续执行下一个插件
}
```

