package pluginaichat

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// ---- clockSubagentSet ----

func TestClockSubagentSetMarkFinishedAndCollect(t *testing.T) {
	set := newClockSubagentSet()
	done1 := make(chan struct{})
	done2 := make(chan struct{})
	set.add(&clockSubagent{id: "1", task: "任务一", startTime: time.Now(), done: done1})
	set.add(&clockSubagent{id: "2", task: "任务二", startTime: time.Now().Add(time.Millisecond), done: done2})

	if !set.hasPending() || set.pendingCount() != 2 {
		t.Fatalf("初始应有 2 个运行中子代理, pending=%d", set.pendingCount())
	}

	set.markFinished("1", "结果一", nil)
	if set.pendingCount() != 1 {
		t.Fatalf("完成后应剩 1 个运行中子代理, pending=%d", set.pendingCount())
	}

	// 重复 markFinished 不应 panic（close 已关闭的 channel）
	set.markFinished("1", "重复", nil)

	finished := set.collectFinished()
	if len(finished) != 1 || finished[0].id != "1" || finished[0].result != "结果一" {
		t.Fatalf("collectFinished 应只取出已完成的子代理, got %+v", finished)
	}
	// 已取出的不再重复返回
	if again := set.collectFinished(); len(again) != 0 {
		t.Fatalf("已完成子代理不应重复收集, got %d 条", len(again))
	}

	set.markFinished("2", "", errors.New("执行失败"))
	finished = set.collectFinished()
	if len(finished) != 1 || finished[0].err == nil {
		t.Fatalf("失败结果应携带 err, got %+v", finished)
	}
	if set.hasPending() {
		t.Fatalf("全部完成后不应有运行中子代理")
	}
}

func TestClockSubagentSetWaitAll(t *testing.T) {
	set := newClockSubagentSet()
	done := make(chan struct{})
	set.add(&clockSubagent{id: "1", task: "任务", startTime: time.Now(), done: done})

	// 50ms 后完成，waitAll 应解除阻塞
	go func() {
		time.Sleep(50 * time.Millisecond)
		set.markFinished("1", "ok", nil)
	}()
	start := time.Now()
	set.waitAll(context.Background(), clockSubagentWaitReserve)
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("waitAll 应在子代理完成后及时返回, 耗时 %v", elapsed)
	}
	if set.hasPending() {
		t.Fatalf("完成后不应有运行中子代理")
	}
}

func TestClockSubagentSetWaitAllBudgetExhausted(t *testing.T) {
	set := newClockSubagentSet()
	set.add(&clockSubagent{id: "1", task: "任务", startTime: time.Now(), done: make(chan struct{})})

	// 父上下文 deadline 已不足 reserve：应立即返回，不再等待
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	start := time.Now()
	set.waitAll(ctx, clockSubagentWaitReserve)
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("预算不足时 waitAll 应立即返回, 耗时 %v", elapsed)
	}
	if !set.hasPending() {
		t.Fatalf("未完成的子代理应仍处于运行中状态")
	}
}

func TestClockSubagentSetCancel(t *testing.T) {
	set := newClockSubagentSet()
	ctx1, cancel1 := context.WithCancel(context.Background())
	defer cancel1()
	set.add(&clockSubagent{id: "1", task: "一", startTime: time.Now(), cancel: cancel1, done: make(chan struct{})})

	if !set.cancelOne("1") {
		t.Fatalf("cancelOne 应命中运行中的子代理")
	}
	if ctx1.Err() == nil {
		t.Fatalf("cancelOne 应取消子代理 ctx")
	}
	if set.cancelOne("1") {
		// 子代理被取消但未 markFinished 前仍视为运行中，重复取消同样命中（幂等 cancel）
		t.Log("重复 cancelOne 命中运行中子代理，符合预期")
	}
	if set.cancelOne("999") {
		t.Fatalf("cancelOne 不存在的 ID 应返回 false")
	}

	// cancelPending 取消全部
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	set.add(&clockSubagent{id: "2", task: "二", startTime: time.Now(), cancel: cancel2, done: make(chan struct{})})
	set.cancelPending()
	if ctx2.Err() == nil {
		t.Fatalf("cancelPending 应取消所有运行中子代理")
	}
}

// ---- buildClockSubagentReport ----

func TestBuildClockSubagentReport(t *testing.T) {
	finished := []*clockSubagent{
		{id: "1", task: "调研 A", result: "【子代理执行完成】结果 A"},
		{id: "2", task: "调研 B", err: errors.New("子代理执行超时（10s），已中止")},
	}
	report := buildClockSubagentReport(finished, 0)
	for _, want := range []string{"【子代理执行结果汇总】", "ID: 1", "调研 A", "结果 A", "ID: 2", "执行失败", "已中止"} {
		if !strings.Contains(report, want) {
			t.Fatalf("报告应包含 %q, got:\n%s", want, report)
		}
	}
	if strings.Contains(report, "仍在执行") {
		t.Fatalf("pending=0 时不应提示仍有子代理执行中, got:\n%s", report)
	}

	report = buildClockSubagentReport(finished, 2)
	if !strings.Contains(report, "2 个子代理仍在执行") {
		t.Fatalf("pending>0 时应提示剩余数量, got:\n%s", report)
	}
}

// ---- newClockSubagentTools ----

func TestNewClockSubagentTools(t *testing.T) {
	p := &AIChatPlugin{}
	task := &ClockTask{ID: "t1", TargetType: clockTargetGroup, TargetID: "12345"}
	tools := newClockSubagentTools(p, nil, task, newClockSubagentSet(), &usageAcc{})
	if len(tools) != 3 {
		t.Fatalf("应创建 3 个子代理工具, got %d", len(tools))
	}
	names := map[string]bool{}
	for _, tool := range tools {
		names[tool.Name()] = true
	}
	for _, want := range []string{"subagent_run", "subagent_list", "subagent_cancel"} {
		if !names[want] {
			t.Fatalf("缺少工具 %s", want)
		}
	}
}
