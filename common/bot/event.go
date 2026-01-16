package bot

import (
	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/jeanhua/AniaBot/common/msgchain"
)

type Bot interface {
	botOuterItf
}

type botOuterItf interface {
	// SendGroupMsg 发送群聊消息
	SendGroupMsg(groupId uint, chain msgchain.Chain) (success bool, msgId uint)

	// SendGroupAIVoiceMsg 发送群聊AI语音消息
	SendGroupAIVoiceMsg(groupId uint, character, msg string) (success bool, msgId uint)

	// SendFriendMsg 发送私聊消息
	SendFriendMsg(userId uint, chain msgchain.Chain) (success bool, msgId uint)

	// SendPokeMsg 发送戳一戳消息
	SendPokeMsg(userId uint, groupId *uint)

	// GetMsgDetail 获取消息详情
	GetMsgDetail(msgId uint) (bool, *message.Message)

	// SendGroupForwardMsg 发送群聊合并转发消息
	SendGroupForwardMsg(groupId uint, chain msgchain.ForwardChain) (success bool, msgId uint)

	// SendFriendForwardMsg 发送私聊合并转发消息
	SendFriendForwardMsg(userId uint, chain msgchain.ForwardChain) (success bool, msgId uint)

	// GetGroupUserInfo 获取群聊中某成员信息
	GetGroupUserInfo(groupId, userId uint) (success bool, info *message.GroupUserInfo)
}
