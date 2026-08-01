package adminpanel

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jeanhua/AniaBot/common/plugininfo"
)

// fakeQuotaSource 测试用 QuotaSource 实现，记录清零请求。
type fakeQuotaSource struct {
	info     plugininfo.QuotaSummaryInfo
	err      error
	resetErr error
	resets   []string
}

func (f *fakeQuotaSource) QuotaSummary() (plugininfo.QuotaSummaryInfo, error) { return f.info, f.err }

func (f *fakeQuotaSource) QuotaReset(scope string) error {
	f.resets = append(f.resets, scope)
	return f.resetErr
}

// TestHandleQuota 配额汇总接口返回 JSON；未启用时 404。
func TestHandleQuota(t *testing.T) {
	s := &Server{opt: Options{Quota: &fakeQuotaSource{info: plugininfo.QuotaSummaryInfo{
		Date: "2026-08-01", GlobalUsed: 500, GlobalLimit: 1000, GlobalRemaining: 500, GlobalReached: false,
		Sessions: []plugininfo.QuotaSessionInfo{
			{Key: "g:1", Kind: "group", Target: "1", Used: 300, Limit: 500, Remaining: 200, Reached: false},
		},
	}}}}
	rec := httptest.NewRecorder()
	s.handleQuota(rec, httptest.NewRequest("GET", "/api/quota", nil))
	if rec.Code != 200 {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var got plugininfo.QuotaSummaryInfo
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, rec.Body.String())
	}
	if got.Date != "2026-08-01" || got.GlobalUsed != 500 || got.GlobalLimit != 1000 || got.GlobalReached {
		t.Fatalf("全局字段不符: %+v", got)
	}
	if len(got.Sessions) != 1 || got.Sessions[0].Key != "g:1" || got.Sessions[0].Remaining != 200 {
		t.Fatalf("会话字段不符: %+v", got.Sessions)
	}

	// 未启用：404
	s2 := &Server{opt: Options{}}
	rec2 := httptest.NewRecorder()
	s2.handleQuota(rec2, httptest.NewRequest("GET", "/api/quota", nil))
	if rec2.Code != 404 {
		t.Fatalf("未启用时 status = %d, want 404", rec2.Code)
	}
}

// TestHandleQuotaReset 清零接口：合法 scope 通过、空 scope 400、未启用 404。
func TestHandleQuotaReset(t *testing.T) {
	src := &fakeQuotaSource{}
	s := &Server{opt: Options{Quota: src}}

	// 合法 scope
	rec := httptest.NewRecorder()
	s.handleQuotaReset(rec, httptest.NewRequest("POST", "/api/quota/reset", strings.NewReader(`{"scope":"g:123"}`)))
	if rec.Code != 200 {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if len(src.resets) != 1 || src.resets[0] != "g:123" {
		t.Fatalf("resets = %v, want [g:123]", src.resets)
	}
	var okBody map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &okBody)
	if okBody["ok"] != true {
		t.Fatalf("响应缺少 ok=true: %s", rec.Body.String())
	}

	// 空 scope：400
	rec2 := httptest.NewRecorder()
	s.handleQuotaReset(rec2, httptest.NewRequest("POST", "/api/quota/reset", strings.NewReader(`{"scope":""}`)))
	if rec2.Code != 400 {
		t.Fatalf("空 scope status = %d, want 400", rec2.Code)
	}

	// 未启用：404
	s2 := &Server{opt: Options{}}
	rec3 := httptest.NewRecorder()
	s2.handleQuotaReset(rec3, httptest.NewRequest("POST", "/api/quota/reset", strings.NewReader(`{"scope":"all"}`)))
	if rec3.Code != 404 {
		t.Fatalf("未启用时 status = %d, want 404", rec3.Code)
	}
}
