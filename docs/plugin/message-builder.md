# 消息构造器

AniaBot 提供链式调用风格的消息构造器，支持文本、图片、视频、文件、语音、表情、@ 等所有消息类型，以及合并转发消息。所有方法都返回构造器自身，可以像搭积木一样自由组合。

## 创建构造器

```go
import "github.com/jeanhua/AniaBot/common/msgchain"

// 群聊消息构造器
builder := msgchain.Builder().Group()

// 私聊消息构造器
builder := msgchain.Builder().Friend()

// 群聊合并转发构造器
fwd := msgchain.Builder().GroupForward()

// 私聊合并转发构造器
fwd := msgchain.Builder().FriendForward()
```

::: warning 群聊 vs 私聊
群聊消息必须用 `.Group()`，私聊消息必须用 `.Friend()`。混用可能导致消息格式异常。发送方法也要对应：群聊用 `SendGroupMsg`，私聊用 `SendFriendMsg`。
:::

## 方法速览

| 方法 | 群聊 | 私聊 | 说明 |
|------|:----:|:----:|------|
| `Text(text)` | ✅ | ✅ | 文本消息 |
| `Face(faceId)` | ✅ | ✅ | QQ 表情 |
| `Reply(msgId)` | ✅ | ✅ | 引用回复 |
| `ImageUrl(url)` | ✅ | ✅ | 图片（URL） |
| `ImageLocal(path)` | ✅ | ✅ | 图片（本地路径） |
| `ImageBase64(bs64)` | ✅ | ✅ | 图片（Base64） |
| `VideoUrl(url)` | ✅ | ✅ | 视频（URL） |
| `VideoLocal(path)` | ✅ | ✅ | 视频（本地路径） |
| `VideoBase64(bs64)` | ✅ | ✅ | 视频（Base64） |
| `RecordUrl(url)` | ✅ | ✅ | 语音（URL） |
| `RecordLocal(path)` | ✅ | ✅ | 语音（本地路径） |
| `RecordBase64(bs64)` | ✅ | ✅ | 语音（Base64） |
| `FileUrl(name, url)` | ✅ | ✅ | 文件（URL） |
| `FileLocal(name, path)` | ✅ | ✅ | 文件（本地路径） |
| `FileBase64(name, bs64)` | ✅ | ✅ | 文件（Base64） |
| `Mention(userId)` | ✅ | ❌ | @ 某人 |
| `Raw(segments...)` | ✅ | ✅ | 原始 OB11 消息段 |

## 文本

```go
builder.Text("这是一段文本")
```

支持换行：

```go
builder.Text("第一行\n第二行")
```

## 表情

传入 QQ 表情 ID：

```go
builder.Face(14)  // 微笑
builder.Face(76)  // 再见
```

## 引用回复

```go
builder.Reply(msg.MessageId)
```

::: warning
`Reply` 通常放在消息链最前面，这样用户一眼就能看到你在回复哪条消息。
:::

## 图片

支持 URL、本地文件、Base64 三种来源：

```go
// URL — 最常用，直接传图片链接
builder.ImageUrl("https://example.com/image.jpg")

// 本地文件（绝对路径）
// 注意：如果是容器化部署，路径要相对于 napcat 的文件系统
builder.ImageLocal("/data/images/avatar.png")

// Base64（不含 data:image/... 前缀，直接传编码字符串）
builder.ImageBase64("iVBORw0KGgoAAAANSUhEUgAA...")
```

**真实案例**：`acgwallpaper` 插件直接用 URL 发送随机壁纸：

```go
// 来自 custom/plugins/acgwallpaper/plugin.go
builder := msgchain.Builder().Group()
builder.ImageUrl("https://api.yppp.net/api.php") // 返回随机二次元图片
bot.SendGroupMsg(msg.GroupId, builder.Build())
```

## 视频

```go
builder.VideoUrl("https://example.com/video.mp4")
builder.VideoLocal("/data/videos/clip.mp4")
builder.VideoBase64("AAAAIGZ0eXBpc29t...")
```

**真实案例**：`douyinparser` 插件解析抖音链接后发送无水印视频：

```go
builder := msgchain.Builder().Group()
builder.Text(fmt.Sprintf("🎬 %s\n%s", title, desc))
builder.VideoUrl(videoURL) // 解析出的无水印视频地址
bot.SendGroupMsg(msg.GroupId, builder.Build())
```

## 语音

```go
builder.RecordUrl("https://example.com/audio.mp3")
builder.RecordLocal("/data/audio/hello.silk")
builder.RecordBase64("AAAA...")
```

::: info
napcat 支持的语音格式以其文档为准，通常为 silk 或 mp3。
:::

## 文件

文件方法比图片多一个 `name` 参数（显示的文件名）：

```go
builder.FileUrl("报告.pdf", "https://example.com/report.pdf")
builder.FileLocal("数据.xlsx", "/data/export/data.xlsx")
builder.FileBase64("说明.txt", "SGVsbG8gV29ybGQ=")
```

**真实案例**：`gdmusicplugin` 插件搜索到歌曲后发送 MP3 文件：

```go
builder := msgchain.Builder().Group()
builder.FileUrl(songName+".mp3", songURL)
bot.SendGroupMsg(msg.GroupId, builder.Build())
```

## @ 某人（仅群聊）

```go
builder.Mention(msg.Sender.UserId)  // @ 消息发送者
builder.Mention(12345678)           // @ 指定 QQ 号
```

**常见用法**：先 @ 用户，再回复内容：

```go
builder := msgchain.Builder().Group()
builder.Mention(msg.Sender.UserId)
builder.Text(" 你的积分是 100 分")
bot.SendGroupMsg(msg.GroupId, builder.Build())
```

## 链式调用

所有方法均返回构造器自身，支持链式调用：

```go
b.SendGroupMsg(msg.GroupId,
    msgchain.Builder().Group().
        Reply(msg.MessageId).
        Mention(msg.Sender.UserId).
        Text(" 你好！").
        Face(14).
        Build(),
)
```

## 发送消息

```go
// 发送群聊消息
b.SendGroupMsg(msg.GroupId, builder.Build())

// 发送私聊消息
b.SendFriendMsg(msg.Sender.UserId, builder.Build())
```

::: tip 发送结果
`SendGroupMsg` 和 `SendFriendMsg` 返回 `(messageId, ok)`。`ok` 为 `false` 表示发送失败，可以据此做错误处理。
:::

## 合并转发消息

将多条消息合并成一条「聊天记录」形式发送。这在展示多条相关内容时非常有用。

### 群聊合并转发

```go
fwd := msgchain.Builder().GroupForward()

// 添加消息节点（模拟不同用户发言）
fwd.Message(
    msg.Sender.UserId,
    msg.Sender.Nickname,
    msgchain.Builder().Group().Text("第一条消息").Build(),
)
fwd.Message(
    bot.SelfId(),
    "AniaBot",
    msgchain.Builder().Group().Text("第二条消息").ImageUrl("https://example.com/img.jpg").Build(),
)

b.SendGroupForwardMsg(msg.GroupId, fwd.Build())
```

### 私聊合并转发

```go
fwd := msgchain.Builder().FriendForward()

fwd.Message(
    msg.Sender.UserId,
    msg.Sender.Nickname,
    msgchain.Builder().Friend().Text("这是转发内容").Build(),
)

b.SendFriendForwardMsg(msg.Sender.UserId, fwd.Build())
```

**真实案例**：`chatrecordsmaker` 插件通过私聊交互收集消息，最后用合并转发发送伪造的聊天记录：

```go
// 来自 custom/plugins/chatrecordsmaker/plugin.go
fwd := msgchain.Builder().FriendForward()
for _, m := range session.messages {
    fwd.Message(m.userId, m.nickname,
        msgchain.Builder().Friend().Text(m.content).Build())
}
b.SendFriendForwardMsg(msg.Sender.UserId, fwd.Build())
```

## Raw — 原始消息段

当内置方法不满足需求时，可直接构造 OB11 消息段：

```go
import "github.com/jeanhua/AniaBot/common/model/message"

builder.Raw(message.OB11Segment{
    Type: "share",
    Data: map[string]any{
        "url":     "https://example.com",
        "title":   "分享标题",
        "content": "分享描述",
        "image":   "https://example.com/thumb.jpg",
    },
})
```

::: tip 什么时候用 Raw？
大多数场景用内置方法就够了。`Raw` 主要用于：
- 分享链接卡片（`share` 类型）
- JSON 消息（`json` 类型）
- 其他 OneBot v11 支持但框架未封装的消息段
:::

## 最佳实践

1. **Reply 放最前面**：引用回复通常作为消息链的第一个元素
2. **先文字后媒体**：先发文字说明，再附上图片/视频/文件
3. **控制消息长度**：单条文本消息过长可能被 QQ 截断，建议分段发送
4. **错误处理**：检查 `SendGroupMsg` 的返回值，发送失败时做降级处理
5. **群聊用 Group，私聊用 Friend**：不要混用构造器类型
