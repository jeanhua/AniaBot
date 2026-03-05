package message

type GroupInfo struct {
	GroupID     QID    `json:"group_id"`
	GroupName   string `json:"group_name"`
	MemberCount int    `json:"member_count"`
	GroupRemark string `json:"group_remark"`
}
