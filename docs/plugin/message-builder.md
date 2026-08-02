# 消息构造器

`msgchain` 包提供链式构造器，用于拼装 OneBot v11 消息段。四种入口对应四种消息形态：

```go
msgchain.Builder().Group()         // 群聊消息
msgchain.Builder().Friend()        // 私聊消息
msgchain.Builder().GroupForward()  // 群聊合并转发
msgchain.Builder().FriendForward() // 私聊合并转发
```

## 快速上手

```go
chain := msgchain.Builder().Group()
chain.Mention(msg.Sender.UserId)     // @某人（仅群聊）
chain.Text(" 你好！")
chain.Face(14)                       // QQ 自带表情
chain.ImageUrl("https://example.com/pic.png")

msgId, ok := bot.SendGroupMsg(msg.GroupId, chain.Build())
```

方法可以链式连写：

```go
chain := msgchain.Builder().Friend().
    Text("任务完成 ✅").
    ImageUrl("https://example.com/report.png")
bot.SendFriendMsg(userId, chain.Build())
```

## 普通消息方法

群聊（`GroupChainBuilder`）与私聊（`FriendChainBuilder`）方法一致，唯一区别是群聊多了 `Mention`：

| 方法 | 说明 |
| --- | --- |
| `Text(text)` | 文本 |
| `Face(faceId)` | QQ 小黄脸表情（id 为数字） |
| `Mention(userId)` | @某人（**仅群聊**） |
| `Reply(msgId)` | 引用回复某条消息 |
| `ImageUrl(url)` / `ImageLocal(path)` / `ImageBase64(b64)` | 图片：网络 / 本地文件 / Base64 |
| `VideoUrl(url)` / `VideoLocal(path)` / `VideoBase64(b64)` | 视频，三种来源同上 |
| `RecordUrl(url)` / `RecordLocal(path)` / `RecordBase64(b64)` | 语音，三种来源同上 |
| `FileUrl(name, url)` / `FileLocal(name, path)` / `FileBase64(name, b64)` | 文件，需指定显示文件名 |
| `Raw(segments...)` | 直接追加原始 `OB11Segment`，用于转发收到的消息段 |

最后调用 `Build()` 得到链对象，传给 `bot.SendGroupMsg` / `bot.SendFriendMsg`。

### Raw 的典型用途：原样转发

复读机插件就是这么实现的 —— 把收到的消息段原封不动发回去：

```go
chain := msgchain.Builder().Group()
chain.Raw(msg.Message...) // msg.Message 是 []message.OB11Segment
bot.SendGroupMsg(msg.GroupId, chain.Build())
```

## 合并转发消息

把多条消息打包成一条「聊天记录」转发，防撤回插件用它回顾消息：

```go
fb := msgchain.Builder().GroupForward()

for _, m := range messages {
    node := msgchain.Builder().Group()
    node.Text(m.Content)
    // 每个 node 显示为 m.UserId 这个人（昵称为 m.Nickname）发的消息
    fb.Message(m.UserId, m.Nickname, node.Build())
}

bot.SendGroupForwardMsg(groupId, fb.Build())
```

私聊合并转发同理：`Builder().FriendForward()` + `bot.SendFriendForwardMsg`。

::: tip 伪造聊天记录？
`Message(userId, nickname, chain)` 的 userId 和 nickname 完全由你指定 —— 利用这一点可以实现「伪造聊天记录」等趣味玩法。
:::

## 发送接口

构造好链之后，通过 `bot.Bot` 发送：

```go
bot.SendGroupMsg(groupId, groupChain)  // → (msgId, ok) 公共能力，所有平台可用
bot.SendFriendMsg(userId, friendChain) // → (msgId, ok) 公共能力，所有平台可用

// 以下为 QQ 平台专属能力，需先断言 bot.QQ（事件来源为 QQ 适配器时成功）：
qb := bot.(bot.QQ)
qb.SendGroupForwardMsg(groupId, forwardChain)    // → (msgId, ok)
qb.SendFriendForwardMsg(userId, forwardChain)    // → (msgId, ok)
qb.SendGroupAIVoiceMsg(groupId, character, text) // AI 语音（需 NapCat 支持）
qb.SendPokeMsg(userId, &groupId)                 // 戳一戳（groupId 可传 nil 表示私聊）
```

::: tip 多平台
公共能力 `SendGroupMsg` / `SendFriendMsg` 在所有平台可用（飞书等平台内部自动翻译）；
QQ 专属方法在 `bot.QQ` 可选接口中，类型断言探测，断言失败即平台不支持。
::: 

所有发送方法返回 `(msgId, success)`，失败时记得处理：

```go
if _, ok := bot.SendGroupMsg(msg.GroupId, chain.Build()); !ok {
    p.Logger.Error("消息发送失败", "group", msg.GroupId)
}
```

完整接口见 [API · Bot 接口](/api/bot)。
