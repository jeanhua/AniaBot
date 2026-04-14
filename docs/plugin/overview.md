# 插件开发概览

AniaBot 的所有功能均通过插件实现。插件是一个实现了特定接口方法的 Go 结构体，框架在运行时自动调用这些方法。

## 插件基础结构

每个插件都嵌入 `plugin.Meta`，并通过 `NewPlugin()` 函数创建：

```go
package pluginhello

import "github.com/jeanhua/AniaBot/common/plugin"
import "github.com/jeanhua/AniaBot/common/plugininfo"

type HelloPlugin struct {
    plugin.Meta
}

func NewPlugin() *HelloPlugin {
    return &HelloPlugin{
        Meta: plugin.Meta{
            Name:      "问候插件",          // 插件名称（唯一标识，勿随意修改）
            HelpWords: "发送 /hello 触发",  // /help 显示的说明
            AdminOnly: false,              // true 时非管理员看不到帮助信息
            ShowFor:   plugininfo.ShowForGroup | plugininfo.ShowForFriend,
            Author:    "jeanhua",
            Version:   "1.0.0",
            Order:     10,                 // 执行顺序，数字越小越先执行
        },
    }
}
```

## Meta 字段说明

| 字段 | 类型 | 说明 |
|------|------|------|
| `Name` | string | 插件唯一名称，也是存储空间的隔离依据 |
| `HelpWords` | string | `/help` 命令展示的插件说明 |
| `AdminOnly` | bool | 是否仅管理员可见帮助信息 |
| `ShowFor` | uint | 显示范围：`ShowForGroup`、`ShowForFriend` 可按位组合 |
| `Author` | string | 插件作者 |
| `Version` | string | 版本号 |
| `Order` | int | 插件执行优先级，越小越先执行 |

::: warning 注意
`Name` 字段是插件存储空间的隔离依据（通过 base64 编码生成前缀）。修改插件名称后，原有持久化数据将无法访问。
:::

## 可实现的事件方法

插件通过实现以下方法来响应各类事件，**只需实现你关心的方法**，其余无需实现：

### 消息事件

```go
// 群聊消息
OnGroupMsg(ctx context.Context, bot bot.Bot, cmd command.Command, msg message.Message) (bool, error)

// 私聊消息
OnFriendMsg(ctx context.Context, bot bot.Bot, cmd command.Command, msg message.Message) (bool, error)
```

返回值 `bool`：`true` 继续执行后续插件，`false` 阻止后续插件。

### 生命周期事件

```go
// 插件启动，读取配置、初始化资源
Start(ctx context.Context, cfg *viper.Viper) error

// 注册定时任务
StartCron(ctx context.Context, bot bot.Bot, c CronManager) error

// Bot 完全启动后触发
Awake(ctx context.Context, bot bot.Bot) error
```

### 通知事件

群/好友的各类通知，如撤回、入群、禁言等，详见 [事件接口参考](../api/events)。

## 注册插件

在 `cmd/main.go` 中通过 `AddPlugin` 注册：

```go
bot.AddPlugin(pluginhello.NewPlugin())
```

插件会按 `Order` 从小到大的顺序依次执行。

## 下一步

- [第一个插件](./first-plugin) — 动手写一个完整的 Hello 插件
- [消息构造器](./message-builder) — 构造各类消息
- [命令解析](./commands) — 处理命令和参数
- [数据存储](./storage) — 持久化插件数据
