package waifupics

const (
	TargetFriend = iota
	TargetGroup
)

type work struct {
	category string
	target   uint
	userId   uint
	groupId  uint
}
