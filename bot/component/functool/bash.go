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

// resolveShell 解析实际使用的 shell 及其包装参数。
// 未配置时使用系统默认 shell，命令由该 shell 解释，
// AI 可在命令中显式调用 bash/ash/python 等其他解释器。
func resolveShell(configured string) (shell, shellArg string) {
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

func NewBashTool(config BashConfig) (*BashTool, error) {
	shell, shellArg := resolveShell(config.Shell)

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

	desc := fmt.Sprintf("在宿主机上执行 shell 命令（由 %s 解释执行），超时2分钟，输出最大4096字符。注意：不要假设环境存在 bash，运行 .sh 脚本优先用 `sh 脚本路径`；需要 python3 等其他解释器时先用 `command -v` 确认其存在", shell)
	return &BashTool{
		BaseTool:  llmtool.MakeBaseTool("bash", desc, BashParams{}),
		shell:     shell,
		shellArg:  shellArg,
		env:       config.Env,
		whitelist: whitelist,
		blacklist: blacklist,
	}, nil
}

func (t *BashTool) checkCommand(cmd string) error {
	for _, blocked := range t.blacklist {
		if blocked.MatchString(cmd) {
			return fmt.Errorf("bash: 命令被规则 %q 禁止", blocked.String())
		}
	}

	if len(t.whitelist) > 0 {
		allowed := false
		for _, w := range t.whitelist {
			if w.MatchString(cmd) {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("bash: 命令不匹配任何白名单规则")
		}
	}

	return nil
}

func (t *BashTool) Execute(_ context.Context, params any, _ llmtool.CallBackFuncs) (string, error) {
	p, ok := params.(*BashParams)
	if !ok {
		return "", fmt.Errorf("bash: 参数类型错误")
	}
	if p.Command == "" {
		return "", fmt.Errorf("bash: 命令不能为空")
	}

	log.Println("执行bash... 参数: ", p.Command)

	if err := t.checkCommand(p.Command); err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(context.Background(), bashTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, t.shell, t.shellArg, p.Command)
	if len(t.env) > 0 {
		cmd.Env = t.env
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := stdout.String()
	if stderr.Len() > 0 {
		if result != "" {
			result += "\n"
		}
		result += "stderr: " + stderr.String()
	}

	if len(result) > bashMaxOutput {
		result = result[:bashMaxOutput] + "\n...(输出已截断)"
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
