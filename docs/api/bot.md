# Bot 接口参考

`bot.Bot` 是插件与机器人交互的唯一入口，每个事件方法都会注入。它聚合了消息收发、信息查询、系统能力与协程管理。

::: info 错误处理约定
所有方法返回 `(value, bool)` —— 失败时内部已记录日志，以 `false` 返回，不暴露 error。
:::

## 发送消息

```go
// SendGroupMsg 发送群聊消息
SendGroupMsg(groupId message.QID, chain msgchain.GroupChain) (msgId message.QID, success bool)

// SendFriendMsg 发送私聊消息
SendFriendMsg(userId message.QID, chain msgchain.FriendChain) (msgId message.QID, success bool)

// SendGroupForwardMsg 发送群聊合并转发
SendGroupForwardMsg(groupId message.QID, chain msgchain.GroupForwardChain) (msgId message.QID, success bool)

// SendFriendForwardMsg 发送私聊合并转发
SendFriendForwardMsg(userId message.QID, chain msgchain.FriendForwardChain) (msgId message.QID, success bool)

// SendGroupAIVoiceMsg 发送群聊 AI 语音（character 为角色 ID，可用 GetAIChatacter 查询）
SendGroupAIVoiceMsg(groupId message.QID, character, msg string) (msgId message.QID, success bool)

// SendPokeMsg 戳一戳。groupId 传 nil 表示私聊戳一戳
SendPokeMsg(userId message.QID, groupId *message.QID) (success bool)

// SetMsgEmojiLike 给消息贴/取消表情回应
SetMsgEmojiLike(msgId message.QID, emojiId int, like bool) (success bool)

// SendGroupSign 群打卡
SendGroupSign(groupId message.QID) (success bool)
```

消息链的构造见 [消息构造器](/plugin/message-builder)。

## 查询消息与资料

```go
// GetMsgDetail 获取单条消息详情
GetMsgDetail(msgId message.QID) (msg *message.Message, success bool)

// GetForwardMsg 获取合并转发消息内容
GetForwardMsg(msgId message.QID) (msgs *[]message.Message, success bool)

// GetGroupUserInfo 获取群成员信息
GetGroupUserInfo(groupId, userId message.QID) (info *message.GroupUserInfo, success bool)

// GetFriendList 获取好友列表
GetFriendList() (*[]message.Friend, bool)

// GetGroupList 获取群聊列表
GetGroupList() (*[]message.GroupInfo, bool)

// GetGroupDetail 获取群详情
GetGroupDetail(groupId message.QID) (info *message.GroupInfo, success bool)

// GetGroupMsgHistory 获取群消息历史。message_seq 传 0 从最新开始
GetGroupMsgHistory(groupId message.QID, count int, messageSeq int) (*[]message.Message, bool)

// GetFriendMsgHistory 获取私聊消息历史
GetFriendMsgHistory(userId message.QID, count int, messageSeq int) (*[]message.Message, bool)

// GetAIChatacter 获取可用的 AI 语音角色列表
GetAIChatacter() (*[]message.AIChatacter, bool)

// GetPrivateFileURL 获取私聊文件的下载地址
GetPrivateFileURL(userId message.QID, fileId string) (string, bool)
```

### 常用返回结构

```go
type GroupUserInfo struct {
    GroupID      QID
    UserID       QID
    Nickname     string
    Card         string  // 群名片
    Sex          string
    Age          int
    JoinTime     uint    // 入群时间
    LastSentTime uint    // 最后发言时间
    Level        string  // 活跃等级
    Role         string  // "owner" / "admin" / "member"
    Title        string  // 专属头衔
    Unfriendly   bool
    IsRobot      bool
}
```

## 系统能力

```go
// GetNCrkey 获取 NapCat rkey（用于图片/文件 URL 续期）
GetNCrkey() ([]message.NCrkey, bool)

// GetPluginList 获取已注册插件信息（/help 的数据来源）
GetPluginList() []plugininfo.PluginInfo

// Stop 停止机器人
Stop()
```

`plugininfo.PluginInfo` 包含 `Name` / `HelpWords` / `AdminOnly` / `ShowFor` / `Author` / `Version`。

## 协程管理（Tracer）

```go
// Go 启动一个受管协程：panic 自动恢复并通知所有插件的 OnPanic
Go(name string, f func())
```

`name` 用于日志与 panic 告警中标识任务来源。插件内的任何后台协程都应该用 `bot.Go` 启动，而不是裸 `go`。
