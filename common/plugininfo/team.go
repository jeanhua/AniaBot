package plugininfo

import "time"

// TeamScopeInfo 是一个会话（群聊/私聊）的 Agent 团队概览，供面板左侧列表展示。
type TeamScopeInfo struct {
	Scope  string `json:"scope"`  // 会话 scope，g:群号 / f:QQ号
	Kind   string `json:"kind"`   // group / friend
	Target string `json:"target"` // 群号或 QQ 号
	Count  int    `json:"count"`  // 该 scope 下的团队数
}

// TeamMemberInfo 是一个团队成员（带角色描述），供面板展示与编辑。
type TeamMemberInfo struct {
	Name string `json:"name"`
	Role string `json:"role,omitempty"` // 角色描述；空表示按普通子代理执行
}

// TeamInfo 是一个已保存的 Agent 团队，供面板展示。
type TeamInfo struct {
	Name      string           `json:"name"`
	Desc      string           `json:"desc,omitempty"`
	Members   []TeamMemberInfo `json:"members"`
	CreatedAt time.Time        `json:"created_at,omitempty"`
}

// TeamUpsert 是面板新增/编辑团队的请求体（团队名即持久化键，不可重命名）。
type TeamUpsert struct {
	Scope   string           `json:"scope"`
	Name    string           `json:"name"`
	Desc    string           `json:"desc,omitempty"`
	Members []TeamMemberInfo `json:"members"`
}
