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
	SendGroupMsg(groupId message.QID, chain msgchain.GroupChain) (msgId message.QID, success bool)
	SendGroupAIVoiceMsg(groupId message.QID, character, msg string) (msgId message.QID, success bool)
	SendFriendMsg(userId message.QID, chain msgchain.FriendChain) (msgId message.QID, success bool)
	SendPokeMsg(userId message.QID, groupId *message.QID) (success bool)
	SendGroupForwardMsg(groupId message.QID, chain msgchain.GroupForwardChain) (msgId message.QID, success bool)
	SendFriendForwardMsg(userId message.QID, chain msgchain.FriendForwardChain) (msgId message.QID, success bool)
	SetMsgEmojiLike(msgId message.QID, emojiId int, like bool) (success bool)

	SendGroupSign(groupId message.QID) (success bool)
}

type GetMsg interface {
	GetMsgDetail(msgId message.QID) (msg *message.Message, success bool)
	GetGroupUserInfo(groupId, userId message.QID) (info *message.GroupUserInfo, success bool)
	GetForwardMsg(msgId message.QID) (msgs *[]message.Message, success bool)
	GetNCrkey() ([]message.NCrkey, bool)
	GetFriendList() (*[]message.Friend, bool)
	GetGroupDetail(groupId message.QID) (info *message.GroupInfo, success bool)
	GetGroupMsgHistory(groupId message.QID, count int) (*[]message.Message, bool)
	GetFriendMsgHistory(userId message.QID, count int) (*[]message.Message, bool)
	GetAIChatacter() (*[]message.AIChatacter, bool)
}
