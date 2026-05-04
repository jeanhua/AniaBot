package functool

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/jeanhua/AniaBot/bot/component/llmtool"
)

const (
	bashTimeout   = 2 * time.Minute
	bashMaxOutput = 4096
)

// BashConfig bash工具配置
type BashConfig struct {
	Enable      bool     `json:"enable" mapstructure:"enable"`
	ContainerID string   `json:"container_id" mapstructure:"container_id"` // Docker 容器 ID 或名称
	Shell       string   `json:"shell" mapstructure:"shell"`               // 容器内的shell，如 bash、ash、sh
	Env         []string `json:"env" mapstructure:"env"`                   // 注入容器的环境变量，格式 KEY=VALUE
	Whitelist   []string `json:"whitelist" mapstructure:"whitelist"`       // 非空时只允许这些命令前缀
	Blacklist   []string `json:"blacklist" mapstructure:"blacklist"`       // 这些命令前缀被禁止
}

type BashParams struct {
	Command string `json:"command" desc:"要执行的bash命令"`
}

type BashTool struct {
	llmtool.BaseTool[BashParams]
	dockerClient *client.Client
	containerID  string
	shell        string
	env          []string
	whitelist    []string
	blacklist    []string
}

func NewBashTool(config BashConfig) (*BashTool, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("bash: 创建Docker客户端失败: %w", err)
	}

	shell := config.Shell
	if shell == "" {
		shell = "bash"
	}

	return &BashTool{
		BaseTool:     llmtool.MakeBaseTool("bash", "在Docker容器中执行命令，超时30秒，输出最大4096字符", BashParams{}),
		dockerClient: cli,
		containerID:  config.ContainerID,
		shell:        shell,
		env:          config.Env,
		whitelist:    config.Whitelist,
		blacklist:    config.Blacklist,
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

	// 创建 exec 实例
	execConfig := container.ExecOptions{
		Cmd:          []string{t.shell, "-c", p.Command},
		Env:          t.env,
		AttachStdout: true,
		AttachStderr: true,
	}
	execResp, err := t.dockerClient.ContainerExecCreate(ctx, t.containerID, execConfig)
	if err != nil {
		return "", fmt.Errorf("bash: 创建exec失败: %w", err)
	}

	// 连接到 exec
	attachResp, err := t.dockerClient.ContainerExecAttach(ctx, execResp.ID, container.ExecAttachOptions{})
	if err != nil {
		return "", fmt.Errorf("bash: 连接exec失败: %w", err)
	}
	defer attachResp.Close()

	// 读取 stdout 和 stderr
	var stdout, stderr bytes.Buffer
	_, err = stdcopy.StdCopy(&stdout, &stderr, attachResp.Reader)
	if err != nil {
		return "", fmt.Errorf("bash: 读取输出失败: %w", err)
	}

	// 检查退出码
	inspectResp, err := t.dockerClient.ContainerExecInspect(ctx, execResp.ID)
	if err != nil {
		return "", fmt.Errorf("bash: 检查执行状态失败: %w", err)
	}

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

	if inspectResp.ExitCode != 0 {
		return result, fmt.Errorf("bash: 命令退出码 %d", inspectResp.ExitCode)
	}
	return result, nil
}
