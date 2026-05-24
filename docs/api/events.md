# 事件接口参考

插件通过实现以下方法响应各类事件。所有方法均为可选，只需实现你关心的即可。

## 消息事件

### OnGroupMsg

群聊消息事件，每条群聊消息触发一次。

```go
OnGroupMsg(ctx context.Context, bot bot.Bot, cmd command.Command, msg message.Message) (bool, error)
```

| 参数 | 类型 | 说明 |
|------|------|------|
| `ctx` | `context.Context` | 上下文，用于超时控制 |
| `bot` | `bot.Bot` | 机器人操作接口（发送消息、获取信息等） |
| `cmd` | `command.Command` | 解析后的命令（Name、Args、Mention） |
| `msg` | `message.Message` | 原始消息（SenderId、GroupId、MessageId 等） |

**返回值**：`true` 继续执行后续插件，`false` 阻止后续插件。

**msg 常用字段**：

| 字段 | 类型 | 说明 |
|------|------|------|
| `msg.GroupId` | `int64` | 群号 |
| `msg.Sender.UserId` | `int64` | 发送者 QQ 号 |
| `msg.Sender.Nickname` | `string` | 发送者昵称 |
| `msg.MessageId` | `int64` | 消息 ID（用于引用回复） |
| `msg.RawMessage` | `string` | 原始消息文本 |

---

### OnFriendMsg

私聊消息事件。

```go
OnFriendMsg(ctx context.Context, bot bot.Bot, cmd command.Command, msg message.Message) (bool, error)
```

参数与返回值同 `OnGroupMsg`。注意私聊消息没有 `msg.GroupId`。

---

## 生命周期事件

### Start

插件启动时调用，用于读取配置、初始化资源（如 HTTP 客户端、数据库连接等）。

```go
Start(ctx context.Context, cfg *viper.Viper) error
```

**cfg 参数**：全局 Viper 实例，插件通过 `cfg.GetString("plugin.xxx.key")` 读取自己的配置节。详见 [配置说明](../guide/configuration)。

**典型用法**：

```go
func (p *MyPlugin) Start(ctx context.Context, cfg *viper.Viper) error {
    // 读取配置
    p.apiKey = cfg.GetString("plugin.myplugin.api_key")
    p.model = cfg.GetString("plugin.myplugin.model")

    // 初始化客户端
    p.client = openai.NewClient(
        option.WithBaseURL(cfg.GetString("plugin.myplugin.base_url")),
        option.WithAPIKey(p.apiKey),
    )

    // 创建插件自己的 context（用于后台任务）
    p.pluginCtx, p.cancel = context.WithCancel(context.Background())

    p.Logger.Info("插件初始化完成")
    return nil
}
```

::: tip 返回 error
如果 `Start` 返回非 nil 的 error，框架会打印错误日志但不会阻止 Bot 启动。对于必须的配置项（如 API Key），建议在 `Start` 中检查并返回 error。
:::

---

### StartCron

注册定时任务。在所有插件 `Start` 完成后调用。

```go
StartCron(ctx context.Context, bot bot.Bot, c CronManager) error
```

详见 [定时任务](../plugin/cron)。

---

### Awake

Bot 完全启动后触发，适合执行依赖其他模块完全启动后的操作（如启动后台 goroutine、发送上线通知）。

```go
Awake(ctx context.Context, bot bot.Bot) error
```

**典型用法**：

```go
func (p *MyPlugin) Awake(ctx context.Context, b bot.Bot) error {
    // 启动后台工作线程
    b.Go("MyPlugin工作线程", func() {
        p.worker(b)
    })

    // 发送上线通知给管理员
    builder := msgchain.Builder().Friend()
    builder.Text("Bot 已上线！")
    b.SendFriendMsg(p.adminId, builder.Build())

    return nil
}
```

::: tip 为什么不在 Start 中启动 goroutine？
`Start` 调用时，其他插件可能还没初始化完成，Bot 本身也没完全就绪。在 `Awake` 中启动后台任务，可以确保所有插件和基础设施都已就绪。
:::

---

## 群通知事件

所有通知事件都返回 `error`。返回非 nil 的 error 会被框架记录到日志。

### OnGroupUpload

群文件上传通知。

```go
OnGroupUpload(ctx context.Context, bot bot.Bot, notice message.GroupUploadNotice) error
```

### OnGroupAdmin

群管理员变更通知（设置/取消管理员）。

```go
OnGroupAdmin(ctx context.Context, bot bot.Bot, notice message.GroupAdminNotice) error
```

### OnGroupDecrease

群成员减少（退群/被踢）。

```go
OnGroupDecrease(ctx context.Context, bot bot.Bot, notice message.GroupDecreaseNotice) error
```

**典型用法**：退群告别

```go
func (p *MyPlugin) OnGroupDecrease(ctx context.Context, b bot.Bot, notice message.GroupDecreaseNotice) error {
    builder := msgchain.Builder().Group()
    builder.Text(fmt.Sprintf("用户 %d 离开了群聊", notice.UserId))
    b.SendGroupMsg(notice.GroupId, builder.Build())
    return nil
}
```

### OnGroupIncrease

群成员增加（入群）。

```go
OnGroupIncrease(ctx context.Context, bot bot.Bot, notice message.GroupIncreaseNotice) error
```

**典型用法**：入群欢迎

```go
func (p *WelcomePlugin) OnGroupIncrease(ctx context.Context, b bot.Bot, notice message.GroupIncreaseNotice) error {
    builder := msgchain.Builder().Group()
    builder.Mention(notice.UserId)
    builder.Text(" 欢迎加入！请阅读群规。")
    b.SendGroupMsg(notice.GroupId, builder.Build())
    return nil
}
```

### OnGroupBan

群成员禁言通知。

```go
OnGroupBan(ctx context.Context, bot bot.Bot, notice message.GroupBanNotice) error
```

### OnGroupRecall

群消息撤回通知。

```go
OnGroupRecall(ctx context.Context, bot bot.Bot, notice message.GroupRecallNotice) error
```

**典型用法**：防撤回（从缓存中找回被撤回的消息）

```go
func (p *AntiWithdrawalPlugin) OnGroupRecall(ctx context.Context, b bot.Bot, notice message.GroupRecallNotice) error {
    // 从缓存中找到被撤回的消息
    cached, ok := p.getCache(notice.GroupId, notice.MessageId)
    if !ok {
        return nil
    }

    // 通知管理员
    builder := msgchain.Builder().Friend()
    builder.Text(fmt.Sprintf("群 %d 有消息被撤回：\n%s", notice.GroupId, cached))
    b.SendFriendMsg(p.adminId, builder.Build())
    return nil
}
```

### OnGroupMsgEmojiLike

群消息表情回应通知。

```go
OnGroupMsgEmojiLike(ctx context.Context, bot bot.Bot, notice message.GroupMsgEmojiLikeNotice) error
```

### OnEssence

群精华消息变更通知。

```go
OnEssence(ctx context.Context, bot bot.Bot, notice message.EssenceNotice) error
```

### OnGroupCard

群名片（群昵称）变更通知。

```go
OnGroupCard(ctx context.Context, bot bot.Bot, notice message.GroupCardNotice) error
```

---

## 好友通知事件

### OnFriendAdd

收到新好友添加通知。

```go
OnFriendAdd(ctx context.Context, bot bot.Bot, notice message.FriendAddNotice) error
```

**典型用法**：新好友自动打招呼

```go
func (p *MyPlugin) OnFriendAdd(ctx context.Context, b bot.Bot, notice message.FriendAddNotice) error {
    builder := msgchain.Builder().Friend()
    builder.Text("你好！我是 AniaBot，发送 /help 查看功能列表。")
    b.SendFriendMsg(notice.UserId, builder.Build())
    return nil
}
```

### OnFriendRecall

好友消息撤回通知。

```go
OnFriendRecall(ctx context.Context, bot bot.Bot, notice message.FriendRecallNotice) error
```

---

## 其他通知事件

### OnPoke

戳一戳通知。

```go
OnPoke(ctx context.Context, bot bot.Bot, notice message.PokeNotice) error
```

### OnLuckyKing

群运气王通知。

```go
OnLuckyKing(ctx context.Context, bot bot.Bot, notice message.LuckyKingNotice) error
```

### OnHonor

群荣誉变更通知。

```go
OnHonor(ctx context.Context, bot bot.Bot, notice message.HonorNotice) error
```

---

## 通知事件的共同特点

- **广播机制**：所有通知事件会发送给**所有**已注册的插件，不存在短路机制（不像消息事件可以 return false 阻止）
- **只读语义**：通知事件主要用于「感知」发生了什么，而不是「控制」消息流向
- **返回 error**：返回非 nil 的 error 会被框架记录到日志，但不会影响其他插件

## 组合使用示例

以下示例展示如何组合多个事件实现一个完整的功能：

```go
type WelcomePlugin struct {
    plugin.Meta
}

// 新人入群时发送欢迎消息
func (p *WelcomePlugin) OnGroupIncrease(ctx context.Context, b bot.Bot, notice message.GroupIncreaseNotice) error {
    builder := msgchain.Builder().Group()
    builder.Mention(notice.UserId)
    builder.Text(" 欢迎新成员！发送 /help 查看功能。")
    b.SendGroupMsg(notice.GroupId, builder.Build())
    return nil
}

// 有人退群时记录日志
func (p *WelcomePlugin) OnGroupDecrease(ctx context.Context, b bot.Bot, notice message.GroupDecreaseNotice) error {
    p.Logger.Info("成员退群",
        "groupId", notice.GroupId,
        "userId", notice.UserId,
    )
    return nil
}

// 有人被禁言时通知管理员
func (p *WelcomePlugin) OnGroupBan(ctx context.Context, b bot.Bot, notice message.GroupBanNotice) error {
    if notice.Duration > 0 {
        builder := msgchain.Builder().Friend()
        builder.Text(fmt.Sprintf("群 %d：用户 %d 被禁言 %d 秒",
            notice.GroupId, notice.UserId, notice.Duration))
        b.SendFriendMsg(p.adminId, builder.Build())
    }
    return nil
}
```
