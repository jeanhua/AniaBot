package bot

import "github.com/jeanhua/AniaBot/common/msgchain"

type Bot interface {
	SendGroupMsg(groupId uint, chain msgchain.Chain) (success bool, msgId uint)
	SendFriendMsg(userId uint, chain msgchain.Chain) (success bool, msgId uint)
}
