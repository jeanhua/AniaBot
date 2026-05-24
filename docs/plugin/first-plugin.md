# 第一个插件

本节将带你从零开始，编写一个响应 `/hello` 命令的问候插件。完成后，你将理解插件的基本结构和工作原理。

## 步骤 1：创建插件目录

在 `custom/plugins/` 下新建目录：

```
custom/plugins/pluginhello/
└── hello.go
```

::: tip 目录命名
目录名建议用小写字母，与包名一致。虽然 Go 允许目录名和包名不同，但保持一致能减少困惑。
:::

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

// HelloPlugin 是我们的第一个插件
type HelloPlugin struct {
    plugin.Meta
}

// NewPlugin 创建插件实例
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
    // cmd.Name 是去掉 / 的命令名
    // 如果消息是 "/hello"，cmd.Name 就是 "hello"
    if cmd.Name != "hello" {
        return true, nil // 不是 /hello 命令，交给后续插件处理
    }

    // 构造回复消息
    builder := msgchain.Builder().Group()
    builder.Reply(msg.MessageId)  // 引用原消息
    builder.Text("你好！我是 AniaBot，很高兴为你服务！")
    b.SendGroupMsg(msg.GroupId, builder.Build())

    return false, nil // 已经处理了，阻止后续插件
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

### 这段代码做了什么？

1. **定义结构体**：`HelloPlugin` 嵌入了 `plugin.Meta`，获得了框架提供的所有基础能力
2. **构造函数**：`NewPlugin()` 初始化插件元信息——名称、帮助文字、执行顺序等
3. **OnGroupMsg**：每条群消息到达时，框架会调用这个方法
   - 先检查命令名是不是 `hello`，不是就跳过（`return true`）
   - 是的话，构造一条回复消息并发送
   - 返回 `false` 告诉框架「我处理完了，不用给后面的插件了」
4. **OnFriendMsg**：私聊消息的处理逻辑，与群聊类似

## 步骤 3：注册插件

在 `cmd/main.go` 中导入并注册：

```go
import (
    // ...其他导入
    "github.com/jeanhua/AniaBot/custom/plugins/pluginhello"
)

func main() {
    adapter := napcat.NewNapcatWebSocketAdapter()
    bot := aniabot.NewAniaBot(adapter)

    bot.AddPlugin(pluginhello.NewPlugin()) // [!code ++]

    bot.Run()
}
```

::: warning 导入路径
确保导入路径与你的目录结构一致。如果你把插件放在其他位置，路径也要相应调整。
:::

## 步骤 4：测试

启动机器人后，在群聊中发送：

```
/hello
```

机器人会回复：**你好！我是 AniaBot，很高兴为你服务！**

在私聊中发送 `/hello` 也会收到回复（内容略有不同）。

## 命令触发规则

AniaBot 的命令解析规则如下：

| 消息内容 | `cmd.Name` | `cmd.Args` | `cmd.Mention` |
|---------|-----------|-----------|---------------|
| `/hello` | `hello` | `[]` | `false` |
| `/weather 北京 明天` | `weather` | `["北京", "明天"]` | `false` |
| `@机器人 /play music` | `play` | `["music"]` | `true` |
| `@机器人 你好` | `你好` | `[]` | `true` |
| `普通消息` | `""` | `[]` | `false` |

几个要点：
- 以 `/` 开头的消息，`/` 后面的部分被解析为命令名
- `@机器人` 后面的内容同样会被解析，同时 `cmd.Mention` 为 `true`
- 普通消息（不以 `/` 开头，也没 @ 机器人）的 `cmd.Name` 为空字符串

## 进阶：让插件更有趣

掌握了基本结构后，你可以尝试这些扩展：

### 发送图片

```go
func (p *HelloPlugin) OnGroupMsg(ctx context.Context, b bot.Bot, cmd command.Command, msg message.Message) (bool, error) {
    if cmd.Name != "hello" {
        return true, nil
    }

    builder := msgchain.Builder().Group()
    builder.Reply(msg.MessageId)
    builder.Text("送你一张壁纸！")
    builder.ImageUrl("https://api.yppp.net/api.php") // 随机二次元图片
    b.SendGroupMsg(msg.GroupId, builder.Build())

    return false, nil
}
```

### 带参数的命令

```go
func (p *HelloPlugin) OnGroupMsg(ctx context.Context, b bot.Bot, cmd command.Command, msg message.Message) (bool, error) {
    if cmd.Name != "greet" {
        return true, nil
    }

    name := "朋友"
    if len(cmd.Args) > 0 {
        name = cmd.Args[0] // 用户指定了称呼
    }

    builder := msgchain.Builder().Group()
    builder.Text(fmt.Sprintf("你好，%s！", name))
    b.SendGroupMsg(msg.GroupId, builder.Build())

    return false, nil
}
```

发送 `/greet 小明`，机器人会回复「你好，小明！」

### @ 触发模式

有些插件（如 AI 对话）不以 `/` 开头，而是通过 @ 机器人触发：

```go
func (p *HelloPlugin) OnGroupMsg(ctx context.Context, b bot.Bot, cmd command.Command, msg message.Message) (bool, error) {
    // 只响应 @ 了机器人的消息
    if !cmd.Mention {
        return true, nil
    }

    // cmd.Name + cmd.Args 拼接就是用户 @ 后面的完整内容
    userInput := cmd.Name + " " + strings.Join(cmd.Args, " ")

    builder := msgchain.Builder().Group()
    builder.Reply(msg.MessageId)
    builder.Text("你说了：" + userInput)
    b.SendGroupMsg(msg.GroupId, builder.Build())

    return false, nil
}
```

## 常见错误

### 忘记 return false

```go
// ❌ 错误：处理了命令但没有 return false
if cmd.Name == "hello" {
    builder := msgchain.Builder().Group()
    builder.Text("你好！")
    b.SendGroupMsg(msg.GroupId, builder.Build())
    // 缺少 return false，后续插件会继续处理这条消息
}
return true, nil
```

```go
// ✅ 正确：处理完命令后 return false
if cmd.Name == "hello" {
    builder := msgchain.Builder().Group()
    builder.Text("你好！")
    b.SendGroupMsg(msg.GroupId, builder.Build())
    return false, nil // 阻止后续插件
}
return true, nil
```

### 在 OnGroupMsg 中使用 Friend 构造器

```go
// ❌ 错误：群聊消息用了 Friend 构造器
builder := msgchain.Builder().Friend()
builder.Text("你好！")
b.SendGroupMsg(msg.GroupId, builder.Build()) // 可能导致消息格式异常
```

```go
// ✅ 正确：群聊用 Group，私聊用 Friend
builder := msgchain.Builder().Group()
builder.Text("你好！")
b.SendGroupMsg(msg.GroupId, builder.Build())
```

## 下一步

- [消息构造器](./message-builder) — 发送图片、表情、@ 等富文本消息
- [命令解析](./commands) — 更复杂的命令处理模式
- [数据存储](./storage) — 为插件添加持久化能力
- [常见模式](./patterns) — 学习更多插件开发技巧
