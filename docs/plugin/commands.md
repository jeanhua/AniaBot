# 命令解析

AniaBot 在调用插件事件方法时，会将消息自动解析为 `command.Command` 结构体传入。理解命令解析规则是编写插件的基础。

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

要点：
- 以 `/` 开头 → 解析为命令，`/` 后第一个空格前的部分是命令名，其余是参数
- `@机器人` 后面的内容同样会被解析，同时 `Mention` 标记为 `true`
- 普通消息（不以 `/` 开头，没 @ 机器人）→ `Name` 为空字符串

## 基础命令处理

最常见的方式是用 `switch` 分发不同命令：

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

::: tip 为什么 default 要 return true？
`return true` 表示「这条消息不是我该处理的，交给下一个插件」。如果你的插件不处理某个命令却 return false，后面的插件就永远收不到这条消息了。
:::

## 处理参数

```go
func (p *WeatherPlugin) OnGroupMsg(ctx context.Context, b bot.Bot, cmd command.Command, msg message.Message) (bool, error) {
    if cmd.Name != "weather" {
        return true, nil
    }

    if len(cmd.Args) == 0 {
        // 用户没传参数，给出用法提示
        builder := msgchain.Builder().Group()
        builder.Text("用法：/weather <城市>\n示例：/weather 北京")
        b.SendGroupMsg(msg.GroupId, builder.Build())
        return false, nil
    }

    city := cmd.Args[0]
    // 查询天气逻辑...
    return false, nil
}
```

::: warning 始终检查参数长度
用户可能只发 `/weather` 而不带任何参数。如果不检查 `len(cmd.Args)` 直接访问 `cmd.Args[0]`，会导致数组越界 panic。
:::

## 仅响应 @ 触发

有些插件（如 AI 对话）只在用户 @ 机器人时才响应：

```go
func (p *AIChatPlugin) OnGroupMsg(ctx context.Context, b bot.Bot, cmd command.Command, msg message.Message) (bool, error) {
    if !cmd.Mention {
        return true, nil // 没有 @ 机器人，跳过
    }

    // 处理 @ 触发的对话
    // cmd.Name + cmd.Args 就是用户 @ 后面的完整内容
    userInput := cmd.Name + " " + strings.Join(cmd.Args, " ")
    response := p.chat(userInput)

    builder := msgchain.Builder().Group()
    builder.Reply(msg.MessageId)
    builder.Text(response)
    b.SendGroupMsg(msg.GroupId, builder.Build())

    return false, nil
}
```

## 子命令模式

当一个命令有多个子功能时，用参数来区分子命令：

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
        } else {
            // 提示用法
        }
    case "unban":
        // /admin unban <qq>
    case "list":
        // /admin list
    default:
        // 未知子命令
    }

    return false, nil
}
```

**真实案例**：`gdmusicplugin`（音乐插件）的命令设计：

```
/music 周杰伦           → 搜索音乐
/music get 3            → 发送第 3 首
/music next             → 下一页
/music prev             → 上一页
/music help             → 显示帮助
/music 周杰伦 -n 5 -s netease  → 带选项的搜索
```

解析逻辑：

```go
func (p *MusicPlugin) OnGroupMsg(ctx context.Context, b bot.Bot, cmd command.Command, msg message.Message) (bool, error) {
    if cmd.Name != "music" {
        return true, nil
    }

    if len(cmd.Args) == 0 {
        p.showHelp(b, msg)
        return false, nil
    }

    switch cmd.Args[0] {
    case "help":
        p.showHelp(b, msg)
    case "next":
        p.nextPage(b, msg)
    case "prev":
        p.prevPage(b, msg)
    case "get":
        if len(cmd.Args) >= 2 {
            p.sendSong(b, msg, cmd.Args[1])
        }
    default:
        // 第一个参数不是关键字，当作搜索关键词
        keyword := strings.Join(cmd.Args, " ")
        p.search(b, msg, keyword)
    }

    return false, nil
}
```

## 命令设计模式

### 模式一：简单命令

最简单的形式，一个命令做一件事：

```
/hello     → 打招呼
/acg       → 获取壁纸
/score     → 查看积分
```

### 模式二：带参数的命令

命令名 + 必选参数：

```
/weather 北京           → 查询北京天气
/music 周杰伦           → 搜索周杰伦的歌
/gr https://github.com/... → 分析仓库
```

### 模式三：子命令

命令名 + 子命令 + 可选参数：

```
/music get 3            → 获取第 3 首歌
/music next             → 下一页
/admin ban 123456       → 封禁用户
/gn gen                 → 立即生成群刊
```

### 模式四：@ 触发（非 / 命令）

不以 `/` 开头，通过 @ 机器人触发：

```
@机器人 今天天气怎么样    → AI 对话
@机器人 /gr https://...  → @ + 命令组合
```

### 模式五：自动检测（无需命令）

不检查命令名，对所有消息做模式匹配：

```go
func (p *DouyinParser) OnGroupMsg(ctx context.Context, b bot.Bot, cmd command.Command, msg message.Message) (bool, error) {
    // 用正则匹配消息中的抖音链接
    matches := p.re.FindString(msg.RawMessage)
    if matches == "" {
        return true, nil // 没有抖音链接，跳过
    }
    // 解析抖音链接...
    return false, nil
}
```

**真实案例**：`douyinparser` 和 `urlparser` 都使用这种模式，自动识别消息中的特定链接。

## 混合模式实战

实际插件通常会混合使用多种模式。以 `groupnewsletter` 为例：

```go
func (p *GroupNewsletter) OnGroupMsg(ctx context.Context, b bot.Bot, cmd command.Command, msg message.Message) (bool, error) {
    // 模式一：检查是否启用了该群
    if !p.isGroupEnabled(msg.GroupId) {
        return true, nil
    }

    // 模式四：@ 触发的命令
    if cmd.Mention && cmd.Name == "gn" {
        return p.handleCommand(b, cmd, msg)
    }

    // 模式五：自动收集所有消息（无需命令）
    p.collectMessage(ctx, b, msg)
    return true, nil // 继续传递给后续插件
}
```

这个插件同时做了三件事：
1. 过滤不相关的群
2. 响应 `/gn` 命令
3. 收集所有群消息（不管是不是命令）

## 下一步

- [消息构造器](./message-builder) — 构造回复消息
- [数据存储](./storage) — 持久化数据
- [常见模式](./patterns) — 更多设计模式
