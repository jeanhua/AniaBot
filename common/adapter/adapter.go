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
	SendGroupMsg(groupId uint, chain msgchain.Chain) (success bool, msgId uint)
	SendGroupAIVoiceMsg(groupId uint, character, msg string) (success bool, msgId uint)
	SendFriendMsg(userId uint, chain msgchain.Chain) (success bool, msgId uint)
	SendPokeMsg(userId uint, groupId *uint)
	SendGroupForwardMsg(groupId uint, chain msgchain.ForwardChain) (success bool, msgId uint)
	SendFriendForwardMsg(userId uint, chain msgchain.ForwardChain) (success bool, msgId uint)
}

type GetMsg interface {
	GetMsgDetail(msgId uint) (bool, *message.Message)
}
