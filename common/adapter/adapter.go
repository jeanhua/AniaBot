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
	SendGroupMsg(groupId uint, chain msgchain.Chain)
	SendFriendMsg(friend uint, chain msgchain.Chain)
}
