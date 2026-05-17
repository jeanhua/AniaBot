package functool

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os/exec"
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
	Shell     string   `json:"shell" mapstructure:"shell"`         // shell 路径，默认 /bin/bash
	Env       []string `json:"env" mapstructure:"env"`             // 环境变量，格式 KEY=VALUE
	Whitelist []string `json:"whitelist" mapstructure:"whitelist"` // 非空时只允许这些命令前缀
	Blacklist []string `json:"blacklist" mapstructure:"blacklist"` // 这些命令前缀被禁止
}

type BashParams struct {
	Command string `json:"command" desc:"要执行的bash命令"`
}

type BashTool struct {
	llmtool.BaseTool[BashParams]
	shell     string
	env       []string
	whitelist []string
	blacklist []string
}

func NewBashTool(config BashConfig) (*BashTool, error) {
	shell := config.Shell
	if shell == "" {
		shell = "/bin/bash"
	}

	return &BashTool{
		BaseTool:  llmtool.MakeBaseTool("bash", "在宿主机上执行bash命令，超时2分钟，输出最大4096字符", BashParams{}),
		shell:     shell,
		env:       config.Env,
		whitelist: config.Whitelist,
		blacklist: config.Blacklist,
	}, nil
}

func (t *BashTool) checkCommand(cmd string) error {
	firstWord := strings.Fields(cmd)[0]

	for _, blocked := range t.blacklist {
		if firstWord == blocked {
			return fmt.Errorf("bash: 命令 '%s' 被禁止", firstWord)
		}
	}

	if len(t.whitelist) > 0 {
		allowed := false
		for _, w := range t.whitelist {
			if firstWord == w {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("bash: 命令 '%s' 不在白名单中", firstWord)
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

	cmd := exec.CommandContext(ctx, t.shell, "-c", p.Command)
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
			return result, fmt.Errorf("bash: 命令退出码 %d", exitErr.ExitCode())
		}
		return result, fmt.Errorf("bash: 执行失败: %w", err)
	}
	return result, nil
}
