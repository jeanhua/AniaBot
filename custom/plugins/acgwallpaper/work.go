package acgwallpaper

const (
	TargetFriend = iota
	TargetGroup
)

type work struct {
	target  uint
	userId  uint
	groupId uint
}
