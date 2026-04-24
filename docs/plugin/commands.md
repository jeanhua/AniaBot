# 命令解析

AniaBot 在调用插件事件方法时，会将消息自动解析为 `command.Command` 结构体传入。

## Command 结构体

```go
type Command struct {
    Name    string   // 命令名（不含 /）
    Args    []string // 命令参数列表
    Mention bool     // 是否 @ 了机器人
}
```

## 解析规则

| 消息内容 | `Name` | `Args` | `Mention` |
|---------|--------|--------|-----------|
| `/hello` | `hello` | `[]` | `false` |
| `/weather 北京 明天` | `weather` | `["北京", "明天"]` | `false` |
| `@机器人 /play music` | `play` | `["music"]` | `true` |
| `@机器人 你好` | `你好` | `[]` | `true` |
| `普通消息` | `""` | `[]` | `false` |

## 基础命令处理

```go
func (p *MyPlugin) OnGroupMsg(ctx context.Context, b bot.Bot, cmd command.Command, msg message.Message) (bool, error) {
    switch cmd.Name {
    case "help":
        // 处理 /help
    case "weather":
        // 处理 /weather
    default:
        return true, nil // 非本插件命令，继续后续插件
    }
    return false, nil
}
```

## 处理参数

```go
func (p *WeatherPlugin) OnGroupMsg(ctx context.Context, b bot.Bot, cmd command.Command, msg message.Message) (bool, error) {
    if cmd.Name != "weather" {
        return true, nil
    }

    if len(cmd.Args) == 0 {
        builder := msgchain.Builder().Group()
        builder.Text("用法：/weather <城市>")
        b.SendGroupMsg(msg.GroupId, builder.Build())
        return false, nil
    }

    city := cmd.Args[0]
    // 查询天气逻辑...
    return false, nil
}
```

## 仅响应 @ 触发

```go
func (p *AIChatPlugin) OnGroupMsg(ctx context.Context, b bot.Bot, cmd command.Command, msg message.Message) (bool, error) {
    if !cmd.Mention {
        return true, nil // 没有 @ 机器人，跳过
    }

    // 处理 @ 触发的对话
    response := p.chat(cmd.Name + " " + strings.Join(cmd.Args, " "))
    builder := msgchain.Builder().Group()
    builder.Reply(msg.MessageId)
    builder.Text(response)
    b.SendGroupMsg(msg.GroupId, builder.Build())

    return false, nil
}
```

## 子命令模式

```go
func (p *AdminPlugin) OnGroupMsg(ctx context.Context, b bot.Bot, cmd command.Command, msg message.Message) (bool, error) {
    if cmd.Name != "admin" || len(cmd.Args) == 0 {
        return true, nil
    }

    switch cmd.Args[0] {
    case "ban":
        // /admin ban <qq>
        if len(cmd.Args) >= 2 {
            p.banUser(cmd.Args[1])
        }
    case "unban":
        // /admin unban <qq>
    case "list":
        // /admin list
    }

    return false, nil
}
```
