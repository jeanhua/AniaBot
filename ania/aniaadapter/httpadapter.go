package aniaadapter

import (
	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/jeanhua/AniaBot/common/msgchain"
	"github.com/spf13/viper"
)

type napcatHttpAdapter struct{}

func (n *napcatHttpAdapter) Serve(v *viper.Viper) {
	//TODO implement me
	panic("implement me")
}

func (n *napcatHttpAdapter) SetGroupMsgEvent(f func(message.Message)) {
	//TODO implement me
	panic("implement me")
}

func (n *napcatHttpAdapter) SetFriendMsgEvent(f func(message.Message)) {
	//TODO implement me
	panic("implement me")
}

func (n *napcatHttpAdapter) SendGroupMsg(groupId uint, chain msgchain.Chain) {
	//TODO implement me
	panic("implement me")
}

func (n *napcatHttpAdapter) SendFriendMsg(friendId uint, chain msgchain.Chain) {
	//TODO implement me
	panic("implement me")
}
