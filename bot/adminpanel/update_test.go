package adminpanel

import "testing"

func TestUpdateStateBlocksRestartWindow(t *testing.T) {
	var u updateState
	if !u.tryBegin() {
		t.Fatal("第一个更新任务应能启动")
	}
	if u.tryBegin() {
		t.Fatal("并发第二个更新任务应被拒绝")
	}

	u.finish()
	if u.running || u.phase != upPhaseDone || !u.restarting {
		t.Fatalf("finish 后状态错误: running=%v phase=%q restarting=%v", u.running, u.phase, u.restarting)
	}
	if u.tryBegin() {
		t.Fatal("重启窗口内不应允许再次启动更新")
	}

	snap := u.snapshot()
	if snap["running"] != false || snap["restarting"] != true {
		t.Fatalf("snapshot 状态错误: %+v", snap)
	}

	u.clearRestarting()
	if !u.tryBegin() {
		t.Fatal("重启失败后清除标记，应允许重试更新")
	}
}
