package pluginaichat

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/jeanhua/AniaBot/bot/component/aichat"
	"github.com/jeanhua/AniaBot/common/storage"
)

func newTestQuotaManager(store storage.PersistentStorage, daily, global int) *quotaManager {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return newQuotaManager(store, logger, daily, global)
}

func usage(total, prompt, completion int) aichat.TokenUsage {
	return aichat.TokenUsage{TotalTokens: total, PromptTokens: prompt, CompletionTokens: completion}
}

// TestQuotaCheckAndAdd 会话维度与全局维度独立生效，超限后 Check 返回拒绝与提示。
func TestQuotaCheckAndAdd(t *testing.T) {
	q := newTestQuotaManager(newPFake(), 100, 1000)

	if _, denied := q.Check("g:1"); denied {
		t.Fatal("空计数不应拒绝")
	}

	q.Add("g:1", usage(60, 0, 0))
	if reason, denied := q.Check("g:1"); denied {
		t.Fatalf("60/100 不应拒绝: %s", reason)
	}
	if _, denied := q.Check("g:2"); denied {
		t.Fatal("其它会话不应被会话维度拒绝")
	}

	q.Add("g:1", usage(40, 0, 0))
	if reason, denied := q.Check("g:1"); !denied || !strings.Contains(reason, "每会话") {
		t.Fatalf("100/100 应拒绝并提示会话维度, denied=%v reason=%q", denied, reason)
	}

	// 会话 g:1 已达 100，全局仍充足：其它会话照常
	if _, denied := q.Check("g:2"); denied {
		t.Fatal("全局未超限时其它会话不应被拒")
	}
	// g:2 消耗 900 后全局达到 1000
	q.Add("g:2", usage(900, 0, 0))
	if reason, denied := q.Check("g:3"); !denied || !strings.Contains(reason, "全局") {
		t.Fatalf("全局 1000/1000 应拒绝并提示全局维度, denied=%v reason=%q", denied, reason)
	}
}

// TestQuotaDateRollover 日期滚动后计数归零（新的一天从零开始）。
func TestQuotaDateRollover(t *testing.T) {
	q := newTestQuotaManager(newPFake(), 100, 0)
	day1 := time.Date(2026, 8, 1, 12, 0, 0, 0, time.Local)
	q.now = func() time.Time { return day1 }

	q.Add("g:1", usage(80, 0, 0))
	if _, denied := q.Check("g:1"); denied {
		t.Fatal("80/100 不应拒绝")
	}

	// 第二天：计数从零开始
	q.now = func() time.Time { return day1.AddDate(0, 0, 1) }
	if _, denied := q.Check("g:1"); denied {
		t.Fatal("新的一天计数应归零")
	}
	q.Add("g:1", usage(100, 0, 0))
	if _, denied := q.Check("g:1"); !denied {
		t.Fatal("第二天 100/100 应拒绝")
	}
}

// TestQuotaTotalTokensFallback 上游未返回 total_tokens 时用 Prompt+Completion 兜底。
func TestQuotaTotalTokensFallback(t *testing.T) {
	q := newTestQuotaManager(newPFake(), 100, 0)

	// Total=0，兜底 30+20=50
	q.Add("g:1", usage(0, 30, 20))
	if _, denied := q.Check("g:1"); denied {
		t.Fatal("50/100 不应拒绝")
	}
	q.Add("g:1", usage(0, 30, 20))
	if _, denied := q.Check("g:1"); !denied {
		t.Fatal("兜底累计 100/100 应拒绝")
	}

	// 全部为 0 不记录
	q2 := newTestQuotaManager(newPFake(), 100, 0)
	q2.Add("g:1", usage(0, 0, 0))
	keys, _ := q2.store.Keys(context.Background(), "daily:")
	if len(keys) != 0 {
		t.Fatalf("全 0 用量不应产生计数, got %v", keys)
	}
}

// TestQuotaReset 清零单会话与全部。
func TestQuotaReset(t *testing.T) {
	q := newTestQuotaManager(newPFake(), 100, 0)

	q.Add("g:1", usage(50, 0, 0))
	q.Add("g:2", usage(60, 0, 0))

	if !q.Reset("g:1") {
		t.Fatal("Reset 单会话失败")
	}
	if _, denied := q.Check("g:1"); denied {
		t.Fatal("清零后 g:1 不应拒绝")
	}
	if _, denied := q.Check("g:2"); denied {
		t.Fatal("g:2 未清零仍 60/100，不应拒绝")
	}

	q.Add("g:1", usage(100, 0, 0))
	if !q.Reset("all") {
		t.Fatal("Reset 全部失败")
	}
	for _, k := range []string{"g:1", "g:2"} {
		if _, denied := q.Check(k); denied {
			t.Fatalf("全部清零后 %s 不应拒绝", k)
		}
	}
}

// TestQuotaPruneOld 惰性清理仅保留最近 keepDays 天的计数键。
func TestQuotaPruneOld(t *testing.T) {
	q := newTestQuotaManager(newPFake(), 100, 100)
	now := time.Date(2026, 8, 1, 12, 0, 0, 0, time.Local)
	q.now = func() time.Time { return now }

	// 直接落盘不同日期的计数（模拟历史数据）
	oldKey := "daily:2026-07-25:g:9"
	q.store.Set(context.Background(), oldKey, int64(50))
	todayKey := "daily:2026-08-01:g:1"
	q.store.Set(context.Background(), todayKey, int64(50))
	recentKey := "daily:2026-07-31:g:2" // 3 天前（保留）
	q.store.Set(context.Background(), recentKey, int64(50))

	q.pruneOld(3)

	if q.store.Has(context.Background(), oldKey) {
		t.Fatal("5 天前的键应被清理")
	}
	if !q.store.Has(context.Background(), recentKey) {
		t.Fatal("3 天内的键应保留")
	}
	if !q.store.Has(context.Background(), todayKey) {
		t.Fatal("当日键应保留")
	}
}

// TestQuotaDisabled 双维度均不限制时 Check 恒不拒绝，Add 不产生计数。
func TestQuotaDisabled(t *testing.T) {
	q := newTestQuotaManager(newPFake(), 0, 0)

	q.Add("g:1", usage(100, 0, 0))
	if _, denied := q.Check("g:1"); denied {
		t.Fatal("未配置限制不应拒绝")
	}
	keys, _ := q.store.Keys(context.Background(), "daily:")
	if len(keys) != 0 {
		t.Fatalf("未配置限制不应产生计数, got %v", keys)
	}
}

// TestQuotaSummary 汇总包含全局与各会话（按用量降序），字段完整。
func TestQuotaSummary(t *testing.T) {
	q := newTestQuotaManager(newPFake(), 100, 500)

	q.Add("g:1", usage(30, 0, 0))
	q.Add("f:2", usage(60, 0, 0))

	info, err := q.Summary()
	if err != nil {
		t.Fatalf("Summary 失败: %v", err)
	}
	if info.GlobalUsed != 90 || info.GlobalLimit != 500 || info.GlobalReached {
		t.Fatalf("全局汇总不符: %+v", info)
	}
	if len(info.Sessions) != 2 {
		t.Fatalf("应有 2 个会话, got %d", len(info.Sessions))
	}
	// 降序：f:2（60）在前
	if info.Sessions[0].Key != "f:2" || info.Sessions[1].Key != "g:1" {
		t.Fatalf("会话排序不符: %+v", info.Sessions)
	}
	if info.Sessions[0].Kind != "friend" || info.Sessions[0].Target != "2" {
		t.Fatalf("会话类型解析不符: %+v", info.Sessions[0])
	}
	if info.Sessions[0].Remaining != 40 || info.Sessions[1].Remaining != 70 {
		t.Fatalf("剩余额度不符: %+v", info.Sessions)
	}
	if info.Sessions[1].Reached {
		t.Fatal("30/100 不应 reached")
	}

	// 清零后汇总归零
	if !q.Reset("all") {
		t.Fatal("Reset 失败")
	}
	info, _ = q.Summary()
	if info.GlobalUsed != 0 || len(info.Sessions) != 0 {
		t.Fatalf("清零后汇总应归零: %+v", info)
	}
}

// TestQuotaResetValidation 面板接口对非法 scope 报错。
func TestQuotaResetValidation(t *testing.T) {
	p := &AIChatPlugin{quotaManager: newTestQuotaManager(newPFake(), 100, 0)}
	if err := p.QuotaReset("bad-scope"); err == nil {
		t.Fatal("非法 scope 应报错")
	}
	if err := p.QuotaReset("g:123"); err != nil {
		t.Fatalf("合法 scope 不应报错: %v", err)
	}
	if err := p.QuotaReset("all"); err != nil {
		t.Fatalf("all 不应报错: %v", err)
	}

	// 未启用时返回错误
	p2 := &AIChatPlugin{}
	if err := p2.QuotaReset("g:123"); err == nil {
		t.Fatal("未启用配额时清零应报错")
	}
	if _, err := p2.QuotaSummary(); err == nil {
		t.Fatal("未启用配额时汇总应报错")
	}
}
