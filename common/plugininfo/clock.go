package plugininfo

import "time"

// ClockTaskInfo 插件定时任务的面板展示信息。
//
// 与插件内部的任务结构解耦：插件（如 AI 对话插件的 clock 功能）把内部任务
// 转换为该结构供 core / Web 控制面板消费，避免面板依赖具体插件类型。
type ClockTaskInfo struct {
	ID         string    `json:"id"`
	Title      string    `json:"title"`
	Content    string    `json:"content"` // 任务内容，触发时作为对话内容发送给 AI
	Note       string    `json:"note"`    // 备注（可为空）
	Cron       string    `json:"cron"`
	TargetType string    `json:"target_type"` // group / friend
	TargetID   string    `json:"target_id"`
	Enabled    bool      `json:"enabled"`
	RunOnce    bool      `json:"run_once"`
	TimeoutSec int       `json:"timeout_sec"` // 单次执行超时秒数，0 表示用默认值
	CreatedBy  string    `json:"created_by"`  // 创建者 ID，空表示无（QQ 为数字，其他平台带前缀）
	CreatedAt  time.Time `json:"created_at"`
	LastRunAt  time.Time `json:"last_run_at,omitempty"`
	NextRunAt  time.Time `json:"next_run_at,omitempty"`
}

// ClockTaskUpdate 定时任务可编辑字段，指针类型表示「仅当提供时才更新」。
type ClockTaskUpdate struct {
	Cron       *string `json:"cron"`
	Title      *string `json:"title"`
	Content    *string `json:"content"`
	Note       *string `json:"note"`
	TimeoutSec *int    `json:"timeout_sec"`
	Enabled    *bool   `json:"enabled"`
	TargetType *string `json:"target_type"` // group / friend
	TargetID   *string `json:"target_id"`   // 目标会话 ID（QQ 为数字，其他平台带前缀）
	RunOnce    *bool   `json:"run_once"`
}

// ClockTaskCreate 新建定时任务的参数。
type ClockTaskCreate struct {
	Cron       string `json:"cron"`
	Title      string `json:"title"`
	Content    string `json:"content"`
	TargetType string `json:"target_type"` // group / friend
	TargetID   string `json:"target_id"`   // 目标会话 ID（QQ 为数字，其他平台带前缀，如 fs:oc_xxx）
	Enabled    bool   `json:"enabled"`
	RunOnce    bool   `json:"run_once"`
	TimeoutSec int    `json:"timeout_sec"`
	Note       string `json:"note"`
}
