# 第一个插件

本节将带你从零开始，编写一个响应 `/hello` 命令的问候插件。

## 步骤 1：创建插件目录

在 `custom/plugins/` 下新建目录：

```
custom/plugins/pluginhello/
└── hello.go
```

## 步骤 2：编写插件代码

```go
package pluginhello

import (
    "context"

    "github.com/jeanhua/AniaBot/common/bot"
    "github.com/jeanhua/AniaBot/common/model/command"
    "github.com/jeanhua/AniaBot/common/model/message"
    "github.com/jeanhua/AniaBot/common/msgchain"
    "github.com/jeanhua/AniaBot/common/plugin"
    "github.com/jeanhua/AniaBot/common/plugininfo"
)

type HelloPlugin struct {
    plugin.Meta
}

func NewPlugin() *HelloPlugin {
    return &HelloPlugin{
        Meta: plugin.Meta{
            Name:      "问候插件",
            HelpWords: "发送 /hello 触发问候",
            AdminOnly: false,
            ShowFor:   plugininfo.ShowForGroup | plugininfo.ShowForFriend,
            Author:    "jeanhua",
            Version:   "1.0.0",
            Order:     10,
        },
    }
}

// OnGroupMsg 处理群聊消息
func (p *HelloPlugin) OnGroupMsg(ctx context.Context, b bot.Bot, cmd command.Command, msg message.Message) (bool, error) {
    if cmd.Name != "hello" {
        return true, nil // 不是 /hello 命令，继续执行后续插件
    }

    builder := msgchain.Builder().Group()
    builder.Reply(msg.MessageId)  // 回复原消息
    builder.Text("你好！我是 AniaBot，很高兴为你服务！")
    b.SendGroupMsg(msg.GroupId, builder.Build())

    return false, nil // 阻止后续插件执行
}

// OnFriendMsg 处理私聊消息
func (p *HelloPlugin) OnFriendMsg(ctx context.Context, b bot.Bot, cmd command.Command, msg message.Message) (bool, error) {
    if cmd.Name != "hello" {
        return true, nil
    }

    builder := msgchain.Builder().Friend()
    builder.Text("你好！我是 AniaBot！")
    b.SendFriendMsg(msg.Sender.UserId, builder.Build())

    return false, nil
}
```

## 步骤 3：注册插件

在 `cmd/main.go` 中导入并注册：

```go
import (
    // ...
    "github.com/jeanhua/AniaBot/custom/plugins/pluginhello"
)

func main() {
    adapter := napcat.NewNapcatWebSocketAdapter()
    bot := aniabot.NewAniaBot(adapter)

    bot.AddPlugin(pluginhello.NewPlugin()) // [!code ++]

    bot.Run()
}
```

## 步骤 4：测试

启动机器人后，在群聊中发送：

```
/hello
```

机器人会回复：**你好！我是 AniaBot，很高兴为你服务！**

## 命令触发规则

AniaBot 的命令解析规则如下：

- 消息以 `/` 开头：`cmd.Name` 为命令名，`cmd.Args` 为后续参数
- `@机器人` 后跟命令：`cmd.Mention` 为 `true`，同时解析命令名和参数
- 普通消息：`cmd.Name` 为空字符串

```go
// 示例："/weather 北京 明天"
// cmd.Name = "weather"
// cmd.Args = ["北京", "明天"]
// cmd.Mention = false（未 @ 机器人）
```

## 下一步

- [消息构造器](./message-builder) — 发送图片、表情、@ 等富文本消息
- [命令解析](./commands) — 更复杂的命令处理模式
- [数据存储](./storage) — 为插件添加持久化能力
