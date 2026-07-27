package adminpanel

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jeanhua/AniaBot/bot/component/querylog"
	"github.com/jeanhua/AniaBot/bot/component/tasklog"
)

func TestTokenStatsDetail(t *testing.T) {
	now := time.Now()
	s := &Server{opt: Options{
		QueryLogs: func(f querylog.Filter) []querylog.Entry {
			return []querylog.Entry{
				{Time: now, ChatType: "group", TargetID: "100", Status: querylog.StatusSuccess, Iterations: 3, PromptTokens: 1000, CompletionTokens: 200, TotalTokens: 1200, CachedTokens: 400},
				{Time: now.Add(-26 * time.Hour), ChatType: "friend", TargetID: "200", Status: querylog.StatusError, Iterations: 1, PromptTokens: 500, CompletionTokens: 100, TotalTokens: 600},
			}
		},
		TaskLogs: func(f tasklog.Filter) []tasklog.Entry {
			return []tasklog.Entry{
				{TriggerTime: now, TargetType: "group", TargetID: "100", Status: tasklog.StatusTimeout, Iterations: 2, PromptTokens: 800, CompletionTokens: 300, TotalTokens: 1100},
			}
		},
	}}
	rec := httptest.NewRecorder()
	s.handleTokenStatsDetail(rec, httptest.NewRequest("GET", "/api/tokenstats/detail", nil))
	if rec.Code != 200 {
		t.Fatalf("status = %d", rec.Code)
	}
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, rec.Body.String())
	}
	for _, k := range []string{"summary", "today", "by_source", "by_chat_type", "by_status", "top_targets", "hourly", "daily", "avg_iterations"} {
		if _, ok := got[k]; !ok {
			t.Fatalf("missing key %q", k)
		}
	}
	summary := got["summary"].(map[string]any)
	if summary["requests"].(float64) != 3 || summary["total_tokens"].(float64) != 2900 {
		t.Fatalf("summary = %v", summary)
	}
	daily := got["daily"].([]any)
	if len(daily) != 30 {
		t.Fatalf("daily len = %d", len(daily))
	}
	hourly := got["hourly"].([]any)
	if len(hourly) != 24 {
		t.Fatalf("hourly len = %d", len(hourly))
	}
	top := got["top_targets"].([]any)
	if len(top) != 2 {
		t.Fatalf("top_targets len = %d", len(top))
	}
	first := top[0].(map[string]any)
	if first["target_id"] != "100" || first["total_tokens"].(float64) != 2300 {
		t.Fatalf("top[0] = %v", first)
	}
}
