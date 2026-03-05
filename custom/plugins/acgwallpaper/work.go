package acgwallpaper

import "github.com/jeanhua/AniaBot/common/model/message"

const (
	TargetFriend = iota
	TargetGroup
)

type work struct {
	target  uint
	userId  message.QID
	groupId message.QID
}
