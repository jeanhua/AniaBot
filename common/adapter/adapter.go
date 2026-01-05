package adapter

import (
	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/jeanhua/AniaBot/common/msgchain"
	"github.com/spf13/viper"
)

type Adapter interface {
	BasicEventOut
	BasicEventInp
	Serve(*viper.Viper)
}

type BasicEventOut interface {
	SetGroupMsgEvent(func(message.Message))
	SetFriendMsgEvent(func(message.Message))
}

type BasicEventInp interface {
	SendGroupMsg(groupId uint, chain msgchain.Chain) (success bool, msgId uint)
	SendGroupAIVoiceMsg(groupId uint, character, msg string) (success bool, msgId uint)
	SendFriendMsg(userId uint, chain msgchain.Chain) (success bool, msgId uint)
	SendPokeMsg(userId uint, groupId *uint)
}
