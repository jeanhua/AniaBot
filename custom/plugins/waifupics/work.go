package waifupics

import "github.com/jeanhua/AniaBot/common/model/message"

const (
	TargetFriend = iota
	TargetGroup
)

type wTarget int

type work struct {
	category string
	target   wTarget
	userId   message.QID
	groupId  message.QID
}
