package githubrepoer

import "github.com/jeanhua/AniaBot/common/model/message"

const (
	TargetFriend = iota
	TargetGroup
)

type work struct {
	target       int
	userId       message.QID
	groupId      message.QID
	msgId        message.QID
	repoURL      string
	compress     bool
	include      string
	exclude      string
	delComment   bool
	delEmptyLine bool
}
