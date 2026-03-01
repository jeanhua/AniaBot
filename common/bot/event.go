package bot

import (
	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/jeanhua/AniaBot/common/msgchain"
)

type Bot interface {
	botSendMsgItf
	botGetMsgItf
	botSysItf
	Stop()
}

type botSendMsgItf interface {
	// SendGroupMsg 发送群聊消息
	SendGroupMsg(groupId uint, chain msgchain.GroupChain) (msgId uint, success bool)
	// SendGroupAIVoiceMsg 发送群聊AI语音消息
	SendGroupAIVoiceMsg(groupId uint, character, msg string) (msgId uint, success bool)
	// SendFriendMsg 发送私聊消息
	SendFriendMsg(userId uint, chain msgchain.FriendChain) (msgId uint, success bool)
	// SendPokeMsg 发送戳一戳消息
	SendPokeMsg(userId uint, groupId *uint)
	// SendGroupForwardMsg 发送群聊合并转发消息
	SendGroupForwardMsg(groupId uint, chain msgchain.GroupForwardChain) (msgId uint, success bool)
	// SendFriendForwardMsg 发送私聊合并转发消息
	SendFriendForwardMsg(userId uint, chain msgchain.FriendForwardChain) (msgId uint, success bool)
}

type botGetMsgItf interface {
	// GetMsgDetail 获取消息详情
	GetMsgDetail(msgId uint) (msg *message.Message, success bool)
	// GetForwardMsg 获取合并转发消息详情
	GetForwardMsg(msgId string) (msgs *[]message.Message, success bool)
	// GetGroupUserInfo 获取群聊中某成员信息
	GetGroupUserInfo(groupId, userId uint) (info *message.GroupUserInfo, success bool)

	// GetFriendList 获取好友列表
	GetFriendList() (*[]message.Friend, bool)
}

type botSysItf interface {
	// GetNCrkey 获取rkey
	GetNCrkey() ([]message.NCrkey, bool)
}
