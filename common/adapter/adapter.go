package adapter

import (
	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/jeanhua/AniaBot/common/msgchain"
	"github.com/spf13/viper"
)

type Adapter interface {
	SendMsg
	GetMsg
	SetTrigger(TriggerWrapper)
	Serve(*viper.Viper)
}

type MessageHandler func(message.Message)
type GroupUploadHandler func(message.GroupUploadNotice)
type GroupAdminHandler func(message.GroupAdminNotice)
type GroupDecreaseHandler func(message.GroupDecreaseNotice)
type GroupIncreaseHandler func(message.GroupIncreaseNotice)
type GroupBanHandler func(message.GroupBanNotice)
type FriendAddHandler func(message.FriendAddNotice)
type GroupRecallHandler func(message.GroupRecallNotice)
type FriendRecallHandler func(message.FriendRecallNotice)
type PokeHandler func(message.PokeNotice)
type LuckyKingHandler func(message.LuckyKingNotice)
type HonorHandler func(message.HonorNotice)
type GroupMsgEmojiLikeHandler func(message.GroupMsgEmojiLikeNotice)
type EssenceHandler func(message.EssenceNotice)
type GroupCardHandler func(message.GroupCardNotice)

// TriggerWrapper 回调函数包装器
type TriggerWrapper struct {
	OnGroupMsg          MessageHandler
	OnFriendMsg         MessageHandler
	OnGroupUpload       GroupUploadHandler
	OnGroupAdmin        GroupAdminHandler
	OnGroupDecrease     GroupDecreaseHandler
	OnGroupIncrease     GroupIncreaseHandler
	OnGroupBan          GroupBanHandler
	OnFriendAdd         FriendAddHandler
	OnGroupRecall       GroupRecallHandler
	OnFriendRecall      FriendRecallHandler
	OnPoke              PokeHandler
	OnLuckyKing         LuckyKingHandler
	OnHonor             HonorHandler
	OnGroupMsgEmojiLike GroupMsgEmojiLikeHandler
	OnEssence           EssenceHandler
	OnGroupCard         GroupCardHandler
}

type SendMsg interface {
	SendGroupMsg(groupId uint, chain msgchain.GroupChain) (msgId uint, success bool)
	SendGroupAIVoiceMsg(groupId uint, character, msg string) (msgId uint, success bool)
	SendFriendMsg(userId uint, chain msgchain.FriendChain) (msgId uint, success bool)
	SendPokeMsg(userId uint, groupId *uint)
	SendGroupForwardMsg(groupId uint, chain msgchain.GroupForwardChain) (msgId uint, success bool)
	SendFriendForwardMsg(userId uint, chain msgchain.FriendForwardChain) (msgId uint, success bool)
	SetMsgEmojiLike(msgId uint, emojiId int, like bool) (success bool)

	SendGroupSign(groupId uint) (success bool)
}

type GetMsg interface {
	GetMsgDetail(msgId uint) (msg *message.Message, success bool)
	GetGroupUserInfo(groupId, userId uint) (info *message.GroupUserInfo, success bool)
	GetForwardMsg(msgId string) (msgs *[]message.Message, success bool)
	GetNCrkey() ([]message.NCrkey, bool)
	GetFriendList() (*[]message.Friend, bool)
	GetGroupDetail(groupId uint) (info *message.GroupInfo, success bool)
}
