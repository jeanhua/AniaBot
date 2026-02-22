
# 事件接口参考

本页列出 AniaBot 插件可实现的事件接口与简要说明。事件方法在 `common/plugin/metainfo.go` 中定义，插件可通过实现这些方法来响应机器人运行时的各种消息与通知。

## 消息类事件

- OnGroupMsg(bot.Bot, command.Command, message.Message) bool
	- 说明：收到群聊消息时触发。返回 `true` 继续执行后续插件，返回 `false` 阻止后续插件。
	- 场景：命令处理、关键字监听、复读等。

- OnFriendMsg(bot.Bot, command.Command, message.Message) bool
	- 说明：收到好友（私聊）消息时触发。返回值含义同上。

## 初始化与生命周期事件

- `Start(cfg *viper.Viper)`
	- 说明：插件启动时调用，用于初始化（读取配置、建立客户端等）。接收全局配置 `viper` 实例。

- `StartCron(bot.Bot, c CronManager)`
	- 说明：在插件中初始化定时任务（Cron）时调用，`CronManager` 提供注册定时任务的能力。

- `Awake(bot.Bot)`
	- 说明：Bot 启动完成后调用，适合执行依赖其他模块启动完成后的操作。

## 群/好友通知（Notice）事件

这些事件用于处理平台或协议层的通知，如群成员变动、文件上传、撤回等。

- `OnGroupUpload(bot.Bot, message.GroupUploadNotice)`
	- 说明：群文件上传通知。

- `OnGroupAdmin(bot.Bot, message.GroupAdminNotice)`
	- 说明：群管理员变更通知（设置/取消管理员）。

- `OnGroupDecrease(bot.Bot, message.GroupDecreaseNotice)`
	- 说明：群成员减少（退群/被踢）。

- `OnGroupIncrease(bot.Bot, message.GroupIncreaseNotice)`
	- 说明：群成员增加（入群）。

- `OnGroupBan(bot.Bot, message.GroupBanNotice)`
	- 说明：群成员被禁言通知。

- `OnFriendAdd(bot.Bot, message.FriendAddNotice)`
	- 说明：收到新的好友添加通知。

- `OnGroupRecall(bot.Bot, message.GroupRecallNotice)`
	- 说明：群消息撤回通知。

- `OnFriendRecall(bot.Bot, message.FriendRecallNotice)`
	- 说明：好友消息撤回通知。

- `OnPoke(bot.Bot, message.PokeNotice)`
	- 说明：收到戳一戳（poke）通知。

- `OnLuckyKing(bot.Bot, message.LuckyKingNotice)`
	- 说明：群运气王通知（群内活动相关）。

- `OnHonor(bot.Bot, message.HonorNotice)`
	- 说明：群荣誉变更通知（如名片、头衔等平台定义的荣誉）。

- `OnGroupMsgEmojiLike(bot.Bot, message.GroupMsgEmojiLikeNotice)`
	- 说明：群消息表情回应/点赞通知。

- `OnEssence(bot.Bot, message.EssenceNotice)`
	- 说明：群精华消息变更通知。

- `OnGroupCard(bot.Bot, message.GroupCardNotice)`
	- 说明：群名片（群昵称）变更通知。

## 使用建议与示例

- 插件返回值：对于消息处理事件推荐在成功处理并希望阻止后续插件时返回 `false`；若只是观察或记录日志则返回 `true`。
- 示例：实现一个简单的群消息处理

```go
func (p *MyPlugin) OnGroupMsg(b bot.Bot, cmd command.Command, msg message.Message) bool {
		if cmd.Name == "hello" {
				builder := msgchain.Builder().Group()
				builder.Text("你好，AniaBot 在。")
				b.SendGroupMsg(msg.GroupId, builder.Build())
				return false
		}
		return true
}
```

> 此插件将在接收群聊中 `/hello` 消息后发送回复

