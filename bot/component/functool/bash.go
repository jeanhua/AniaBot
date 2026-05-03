package functool

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/jeanhua/AniaBot/bot/component/llmtool"
)

const (
	bashTimeout   = 30 * time.Second
	bashMaxOutput = 4096
)

// BashConfig bash工具配置
type BashConfig struct {
	Enable    bool     `json:"enable" mapstructure:"enable"`
	Whitelist []string `json:"whitelist" mapstructure:"whitelist"` // 非空时只允许这些命令前缀
	Blacklist []string `json:"blacklist" mapstructure:"blacklist"` // 这些命令前缀被禁止
}

type BashParams struct {
	Command string `json:"command" desc:"要执行的bash命令"`
}

type BashTool struct {
	llmtool.BaseTool[BashParams]
	whitelist []string
	blacklist []string
}

func NewBashTool(config BashConfig) *BashTool {
	return &BashTool{
		BaseTool:  llmtool.MakeBaseTool("bash", "用于执行bash命令，超时30秒，输出最大4096字符", BashParams{}),
		whitelist: config.Whitelist,
		blacklist: config.Blacklist,
	}
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

	if err := t.checkCommand(p.Command); err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(context.Background(), bashTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", "-c", p.Command)
	output, err := cmd.CombinedOutput()

	result := string(output)
	if len(result) > bashMaxOutput {
		result = result[:bashMaxOutput] + "\n...(输出已截断)"
	}

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return result + "\n命令执行超时(30秒)", nil
		}
		return result + "\n错误: " + err.Error(), nil
	}
	return result, nil
}
