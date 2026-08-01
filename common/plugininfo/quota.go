package plugininfo

// QuotaSessionInfo 一个会话（群聊/私聊）当日的 Token 用量概览，供面板「配额管理」页展示。
type QuotaSessionInfo struct {
	Key       string `json:"key"`       // 会话 key：g:群号 / f:QQ号
	Kind      string `json:"kind"`      // group / friend
	Target    string `json:"target"`    // 群号或 QQ 号
	Used      int64  `json:"used"`      // 当日已用 token
	Limit     int64  `json:"limit"`     // 每会话每日上限（0 表示不限制）
	Remaining int64  `json:"remaining"` // 剩余额度（负值按 0 展示）
	Reached   bool   `json:"reached"`   // 是否已达上限
}

// QuotaSummaryInfo 当日配额汇总：全局总额 + 各会话明细。
type QuotaSummaryInfo struct {
	Date            string             `json:"date"`             // 汇总日期（本地时区）
	GlobalUsed      int64              `json:"global_used"`      // 全局当日已用 token
	GlobalLimit     int64              `json:"global_limit"`     // 全局每日上限（0 表示不限制）
	GlobalRemaining int64              `json:"global_remaining"` // 全局剩余额度（负值按 0 展示）
	GlobalReached   bool               `json:"global_reached"`   // 是否已达上限
	Sessions        []QuotaSessionInfo `json:"sessions"`         // 会话明细（按用量降序）
}
