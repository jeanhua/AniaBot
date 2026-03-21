package pluginaichat

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jeanhua/AniaBot/bot/component/aichat"
	"github.com/jeanhua/AniaBot/bot/component/llmtool"
	"github.com/jeanhua/AniaBot/common/bot"
	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/spf13/viper"
	"github.com/tmc/langchaingo/llms"
)

func (p *AIChatPlugin) extraMsg(ctx context.Context, bot bot.Bot, msg message.Message, ocrLLM *aichat.ChatBot, opt ...llms.CallOption) string {
	var str strings.Builder
	str.WriteString(msg.FriendlyText(true,
		message.WithGetMsgFunc(bot.GetMsgDetail),
		message.WithGetForwardMsgFunc(bot.GetForwardMsg),
		message.WithGetImageOCRFunc(func(url string) string {
			if ocrLLM == nil {
				return "OCR服务未开启，无法解析图片"
			}
			resp, err := ocrLLM.GetSingleImageDesc(ctx, "描述图片内容", url, opt...)
			if err != nil {
				p.Logger.Error("OCR请求失败:", "error", err.Error())
				return "OCR请求失败，无法解析的图片内容"
			} else {
				return resp
			}
		}),
	))
	return str.String()
}

// mcpFileConfig aniabot.mcp.json 文件结构
type mcpFileConfig struct {
	Servers []*mcpServerEntry `json:"servers"`
}

// mcpServerEntry JSON 文件中单个服务器配置（timeout 用秒数表示）
type mcpServerEntry struct {
	Name        string            `json:"name"`
	Transport   string            `json:"transport"`
	Command     string            `json:"command"`
	Args        []string          `json:"args"`
	Env         map[string]string `json:"env"`
	Endpoint    string            `json:"endpoint"`
	Headers     map[string]string `json:"headers"`
	TimeoutSecs int               `json:"timeout"`
	Description string            `json:"description"`
}

const mcpConfigFile = "aniabot.mcp.json"

// loadMCPConfigs 从 aniabot.mcp.json 加载 MCP 服务器配置
func (p *AIChatPlugin) loadMCPConfigs(_ *viper.Viper) error {
	data, err := os.ReadFile(mcpConfigFile)
	if err != nil {
		if os.IsNotExist(err) {
			p.Logger.Info("未找到 MCP 配置文件，跳过 MCP 加载", "file", mcpConfigFile)
			return nil
		}
		return fmt.Errorf("读取 MCP 配置文件失败: %w", err)
	}

	var fileCfg mcpFileConfig
	if err := json.Unmarshal(data, &fileCfg); err != nil {
		return fmt.Errorf("解析 MCP 配置文件失败: %w", err)
	}

	if len(fileCfg.Servers) == 0 {
		p.Logger.Info("MCP 配置文件中未配置任何服务器")
		return nil
	}

	for i, entry := range fileCfg.Servers {
		if entry.Name == "" {
			p.Logger.Warn("MCP 服务器配置缺少名称", "index", i)
			continue
		}

		mcpConfig := &llmtool.MCPConfig{
			Name:        entry.Name,
			Transport:   entry.Transport,
			Command:     entry.Command,
			Args:        entry.Args,
			Env:         entry.Env,
			Endpoint:    entry.Endpoint,
			Headers:     entry.Headers,
			Description: entry.Description,
		}
		if entry.TimeoutSecs > 0 {
			mcpConfig.Timeout = time.Duration(entry.TimeoutSecs) * time.Second
		}

		transport := strings.ToLower(mcpConfig.Transport)
		isHTTP := transport == "streamable" || transport == "streamable-http" || transport == "sse"

		if isHTTP {
			if mcpConfig.Endpoint == "" {
				p.Logger.Warn("MCP 服务器配置缺少 endpoint", "name", mcpConfig.Name)
				continue
			}
		} else {
			if mcpConfig.Command == "" {
				p.Logger.Warn("MCP 服务器配置缺少 command", "name", mcpConfig.Name)
				continue
			}
		}

		p.mcpConfigs = append(p.mcpConfigs, mcpConfig)
		if isHTTP {
			p.Logger.Info("已加载 MCP 服务器配置", "name", mcpConfig.Name, "transport", mcpConfig.Transport, "endpoint", mcpConfig.Endpoint)
		} else {
			p.Logger.Info("已加载 MCP 服务器配置", "name", mcpConfig.Name, "command", mcpConfig.Command)
		}
	}

	p.Logger.Info("MCP 服务器配置加载完成", "count", len(p.mcpConfigs))
	return nil
}

// thinkingOpts 根据配置返回思考模式的 CallOption 列表
func (p *AIChatPlugin) thinkingOpts() []llms.CallOption {
	if !p.llmParameter.enableThinking {
		return nil
	}
	modeMap := map[string]llms.ThinkingMode{
		"none":   llms.ThinkingModeNone,
		"low":    llms.ThinkingModeLow,
		"medium": llms.ThinkingModeMedium,
		"high":   llms.ThinkingModeHigh,
		"auto":   llms.ThinkingModeAuto,
	}
	mode, ok := modeMap[p.llmParameter.thinkingMode]
	if !ok {
		mode = llms.ThinkingModeAuto
	}
	return []llms.CallOption{llms.WithThinkingMode(mode)}
}
