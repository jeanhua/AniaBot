package plugininfo

import "time"

// MemoryScopeInfo 是一个会话（群聊/私聊）的记忆概览，供面板左侧列表展示。
type MemoryScopeInfo struct {
	Scope  string `json:"scope"`  // 会话 scope，g:群号 / f:QQ号
	Kind   string `json:"kind"`   // group / friend
	Target string `json:"target"` // 群号或 QQ 号
	Count  int    `json:"count"`  // 该 scope 下的记忆条数
}

// MemoryEntryInfo 是一条长期记忆，供面板展示。
type MemoryEntryInfo struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id,omitempty"` // 关联的群成员 QQ 号；空表示属于整个会话
	Content   string    `json:"content"`
	Tags      []string  `json:"tags,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// MemoryEntryUpsert 是面板新增/编辑记忆的请求体（新增时 ID 为空）。
type MemoryEntryUpsert struct {
	Scope   string   `json:"scope"`
	ID      string   `json:"id,omitempty"`
	UserID  string   `json:"user_id,omitempty"`
	Content string   `json:"content"`
	Tags    []string `json:"tags,omitempty"`
}
