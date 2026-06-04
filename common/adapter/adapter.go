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
	// SendGroupMsg 发送群消息
	SendGroupMsg(groupId message.QID, chain msgchain.GroupChain) (msgId message.QID, success bool)
	// SendGroupAIVoiceMsg 发送群AI语音消息
	SendGroupAIVoiceMsg(groupId message.QID, character, msg string) (msgId message.QID, success bool)
	// SendFriendMsg 发送好友消息
	SendFriendMsg(userId message.QID, chain msgchain.FriendChain) (msgId message.QID, success bool)
	// SendPokeMsg 发送戳一戳消息
	SendPokeMsg(userId message.QID, groupId *message.QID) (success bool)
	// SendGroupForwardMsg 发送群转发消息
	SendGroupForwardMsg(groupId message.QID, chain msgchain.GroupForwardChain) (msgId message.QID, success bool)
	// SendFriendForwardMsg 发送好友转发消息
	SendFriendForwardMsg(userId message.QID, chain msgchain.FriendForwardChain) (msgId message.QID, success bool)
	// SetMsgEmojiLike 设置消息表情点赞
	SetMsgEmojiLike(msgId message.QID, emojiId int, like bool) (success bool)
	// SendGroupSign 发送群签到消息
	SendGroupSign(groupId message.QID) (success bool)
}

type GetMsg interface {
	// GetMsgDetail 获取消息详情
	GetMsgDetail(msgId message.QID) (msg *message.Message, success bool)
	// GetGroupUserInfo 获取群用户信息
	GetGroupUserInfo(groupId, userId message.QID) (info *message.GroupUserInfo, success bool)
	// GetForwardMsg 获取转发消息
	GetForwardMsg(msgId message.QID) (msgs *[]message.Message, success bool)
	// GetNCrkey 获取NCRKEY
	GetNCrkey() ([]message.NCrkey, bool)
	// GetFriendList 获取好友列表
	GetFriendList() (*[]message.Friend, bool)
	// GetGroupList 获取群聊列表
	GetGroupList() (*[]message.GroupInfo, bool)
	// GetGroupDetail 获取群详情
	GetGroupDetail(groupId message.QID) (info *message.GroupInfo, success bool)
	// GetGroupMsgHistory 获取群消息历史
	GetGroupMsgHistory(groupId message.QID, count int, message_seq int) (*[]message.Message, bool)
	// GetFriendMsgHistory 获取好友消息历史
	GetFriendMsgHistory(userId message.QID, count int, message_seq int) (*[]message.Message, bool)
	// GetAIChatacter 获取AI聊天角色
	GetAIChatacter() (*[]message.AIChatacter, bool)
	// GetPrivateFileURL 获取私聊文件URL
	GetPrivateFileURL(userId message.QID, fileId string) (string, bool)
}
