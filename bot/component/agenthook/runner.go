package agenthook

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jeanhua/AniaBot/bot/component/functool"
)

const (
	// DefaultHookTimeout 单个 shell 钩子的默认超时
	DefaultHookTimeout = 10
	// MaxHookTimeout 单个 shell 钩子的超时上限（秒）：钩子是管理员配置的运维手段，
	// 但挂在高频事件（如 PreToolUse）上的慢钩子会成倍放大每轮延迟
	MaxHookTimeout = 60
	// maxOutputRunes 钩子 stdout/stderr 回填（Context/Reason）的截断长度
	maxOutputRunes = 2000
)

// execCommandContext 包级变量便于测试替换（exec 测试惯例）
var execCommandContext = exec.CommandContext

// shellRunner shell 钩子执行器：无状态，并发安全
type shellRunner struct {
	shell    string
	shellArg string
}

func newShellRunner() *shellRunner {
	// 钩子是管理员配置（可信），不走 bash 工具的白/黑名单，环境变量默认继承进程
	shell, arg := functool.ResolveShell("")
	return &shellRunner{shell: shell, shellArg: arg}
}

// run 执行单个 shell 钩子：stdin 写入 Payload JSON。
// 退出码语义对齐 Claude Code：0=通过（stdout→Result.Context，按 rune 截断）；
// 2=阻断（stderr 优先、stdout 兜底→Reason）；其他=非阻断错误（Result.Err）。
// 超时视为非阻断错误，不阻断主流程。
func (r *shellRunner) run(ctx context.Context, hook compiledHook, p Payload) Result {
	timeoutSec := hook.spec.TimeoutSec
	if timeoutSec <= 0 {
		timeoutSec = DefaultHookTimeout
	}
	if timeoutSec > MaxHookTimeout {
		timeoutSec = MaxHookTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
	defer cancel()

	stdin, err := json.Marshal(p)
	if err != nil {
		return Result{Err: fmt.Errorf("序列化钩子载荷失败: %w", err)}
	}
	cmd := execCommandContext(ctx, r.shell, r.shellArg, hook.spec.Command)
	cmd.Stdin = bytes.NewReader(stdin)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return Result{Err: fmt.Errorf("钩子执行超时（%d 秒）", timeoutSec)}
	}
	if runErr == nil {
		return Result{Context: truncateRunes(strings.TrimSpace(stdout.String()), maxOutputRunes)}
	}
	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) {
		if exitErr.ExitCode() == 2 {
			reason := truncateRunes(strings.TrimSpace(stderr.String()), maxOutputRunes)
			if reason == "" {
				reason = truncateRunes(strings.TrimSpace(stdout.String()), maxOutputRunes)
			}
			if reason == "" {
				reason = "钩子要求阻断"
			}
			return Result{Block: true, Reason: reason}
		}
		return Result{Err: fmt.Errorf("钩子退出码 %d: %s", exitErr.ExitCode(),
			truncateRunes(strings.TrimSpace(stderr.String()), 200))}
	}
	return Result{Err: fmt.Errorf("钩子执行失败: %w", runErr)}
}

// truncateRunes 按 rune 截断，避免切在多字节字符中间产生非法 UTF-8
func truncateRunes(s string, max int) string {
	if max <= 0 || utf8.RuneCountInString(s) <= max {
		return s
	}
	return string([]rune(s)[:max]) + "…"
}
