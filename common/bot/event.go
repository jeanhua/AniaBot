package bot

import (
	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/jeanhua/AniaBot/common/msgchain"
)

type Bot interface {
	SendGroupMsg(groupId uint, chain msgchain.Chain) (success bool, msgId uint)
	SendGroupAIVoiceMsg(groupId uint, character, msg string) (success bool, msgId uint)
	SendFriendMsg(userId uint, chain msgchain.Chain) (success bool, msgId uint)
	SendPokeMsg(userId uint, groupId *uint)
	GetMsgDetail(msgId uint) (bool, *message.Message)
}
