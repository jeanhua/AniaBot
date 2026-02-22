package githubrepoer

const (
	TargetFriend = iota
	TargetGroup
)

type work struct {
	target       int
	userId       uint
	groupId      uint
	msgId        uint
	repoURL      string
	compress     bool
	include      string
	exclude      string
	delComment   bool
	delEmptyLine bool
}
