package message

type Friend struct {
	UserID   uint   `json:"user_id"`
	Nickname string `json:"nickname"`
	Remark   string `json:"remark"`
}
