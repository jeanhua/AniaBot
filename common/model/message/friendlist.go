package message

type Friend struct {
	UserID   QID    `json:"user_id"`
	Nickname string `json:"nickname"`
	Remark   string `json:"remark"`
}
