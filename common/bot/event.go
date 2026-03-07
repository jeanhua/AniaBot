package bot

import (
	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/jeanhua/AniaBot/common/msgchain"
	"github.com/jeanhua/AniaBot/common/tracer"
)

type Bot interface {
	botSendMsgItf
	botGetMsgItf
	botSysItf
	pluginItf
	Stop()

	tracer.Tracer
}

type botSendMsgItf interface {
	// SendGroupMsg 发送群聊消息
	SendGroupMsg(groupId message.QID, chain msgchain.GroupChain) (msgId message.QID, success bool)
	// SendGroupAIVoiceMsg 发送群聊AI语音消息
	SendGroupAIVoiceMsg(groupId message.QID, character, msg string) (msgId message.QID, success bool)
	// SendFriendMsg 发送私聊消息
	SendFriendMsg(userId message.QID, chain msgchain.FriendChain) (msgId message.QID, success bool)
	// SendPokeMsg 发送戳一戳消息
	SendPokeMsg(userId message.QID, groupId *message.QID) (success bool)
	// SendGroupForwardMsg 发送群聊合并转发消息
	SendGroupForwardMsg(groupId message.QID, chain msgchain.GroupForwardChain) (msgId message.QID, success bool)
	// SendFriendForwardMsg 发送私聊合并转发消息
	SendFriendForwardMsg(userId message.QID, chain msgchain.FriendForwardChain) (msgId message.QID, success bool)
	// SetMsgEmojiLike 设置消息表情点赞
	SetMsgEmojiLike(msgId message.QID, emojiId int, like bool) (success bool)

	// SendGroupSign 群打卡
	SendGroupSign(groupId message.QID) (success bool)
}

type botGetMsgItf interface {
	// GetMsgDetail 获取消息详情
	GetMsgDetail(msgId message.QID) (msg *message.Message, success bool)
	// GetForwardMsg 获取合并转发消息详情
	GetForwardMsg(msgId message.QID) (msgs *[]message.Message, success bool)
	// GetGroupUserInfo 获取群聊中某成员信息
	GetGroupUserInfo(groupId, userId message.QID) (info *message.GroupUserInfo, success bool)

	// GetFriendList 获取好友列表
	GetFriendList() (*[]message.Friend, bool)
	// GetGroupDetail 获取群聊详情
	GetGroupDetail(groupId message.QID) (info *message.GroupInfo, success bool)

	// GetGroupMsgHistory 获取群聊消息历史记录
	GetGroupMsgHistory(groupId message.QID, count int) (*[]message.Message, bool)
	// GetFriendMsgHistory 获取好友消息历史记录
	GetFriendMsgHistory(userId message.QID, count int) (*[]message.Message, bool)

	// GetAIChatacter 获取AI角色列表
	GetAIChatacter() (*[]message.AIChatacter, bool)
}

type botSysItf interface {
	// GetNCrkey 获取rkey
	GetNCrkey() ([]message.NCrkey, bool)
}

type pluginItf interface {
	GetPluginList() []PluginInfo
}

type PluginInfo struct {
	Name      string
	HelpWords string
	AdminOnly bool
}
