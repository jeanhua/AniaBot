package adminpanel

import (
	"net/http"
	"time"

	"github.com/jeanhua/AniaBot/bot/component/querylog"
	"github.com/jeanhua/AniaBot/bot/component/tasklog"
)

// token 用量聚合：总量 / 今日 / 每日序列共用同一结构。
type tokenStatAcc struct {
	Requests         int     `json:"requests"`
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	TotalTokens      int     `json:"total_tokens"`
	CachedTokens     int     `json:"cached_tokens"`
	CacheHitRate     float64 `json:"cache_hit_rate"` // cached / prompt，prompt 为 0 时为 0
}

func (a *tokenStatAcc) add(prompt, completion, total, cached int) {
	a.Requests++
	a.PromptTokens += prompt
	a.CompletionTokens += completion
	a.TotalTokens += total
	a.CachedTokens += cached
}

func (a *tokenStatAcc) finish() {
	if a.PromptTokens > 0 {
		// 上游语义中 prompt_tokens 包含命中缓存的部分（hit + miss），
		// 因此命中率 = cached / prompt
		a.CacheHitRate = float64(a.CachedTokens) / float64(a.PromptTokens)
	}
}

// tokenDayStat 单日用量（daily 序列元素）。
type tokenDayStat struct {
	tokenStatAcc
	Date string `json:"date"` // 2006-01-02（本地时区）
}

// tokenStatsDailyDays daily 序列覆盖的天数（含今天）
const tokenStatsDailyDays = 14

// handleTokenStats 返回 token 消耗监控指标：历史总量、今日用量与最近 14 天
// 每日用量（含缓存命中率）。数据源为 Query 日志与定时任务执行日志的留存记录
// （各受容量上限约束，非全量历史），缓存字段依赖上游 API 返回，不支持时为 0。
func (s *Server) handleTokenStats(w http.ResponseWriter, _ *http.Request) {
	now := time.Now()
	todayStr := now.Format("2006-01-02")

	// 预建最近 N 天的桶（含无数据的日期，前端直接渲染连续序列）
	daily := make([]tokenDayStat, 0, tokenStatsDailyDays)
	buckets := make(map[string]*tokenStatAcc, tokenStatsDailyDays)
	for i := tokenStatsDailyDays - 1; i >= 0; i-- {
		d := now.AddDate(0, 0, -i)
		ds := d.Format("2006-01-02")
		daily = append(daily, tokenDayStat{Date: ds})
		buckets[ds] = &daily[len(daily)-1].tokenStatAcc
	}

	var summary, today tokenStatAcc
	addEntry := func(t time.Time, prompt, completion, total, cached int) {
		summary.add(prompt, completion, total, cached)
		ds := t.Local().Format("2006-01-02")
		if ds == todayStr {
			today.add(prompt, completion, total, cached)
		}
		if b, ok := buckets[ds]; ok {
			b.add(prompt, completion, total, cached)
		}
	}

	if s.opt.QueryLogs != nil {
		for _, e := range s.opt.QueryLogs(querylog.Filter{Limit: 500}) {
			addEntry(e.Time, e.PromptTokens, e.CompletionTokens, e.TotalTokens, e.CachedTokens)
		}
	}
	if s.opt.TaskLogs != nil {
		for _, e := range s.opt.TaskLogs(tasklog.Filter{Limit: 500}) {
			addEntry(e.TriggerTime, e.PromptTokens, e.CompletionTokens, e.TotalTokens, e.CachedTokens)
		}
	}

	summary.finish()
	today.finish()
	for i := range daily {
		daily[i].finish()
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"summary": summary,
		"today":   today,
		"daily":   daily,
	})
}
