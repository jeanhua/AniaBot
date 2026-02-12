package plugin

import (
	"github.com/jeanhua/AniaBot/common/bot"
	"github.com/jeanhua/AniaBot/common/model/command"
	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/robfig/cron/v3"
	"github.com/spf13/viper"
)

type Plugin interface {
	GetMeta() *Meta
	BasicEvent
	StartupEvent
	NoticeEvent
}

type BasicEvent interface {
	// OnGroupMsg 收到群聊消息触发事件
	OnGroupMsg(bot.Bot, command.Command, message.Message) bool
	// OnFriendMsg 收到私聊消息触发事件
	OnFriendMsg(bot.Bot, command.Command, message.Message) bool
}

type CronManager interface {
	// AddFunc 添加定时任务
	AddFunc(spec string, cmd func()) (cron.EntryID, error)
}

type StartupEvent interface {
	// Start 插件初始化事件
	Start(cfg *viper.Viper)
	// StartCron 初始化cron事件
	StartCron(bot bot.Bot, c CronManager)
}

type NoticeEvent interface {
	// OnGroupUpload 处理群文件上传
	OnGroupUpload(bot.Bot, message.GroupUploadNotice)
	// OnGroupAdmin 处理群管理员变动
	OnGroupAdmin(bot.Bot, message.GroupAdminNotice)
	// OnGroupDecrease 处理群成员减少
	OnGroupDecrease(bot.Bot, message.GroupDecreaseNotice)
	// OnGroupIncrease 处理群成员增加
	OnGroupIncrease(bot.Bot, message.GroupIncreaseNotice)
	// OnGroupBan 处理群禁言
	OnGroupBan(bot.Bot, message.GroupBanNotice)
	// OnFriendAdd 处理好友添加
	OnFriendAdd(bot.Bot, message.FriendAddNotice)
	// OnGroupRecall 处理群消息撤回
	OnGroupRecall(bot.Bot, message.GroupRecallNotice)
	// OnFriendRecall 处理好友消息撤回
	OnFriendRecall(bot.Bot, message.FriendRecallNotice)
	// OnPoke 处理戳一戳
	OnPoke(bot.Bot, message.PokeNotice)
	// OnLuckyKing 处理运气王
	OnLuckyKing(bot.Bot, message.LuckyKingNotice)
	// OnHonor 处理群荣誉变更
	OnHonor(bot.Bot, message.HonorNotice)
	// OnGroupMsgEmojiLike 处理群消息表情回应
	OnGroupMsgEmojiLike(bot.Bot, message.GroupMsgEmojiLikeNotice)
	// OnEssence 处理群精华消息变更
	OnEssence(bot.Bot, message.EssenceNotice)
	// OnGroupCard 处理群名片变更
	OnGroupCard(bot.Bot, message.GroupCardNotice)
}
