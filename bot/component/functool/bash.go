package functool

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/jeanhua/AniaBot/bot/component/llmtool"
)

const (
	bashTimeout   = 2 * time.Minute
	bashMaxOutput = 4096
)

// BashConfig bash工具配置
type BashConfig struct {
	Enable    bool     `json:"enable" mapstructure:"enable"`
	Shell     string   `json:"shell" mapstructure:"shell"`         // shell 路径，留空使用系统默认（Linux/macOS 为 sh，Windows 为 cmd）
	Env       []string `json:"env" mapstructure:"env"`             // 环境变量，格式 KEY=VALUE
	Whitelist []string `json:"whitelist" mapstructure:"whitelist"` // 非空时只允许匹配这些正则的命令
	Blacklist []string `json:"blacklist" mapstructure:"blacklist"` // 匹配这些正则的命令被禁止
}

type BashParams struct {
	Command string `json:"command" desc:"要执行的 shell 命令"`
}

type BashTool struct {
	llmtool.BaseTool[BashParams]
	shell     string
	shellArg  string // 命令行包装参数：sh/bash 为 -c，cmd 为 /C
	env       []string
	whitelist []*regexp.Regexp
	blacklist []*regexp.Regexp
}

// ResolveShell 解析实际使用的 shell 及其包装参数。
// 未配置时使用系统默认 shell，命令由该 shell 解释，
// AI 可在命令中显式调用 bash/ash/python 等其他解释器。
// 导出供 agenthook 等需要在宿主机执行管理员配置命令的组件复用。
func ResolveShell(configured string) (shell, shellArg string) {
	if configured == "" {
		if runtime.GOOS == "windows" {
			return "cmd", "/C"
		}
		return "sh", "-c"
	}
	base := strings.ToLower(filepath.Base(configured))
	if base == "cmd" || base == "cmd.exe" {
		return configured, "/C"
	}
	return configured, "-c"
}

// CmdVerdict 命令校验结论（三段式权限模型）。
type CmdVerdict int

const (
	// CmdAllow 命中白名单（或无任何名单），直接放行
	CmdAllow CmdVerdict = iota
	// CmdDeny 命中黑名单，直接拒绝
	CmdDeny
	// CmdAsk 既不在黑名单也不在白名单：需人工审批（经 CallBackFuncs.RequestApproval）
	CmdAsk
)

func NewBashTool(config BashConfig) (*BashTool, error) {
	shell, shellArg := ResolveShell(config.Shell)

	compile := func(patterns []string) ([]*regexp.Regexp, error) {
		regs := make([]*regexp.Regexp, 0, len(patterns))
		for _, p := range patterns {
			r, err := regexp.Compile(p)
			if err != nil {
				return nil, fmt.Errorf("编译正则 %q 失败: %w", p, err)
			}
			regs = append(regs, r)
		}
		return regs, nil
	}

	whitelist, err := compile(config.Whitelist)
	if err != nil {
		return nil, err
	}
	blacklist, err := compile(config.Blacklist)
	if err != nil {
		return nil, err
	}

	desc := fmt.Sprintf("在宿主机上执行 shell 命令（由 %s 解释执行），超时2分钟，输出最大4096字符。权限分三档：命中黑名单直接拒绝；命中白名单直接放行；两者都不命中时会向用户发起审批，等用户回复「允许」后才执行（审批未启用则拒绝）。注意：不要假设环境存在 bash，运行 .sh 脚本优先用 `sh 脚本路径`；需要 python3 等其他解释器时先用 `command -v` 确认其存在", shell)
	return &BashTool{
		BaseTool:  llmtool.MakeBaseTool("bash", desc, BashParams{}),
		shell:     shell,
		shellArg:  shellArg,
		env:       config.Env,
		whitelist: whitelist,
		blacklist: blacklist,
	}, nil
}

// checkCommand 三段式校验：黑名单优先（命中即拒绝）；白名单命中即放行；
// 两者都不命中返回 CmdAsk 交由人工审批（含两份名单都为空的场景——此时所有命令
// 都需要审批，避免「未配置名单=全部放行」的隐式风险）。
func (t *BashTool) checkCommand(cmd string) (CmdVerdict, error) {
	for _, blocked := range t.blacklist {
		if blocked.MatchString(cmd) {
			return CmdDeny, fmt.Errorf("bash: 命令被规则 %q 禁止", blocked.String())
		}
	}

	for _, w := range t.whitelist {
		if w.MatchString(cmd) {
			return CmdAllow, nil
		}
	}

	return CmdAsk, nil
}

// summarizeCommand 审批提示中的命令摘要：压缩空白并按 rune 截断，避免超长命令刷屏。
func summarizeCommand(cmd string) string {
	const maxRunes = 300
	compact := strings.Join(strings.Fields(cmd), " ")
	if r := []rune(compact); len(r) > maxRunes {
		return string(r[:maxRunes]) + "…"
	}
	return compact
}

func (t *BashTool) Execute(ctx context.Context, params any, cbs llmtool.CallBackFuncs) (string, error) {
	p, ok := params.(*BashParams)
	if !ok {
		return "", fmt.Errorf("bash: 参数类型错误")
	}
	if p.Command == "" {
		return "", fmt.Errorf("bash: 命令不能为空")
	}

	log.Println("执行bash... 参数: ", p.Command)

	verdict, err := t.checkCommand(p.Command)
	if err != nil {
		return "", err
	}
	if verdict == CmdAsk {
		if cbs.RequestApproval == nil {
			return "", fmt.Errorf("bash: 命令 %q 不在白名单中且当前未启用人工审批，已拒绝。如需放行：请管理员把匹配该命令的正则加入白名单（plugin.ai_chat_bot.bash.whitelist），或启用工具审批（plugin.ai_chat_bot.approval.enable）", summarizeCommand(p.Command))
		}
		allowed, reason := cbs.RequestApproval(ctx, "bash", "执行命令："+summarizeCommand(p.Command))
		if !allowed {
			return "", fmt.Errorf("bash: 命令未获批准：%s", reason)
		}
	}

	// 基于调用方 ctx 派生超时：/stop 取消请求时命令随之终止，
	// 不会因忽略 ctx 而让长命令继续占满会话锁与并发槽
	ctx, cancel := context.WithTimeout(ctx, bashTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, t.shell, t.shellArg, p.Command)
	if len(t.env) > 0 {
		// 追加而非替换：直接赋值 cmd.Env 会丢弃进程继承的 PATH/HOME 等变量，
		// 配置任意一个自定义变量就会破坏依赖继承环境的命令查找
		cmd.Env = append(cmd.Environ(), t.env...)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()

	result := stdout.String()
	if stderr.Len() > 0 {
		if result != "" {
			result += "\n"
		}
		result += "stderr: " + stderr.String()
	}

	if r := []rune(result); len(r) > bashMaxOutput {
		// 按 rune 截断，避免切在多字节字符中间产生非法 UTF-8
		result = string(r[:bashMaxOutput]) + "\n...(输出已截断)"
	}

	if ctx.Err() == context.DeadlineExceeded {
		return result + "\n命令执行超时(2分钟)", nil
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			// 非零退出码不是工具层错误：返回 error 会导致 stdout/stderr 被执行器
			// 丢弃、模型只看到"退出码 N"。把输出与退出码一并作为结果返回
			if result != "" {
				result += "\n"
			}
			if exitErr.ExitCode() == 127 {
				// 127 只表示"命令未找到"，具体是哪个命令缺失应以 stderr 为准（如 "sh: curl: not found"），
				// 不要臆断为某个特定解释器缺失
				result += "(命令退出码 127：命令未找到。请根据 stderr 确认缺失的命令或解释器，可用 `command -v <命令> || echo missing` 验证后重试)"
			} else {
				result += fmt.Sprintf("(命令退出码 %d)", exitErr.ExitCode())
			}
			return result, nil
		}
		return result, fmt.Errorf("bash: 执行失败: %w", err)
	}
	return result, nil
}
