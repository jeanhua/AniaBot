package agenthook

import (
	"context"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// 标准 os/exec 测试技巧：假命令 = 以 -test.run=TestHelperProcess 重新运行
// 测试二进制，由环境变量与 "--" 后的脚本参数决定行为（输出/退出码/睡眠）。

// fakeExecCommand 返回替换 execCommandContext 的工厂：忽略真实命令，
// 按 script 运行 helper 进程。
func fakeExecCommand() func(context.Context, string, ...string) *exec.Cmd {
	return func(ctx context.Context, name string, args ...string) *exec.Cmd {
		script := ""
		if len(args) > 0 {
			script = args[len(args)-1] // shell 包装参数之后即命令文本
		}
		cs := []string{"-test.run=TestHelperProcess", "--", script}
		cmd := exec.CommandContext(ctx, os.Args[0], cs...)
		cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
		return cmd
	}
}

// TestHelperProcess 不是真正的测试：作为子进程被 fakeExecCommand 启动，
// 按 "--" 后的脚本参数模拟钩子命令行为。
// 脚本：sleep（睡眠 5s，配超时测试）/ echo-stdin（stdin 原样回写 stdout）/
// exit<N>[:stdout=<文本>][:stderr=<文本>]（输出后以退出码 N 退出）。
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	var script string
	for i, a := range os.Args {
		if a == "--" && i+1 < len(os.Args) {
			script = os.Args[i+1]
			break
		}
	}
	switch script {
	case "sleep":
		time.Sleep(5 * time.Second)
		os.Exit(0)
	case "echo-stdin":
		_, _ = io.Copy(os.Stdout, os.Stdin)
		os.Exit(0)
	}
	code := 0
	for _, part := range strings.Split(script, ":") {
		switch {
		case strings.HasPrefix(part, "exit"):
			code = atoiHelper(strings.TrimPrefix(part, "exit"))
		case strings.HasPrefix(part, "stdout="):
			_, _ = io.WriteString(os.Stdout, strings.TrimPrefix(part, "stdout="))
		case strings.HasPrefix(part, "stderr="):
			_, _ = io.WriteString(os.Stderr, strings.TrimPrefix(part, "stderr="))
		}
	}
	os.Exit(code)
}

// atoiHelper 简易数字解析（避免 helper 中引入 strconv 的导入噪音；失败返回 0）
func atoiHelper(s string) int {
	v := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		v = v*10 + int(c-'0')
	}
	return v
}

func withFakeExec(t *testing.T) {
	t.Helper()
	orig := execCommandContext
	execCommandContext = fakeExecCommand()
	t.Cleanup(func() { execCommandContext = orig })
}

func runOne(t *testing.T, script string, timeoutSec int, p Payload) Result {
	t.Helper()
	r := newShellRunner()
	hooks, err := compileHooks(&FileConfig{Hooks: map[Event][]ShellHookSpec{
		EventPreToolUse: {{Command: script, TimeoutSec: timeoutSec}},
	}})
	if err != nil {
		t.Fatalf("compileHooks: %v", err)
	}
	return r.run(context.Background(), hooks[EventPreToolUse][0], p)
}

func TestRunShellHookExitSemantics(t *testing.T) {
	withFakeExec(t)
	p := Payload{Event: EventPreToolUse, SessionKey: "g:1", ToolName: "bash", ToolInput: `{"command":"ls"}`}

	t.Run("退出码0 stdout进Context", func(t *testing.T) {
		res := runOne(t, "exit0:stdout=额外上下文", 0, p)
		if res.Block || res.Err != nil {
			t.Fatalf("应通过, got %+v", res)
		}
		if res.Context != "额外上下文" {
			t.Fatalf("Context = %q", res.Context)
		}
	})
	t.Run("退出码2 stderr进Reason并阻断", func(t *testing.T) {
		res := runOne(t, "exit2:stderr=危险命令", 0, p)
		if !res.Block || res.Reason != "危险命令" {
			t.Fatalf("应阻断且带原因, got %+v", res)
		}
	})
	t.Run("退出码2无输出时兜底原因", func(t *testing.T) {
		res := runOne(t, "exit2", 0, p)
		if !res.Block || res.Reason == "" {
			t.Fatalf("应阻断且有兜底原因, got %+v", res)
		}
	})
	t.Run("退出码1为非阻断错误", func(t *testing.T) {
		res := runOne(t, "exit1:stderr=坏了", 0, p)
		if res.Block || res.Err == nil {
			t.Fatalf("应为非阻断错误, got %+v", res)
		}
	})
	t.Run("超时为非阻断错误", func(t *testing.T) {
		res := runOne(t, "sleep", 1, p)
		if res.Block || res.Err == nil || !strings.Contains(res.Err.Error(), "超时") {
			t.Fatalf("应为超时错误, got %+v", res)
		}
	})
	t.Run("stdin收到PayloadJSON", func(t *testing.T) {
		// helper 把 stdin 原样写回 stdout：退出码 0 时进入 Context
		res := runOne(t, "echo-stdin", 0, Payload{Event: EventPreToolUse, SessionKey: "g:9", AgentKind: AgentKindMain, ToolName: "bash", ToolInput: "{}"})
		if !strings.Contains(res.Context, `"tool_name":"bash"`) || !strings.Contains(res.Context, `"session_id":"g:9"`) {
			t.Fatalf("stdin 载荷未正确传递, Context = %q", res.Context)
		}
	})
}
