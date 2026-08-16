package functool

import (
	"context"
	"strings"
	"testing"

	"github.com/jeanhua/AniaBot/bot/component/llmtool"
)

func mustBash(t *testing.T, cfg BashConfig) *BashTool {
	t.Helper()
	tool, err := NewBashTool(cfg)
	if err != nil {
		t.Fatalf("NewBashTool: %v", err)
	}
	return tool
}

// TestBashCheckCommandThreeTier 三段式权限模型：黑名单→拒绝；白名单→放行；
// 都不命中→审批（含两份名单都为空的场景）。
func TestBashCheckCommandThreeTier(t *testing.T) {
	tool := mustBash(t, BashConfig{
		Whitelist: []string{`^echo `},
		Blacklist: []string{`rm -rf`, `shutdown`},
	})

	cases := []struct {
		cmd  string
		want CmdVerdict
	}{
		{"echo hello", CmdAllow},
		{"rm -rf /", CmdDeny},
		{"shutdown now", CmdDeny},
		{"ls -la", CmdAsk},       // 不在白名单 → 审批（旧语义为拒绝）
		{"cat file.txt", CmdAsk}, // 同上
	}
	for _, tc := range cases {
		got, err := tool.checkCommand(tc.cmd)
		if tc.want == CmdDeny {
			if got != CmdDeny || err == nil {
				t.Errorf("%q: 期望拒绝, got verdict=%v err=%v", tc.cmd, got, err)
			}
			continue
		}
		if err != nil {
			t.Errorf("%q: 非拒绝档不应返回 error, got %v", tc.cmd, err)
		}
		if got != tc.want {
			t.Errorf("%q: 期望 %v, got %v", tc.cmd, tc.want, got)
		}
	}

	// 黑名单优先于白名单
	tool2 := mustBash(t, BashConfig{Whitelist: []string{`.*`}, Blacklist: []string{`rm -rf`}})
	if v, _ := tool2.checkCommand("echo a && rm -rf /"); v != CmdDeny {
		t.Errorf("黑名单应优先于白名单, got %v", v)
	}

	// 无名单：全部走审批档（审批未启用时默认放行，只认黑名单）
	tool3 := mustBash(t, BashConfig{})
	if v, err := tool3.checkCommand("echo hi"); v != CmdAsk || err != nil {
		t.Errorf("无名单时应进入审批档, got verdict=%v err=%v", v, err)
	}
}

// TestBashAskWithoutApprovalChannel 审批档命令在无审批通道（RequestApproval=nil）
// 时默认放行（只认黑名单），命令正常执行。
func TestBashAskWithoutApprovalChannel(t *testing.T) {
	tool := mustBash(t, BashConfig{})
	out, err := tool.Execute(context.Background(), &BashParams{Command: "echo hi"}, llmtool.CallBackFuncs{})
	if err != nil || !strings.Contains(out, "hi") {
		t.Fatalf("审批未启用时应默认放行, got out=%q err=%v", out, err)
	}
}

// TestBashAskApprovalFlow 审批档命令经 RequestApproval 批准才执行，拒绝则返回原因。
func TestBashAskApprovalFlow(t *testing.T) {
	tool := mustBash(t, BashConfig{})

	var gotTool, gotSummary string
	approve := llmtool.CallBackFuncs{RequestApproval: func(_ context.Context, toolName, summary string) (bool, string) {
		gotTool, gotSummary = toolName, summary
		return true, ""
	}}
	out, err := tool.Execute(context.Background(), &BashParams{Command: "echo bash-approval-ok"}, approve)
	if err != nil {
		t.Fatalf("批准后应执行成功: %v", err)
	}
	if !strings.Contains(out, "bash-approval-ok") {
		t.Fatalf("输出不符: %q", out)
	}
	if gotTool != "bash" || !strings.Contains(gotSummary, "echo bash-approval-ok") {
		t.Fatalf("审批参数不符: tool=%q summary=%q", gotTool, gotSummary)
	}

	deny := llmtool.CallBackFuncs{RequestApproval: func(context.Context, string, string) (bool, string) {
		return false, "用户拒绝了本次操作"
	}}
	if _, err := tool.Execute(context.Background(), &BashParams{Command: "echo never-runs"}, deny); err == nil || !strings.Contains(err.Error(), "用户拒绝了本次操作") {
		t.Fatalf("拒绝后应返回原因, got %v", err)
	}
}

// TestBashWhitelistSkipsApproval 白名单命令直接执行，不触发审批。
func TestBashWhitelistSkipsApproval(t *testing.T) {
	tool := mustBash(t, BashConfig{Whitelist: []string{`^echo `}})
	called := false
	cbs := llmtool.CallBackFuncs{RequestApproval: func(context.Context, string, string) (bool, string) {
		called = true
		return false, "不应被调用"
	}}
	out, err := tool.Execute(context.Background(), &BashParams{Command: "echo whitelist-ok"}, cbs)
	if err != nil || !strings.Contains(out, "whitelist-ok") {
		t.Fatalf("白名单命令应直接执行: out=%q err=%v", out, err)
	}
	if called {
		t.Fatal("白名单命令不应触发审批")
	}
}
