package bot

import "github.com/jeanhua/AniaBot/common/msgchain"

type Bot interface {
	SendGroupMsg(groupId uint, chain msgchain.Chain)
	SendFriendMsg(friendId uint, chain msgchain.Chain)
}
