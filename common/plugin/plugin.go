package plugin

import (
	"github.com/jeanhua/AniaBot/common/bot"
	"github.com/jeanhua/AniaBot/common/model/command"
	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/spf13/viper"
)

type Plugin interface {
	GetMeta() *Meta
	BasicEvent
	StartupEvent
	NoticeEvent
}

type BasicEvent interface {
	OnGroupMsg(bot.Bot, *command.Command, message.Message) bool
	OnFriendMsg(bot.Bot, *command.Command, message.Message) bool
}

type StartupEvent interface {
	Start(cfg *viper.Viper)
}

type NoticeEvent interface {
	OnGroupUpload(bot.Bot, message.GroupUploadNotice)
	OnGroupAdmin(bot.Bot, message.GroupAdminNotice)
	OnGroupDecrease(bot.Bot, message.GroupDecreaseNotice)
	OnGroupIncrease(bot.Bot, message.GroupIncreaseNotice)
	OnGroupBan(bot.Bot, message.GroupBanNotice)
	OnFriendAdd(bot.Bot, message.FriendAddNotice)
	OnGroupRecall(bot.Bot, message.GroupRecallNotice)
	OnFriendRecall(bot.Bot, message.FriendRecallNotice)
	OnPoke(bot.Bot, message.PokeNotice)
	OnLuckyKing(bot.Bot, message.LuckyKingNotice)
	OnHonor(bot.Bot, message.HonorNotice)
	OnGroupMsgEmojiLike(bot.Bot, message.GroupMsgEmojiLikeNotice)
	OnEssence(bot.Bot, message.EssenceNotice)
	OnGroupCard(bot.Bot, message.GroupCardNotice)
}
