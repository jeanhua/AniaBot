package waifupics

import "github.com/jeanhua/AniaBot/common/model/message"

const (
	TargetFriend = iota
	TargetGroup
)

type work struct {
	category string
	target   int
	userId   message.QID
	groupId  message.QID
}
