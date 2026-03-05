package acgwallpaper

import "github.com/jeanhua/AniaBot/common/model/message"

const (
	TargetFriend wTarget = iota
	TargetGroup
)

type wTarget int

type work struct {
	target  wTarget
	userId  message.QID
	groupId message.QID
}
