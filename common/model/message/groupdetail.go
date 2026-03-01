package message

type GroupInfo struct {
	GroupID     uint   `json:"group_id"`
	GroupName   string `json:"group_name"`
	MemberCount uint   `json:"member_count"`
	GroupRemark string `json:"group_remark"`
}
