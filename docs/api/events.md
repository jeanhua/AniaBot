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
| `ctx` | `context.Context` | 上下文 |
| `bot` | `bot.Bot` | 机器人操作接口 |
| `cmd` | `command.Command` | 解析后的命令 |
| `msg` | `message.Message` | 原始消息 |

**返回值**：`true` 继续执行后续插件，`false` 阻止后续插件。

---

### OnFriendMsg

私聊消息事件。

```go
OnFriendMsg(ctx context.Context, bot bot.Bot, cmd command.Command, msg message.Message) (bool, error)
```

参数与返回值同 `OnGroupMsg`。

---

## 生命周期事件

### Start

插件启动时调用，用于读取配置、初始化资源（如 HTTP 客户端、数据库连接等）。

```go
Start(ctx context.Context, cfg *viper.Viper) error
```

### StartCron

注册定时任务。在所有插件 `Start` 完成后调用。

```go
StartCron(ctx context.Context, bot bot.Bot, c CronManager) error
```

### Awake

Bot 完全启动后触发，适合执行依赖其他模块完全启动后的操作（如发送上线通知）。

```go
Awake(ctx context.Context, bot bot.Bot) error
```

---

## 群通知事件

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

### OnGroupIncrease

群成员增加（入群）。

```go
OnGroupIncrease(ctx context.Context, bot bot.Bot, notice message.GroupIncreaseNotice) error
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

## 使用示例

```go
// 监听入群欢迎
func (p *WelcomePlugin) OnGroupIncrease(ctx context.Context, b bot.Bot, notice message.GroupIncreaseNotice) error {
    builder := msgchain.Builder().Group()
    builder.Mention(notice.UserId)
    builder.Text(" 欢迎加入！请阅读群规。")
    b.SendGroupMsg(notice.GroupId, builder.Build())
    return nil
}

// 监听撤回，记录被撤回的消息
func (p *AntiWithdrawalPlugin) OnGroupRecall(ctx context.Context, b bot.Bot, notice message.GroupRecallNotice) error {
    // 从缓存中找到被撤回的消息并转发给管理员
    return nil
}
```
