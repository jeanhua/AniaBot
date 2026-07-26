# 事件接口参考

插件通过重写 `plugin.Meta` 的对应方法接收事件。所有事件方法签名中的 `bot bot.Bot` 为框架注入的机器人操作接口（见 [Bot 接口](/api/bot)）。

## 消息事件（中间件链）

按 `Order` 从小到大依次执行，返回 `false` 阻断传播。

```go
// OnGroupMsg 收到群聊消息触发
OnGroupMsg(ctx context.Context, bot bot.Bot, cmd command.Command, msg message.Message) (bool, error)

// OnFriendMsg 收到私聊消息触发
OnFriendMsg(ctx context.Context, bot bot.Bot, cmd command.Command, msg message.Message) (bool, error)
```

### message.Message

```go
type Message struct {
    Time        uint            // 消息时间戳
    PostType    string          // 上报类型，"message"
    MessageType string          // "group" / "private"
    SubType     string          // 子类型
    MessageId   QID             // 消息 ID
    MessageSeq  int             // 消息序号
    UserId      QID             // 发送者 QQ
    GroupId     QID             // 群号（私聊为空字符串）
    Message     []OB11Segment   // OneBot v11 消息段
    RawMessage  string          // 纯文本
    Sender      MessageSender   // 发送者信息
    SelfId      QID             // 机器人自身 QQ
}

type MessageSender struct {
    UserId   QID
    Nickname string
    Sex      string
    Card     string  // 群名片
    Role     string  // "owner" / "admin" / "member"
}

type OB11Segment struct {
    Type string         // "text" / "at" / "image" / "face" / ...
    Data map[string]any
}
```

`message.QID` 是 `string`（十进制数字字符串）的封装，提供 `String()` / `Uint64()` 方法与 `FromString()` / `FromUint64()` 构造函数。用整数构造 QID 时**不要**使用 `message.QID(x)`（这会把 int 转成 Unicode 码点），应使用 `message.FromUint64(uint64(x))`。

### command.Command

```go
type Command struct {
    Name    string   // / 后的命令名
    Args    []string // 空白切分的参数
    Mention bool     // 是否 @ 了机器人
}
```

解析规则见 [命令解析](/plugin/commands)。

## 通知事件（广播制）

全部 14 种通知广播给**所有**插件，无阻断、无顺序影响。返回的 `error` 仅用于日志记录。

所有通知结构体内嵌：

```go
type BasicNotice struct {
    Time       uint
    PostType   string  // "notice"
    SelfId     QID
    NoticeType string
}
```

### OnGroupUpload —— 群文件上传

```go
type GroupUploadNotice struct {
    BasicNotice
    GroupId QID
    UserId  QID
    File    struct {
        Id    QID
        Name  string
        Size  uint
        Busid uint
    }
}
```

### OnGroupAdmin —— 群管理员变动

```go
type GroupAdminNotice struct {
    BasicNotice
    SubType string // "set" / "unset"
    GroupId QID
    UserId  QID
}
```

### OnGroupDecrease —— 群成员减少

```go
type GroupDecreaseNotice struct {
    BasicNotice
    SubType    string // "leave" / "kick" / "kick_me"
    GroupId    QID
    OperatorId QID    // 操作者（主动退群时 = UserId）
    UserId     QID
}
```

### OnGroupIncrease —— 群成员增加

```go
type GroupIncreaseNotice struct {
    BasicNotice
    SubType    string // "approve"（审批入群）/ "invite"（邀请入群）
    GroupId    QID
    OperatorId QID
    UserId     QID
}
```

### OnGroupBan —— 群禁言

```go
type GroupBanNotice struct {
    BasicNotice
    SubType    string // "ban" / "lift_ban"
    GroupId    QID
    OperatorId QID
    UserId     QID    // 为 0 表示全员禁言
    Duration   uint   // 禁言时长（秒），解禁时为 0
}
```

### OnFriendAdd —— 好友添加

```go
type FriendAddNotice struct {
    BasicNotice
    UserId QID
}
```

### OnGroupRecall —— 群消息撤回

```go
type GroupRecallNotice struct {
    BasicNotice
    GroupId    QID
    UserId     QID   // 消息发送者
    OperatorId QID   // 操作者（自己撤回时 = UserId）
    MessageId  uint
}
```

### OnFriendRecall —— 好友消息撤回

```go
type FriendRecallNotice struct {
    BasicNotice
    UserId    QID
    MessageId uint
}
```

### OnPoke —— 戳一戳

```go
type PokeNotice struct {
    BasicNotice
    SubType  string  // "poke"
    GroupId  *QID    // 群内戳一戳才有值，私聊为 nil
    UserId   QID     // 发起者
    TargetId QID     // 被戳者
}
```

### OnLuckyKing —— 群红包运气王

```go
type LuckyKingNotice struct {
    BasicNotice
    SubType  string // "lucky_king"
    GroupId  QID
    UserId   QID    // 发红包者
    TargetId QID    // 运气王
}
```

### OnHonor —— 群荣誉变更

```go
type HonorNotice struct {
    BasicNotice
    SubType   string // "honor"
    GroupId   QID
    HonorType string // "talkative"（龙王）/ "performer"（群聊之火）/ ...
    UserId    QID
}
```

### OnGroupMsgEmojiLike —— 群消息表情回应

```go
type GroupMsgEmojiLikeNotice struct {
    BasicNotice
    GroupId    QID
    OperatorId QID
    MessageId  QID
    Likes      []struct {
        Code  int // 表情 ID
        Count int // 数量
    }
}
```

### OnEssence —— 群精华消息变更

```go
type EssenceNotice struct {
    BasicNotice
    SubType    string // "add" / "delete"
    GroupId    QID
    MessageId  QID
    SenderId   QID    // 消息发送者
    OperatorId QID    // 操作者
}
```

### OnGroupCard —— 群名片变更

```go
type GroupCardNotice struct {
    BasicNotice
    GroupId QID
    UserId  QID
    CardNew string
    CardOld string
}
```

## 生命周期事件

```go
// Start 插件初始化，读取配置。返回错误将标记插件初始化失败
Start(ctx context.Context, cfg *viper.Viper) error

// StartCron 注册定时任务，c 为框架共享的 cron 管理器
StartCron(ctx context.Context, bot bot.Bot, c plugin.CronManager) error

// Awake Bot 完全启动完成，此时可以安全发送消息
Awake(ctx context.Context, bot bot.Bot) error
```

`CronManager` 接口：

```go
type CronManager interface {
    AddFunc(spec string, cmd func()) (cron.EntryID, error)
}
```

## 异常事件

```go
// OnPanic 任何插件（或 bot.Go 协程）运行时 panic 时触发
OnPanic(ctx context.Context, bot bot.Bot, name string, err any)
```

- `name` 为 panic 发生的上下文标签（如「群聊消息事件」或 `bot.Go` 的任务名）
- 系统插件默认实现：私聊通知管理员，1 分钟内防抖

## 执行顺序常量

```go
const (
    LevelLog        = -1000 // 日志层
    LevelNormal     = 0     // 普通插件
    LevelPostHandle = 1000  // 后置处理层
)
```
