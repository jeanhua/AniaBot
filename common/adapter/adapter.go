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

type TriggerWrapper struct {
	OnGroupMsg  func(message.Message)
	OnFriendMsg func(message.Message)
}

type SendMsg interface {
	SendGroupMsg(groupId uint, chain msgchain.Chain) (success bool, msgId uint)
	SendGroupAIVoiceMsg(groupId uint, character, msg string) (success bool, msgId uint)
	SendFriendMsg(userId uint, chain msgchain.Chain) (success bool, msgId uint)
	SendPokeMsg(userId uint, groupId *uint)
}

type GetMsg interface {
	GetMsgDetail(msgId uint) (bool, *message.Message)
}
