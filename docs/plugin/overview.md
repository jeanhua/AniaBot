# 插件系统概览

AniaBot 中一切功能都是插件。本章介绍插件系统的核心概念，读完你将理解插件如何被加载、执行与管理。

## 插件是什么

一个插件就是一个嵌入了 `plugin.Meta` 的结构体：

```go
type MyPlugin struct {
    plugin.Meta
}

func NewPlugin() *MyPlugin {
    return &MyPlugin{
        Meta: plugin.Meta{
            Name:      "我的插件",
            HelpWords: "这是 /help 中显示的介绍",
            Order:     plugin.LevelNormal,
            ShowFor:   plugininfo.ShowForGroup | plugininfo.ShowForFriend,
            Author:    "you",
            Version:   "1.0.0",
        },
    }
}
```

`Meta` 已经提供了 `Plugin` 接口全部方法的默认实现 —— 你只需**按需重写**关心的方法。

## Meta 字段

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `Name` | `string` | 插件名，也用于存储命名空间隔离 |
| `HelpWords` | `string` | `/help` 命令显示的帮助文本 |
| `AdminOnly` | `bool` | 为 `true` 时仅管理员能在 `/help` 中看到 |
| `ShowFor` | `ShowFor` | 显示范围：`ShowForGroup` / `ShowForFriend` / `ShowForNone`，可按位或 |
| `Order` | `int` | 执行顺序，从小到大 |
| `Author` / `Version` | `string` | 作者与版本信息 |

## 执行顺序（Order）

框架启动时按 `Order` 从小到大排序插件。预定义三档参考值：

```go
plugin.LevelLog        = -1000  // 日志层：最先执行，如日志插件、消息拦截器
plugin.LevelNormal     = 0      // 普通业务插件（默认值）
plugin.LevelPostHandle = 1000   // 后置处理：最后执行，如 AI 对话兜底
```

## 中间件链

群聊与私聊消息事件返回 `(bool, error)`：

- 返回 `true` —— **继续**传播给后续插件
- 返回 `false` —— **阻断**传播，后续插件收不到这条消息

```go
func (p *MyPlugin) OnGroupMsg(ctx context.Context, bot bot.Bot, cmd command.Command, msg message.Message) (bool, error) {
    if cmd.Name == "hello" {
        // 处理完毕，不再传递给其他插件
        return false, nil
    }
    return true, nil // 放行
}
```

::: warning 注意
**通知事件（Notice）不走中间件链** —— 全部 14 种通知会广播给所有插件，无法被阻断。某个插件 panic 也不会影响其他插件收到通知。
:::

## 生命周期

```
AddPlugin() 注册
    │
    ▼
Start(ctx, cfg)        ← 插件初始化（声明了 ConfigSchema 的配置结构体此时已自动填充完毕）
    │
    ▼
StartCron(ctx, bot, c) ← 注册 cron 定时任务
    │
    ▼
Awake(ctx, bot)        ← Bot 完全启动完成（此时可以发消息）
    │
    ▼
OnGroupMsg / OnFriendMsg / OnXxxNotice ...   ← 运行期事件
    │
    ▼
OnPanic(ctx, bot, name, err)  ← 任何插件 panic 时触发
```

## 依赖注入（DI）

在 `Start()` 之前，框架向每个插件注入以下依赖，直接通过 `Meta` 的字段使用：

| 字段 | 类型 | 用途 |
| --- | --- | --- |
| `Storage` | `storage.Storage` | 缓存层（TTL / 列表语义），已按插件名隔离 |
| `PersistentStorage` | `storage.PersistentStorage` | 持久化 KV 存储，已按插件名隔离 |
| `RestyClient` | `*resty.Client` | HTTP 客户端 |
| `Logger` | `*slog.Logger` | 结构化日志 |
| `SystemConfig` | `SystemConfig` | 系统级配置（如 `AdminId`） |

## 事件一览

| 类别 | 方法 | 说明 |
| --- | --- | --- |
| 消息 | `OnGroupMsg` / `OnFriendMsg` | 群聊 / 私聊消息，中间件链 |
| 通知 | `OnGroupUpload` `OnGroupAdmin` `OnGroupDecrease` `OnGroupIncrease` `OnGroupBan` `OnFriendAdd` `OnGroupRecall` `OnFriendRecall` `OnPoke` `OnLuckyKing` `OnHonor` `OnGroupMsgEmojiLike` `OnEssence` `OnGroupCard` | 14 种通知，广播制 |
| 启动 | `Start` / `StartCron` / `Awake` | 生命周期钩子 |
| 异常 | `OnPanic` | panic 通知 |

事件字段详见 [API · 事件接口](/api/events)。

## 并发模型与 panic 恢复

- 每个插件的每次调用都被 `safeExecute` 包裹 —— 单个插件 panic **不会**影响其他插件或主进程
- 插件内需要起协程时，使用 `bot.Go(name, f)` 代替裸 `go f()`，同样享有崩溃恢复
- panic 发生后，所有插件的 `OnPanic` 会被回调，系统插件默认私聊告警管理员

## 注册插件

在 `cmd/main.go` 中：

```go
adapter := napcat.NewNapcatWebSocketAdapter()
bot := core.NewAniaBot(adapter)

bot.AddPlugin(pluginsys.NewPluginSys())
bot.AddPlugin(myplugin.NewPlugin())   // ← 你的插件

bot.Run()
```

## 下一步

- [第一个插件](/plugin/first-plugin) —— 动手写代码
- [完整示例](/plugin/examples) —— 看几个真实插件的完整实现
