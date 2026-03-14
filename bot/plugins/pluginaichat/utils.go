package pluginaichat

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jeanhua/AniaBot/bot/component/aichat"
	"github.com/jeanhua/AniaBot/bot/component/llmtool"
	"github.com/jeanhua/AniaBot/common/bot"
	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/spf13/viper"
	"github.com/tmc/langchaingo/llms"
)

// getStringFromMap 从 map 中获取字符串值
func getStringFromMap(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		if str, ok := v.(string); ok {
			return str
		}
	}
	return ""
}

func (p *AIChatPlugin) extraMsg(ctx context.Context, bot bot.Bot, msg message.Message, ocrLLM *aichat.ChatBot, opt ...llms.CallOption) string {
	var str strings.Builder
	nickname := msg.Sender.Card
	if nickname == "" {
		nickname = msg.Sender.Nickname
	}
	str.WriteString(fmt.Sprintf("[nickname:%s id:%d]:", nickname, msg.Sender.UserId))
	for _, m := range msg.Message {
		str.WriteString(
			m.FriendlyText(
				message.WithIgnoreMentionId(msg.SelfId),
				message.WithGetMsgFunc(bot.GetMsgDetail),
				message.WithGetGroupUserInfo(msg.GroupId, bot.GetGroupUserInfo),
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
			),
		)
	}
	return str.String()
}

// loadMCPConfigs 从配置加载 MCP 服务器配置
func (p *AIChatPlugin) loadMCPConfigs(cfg *viper.Viper) error {
	// 检查是否有 MCP 配置
	mcpServers := cfg.Get("plugin.ai_chat_bot.mcp.servers")
	if mcpServers == nil {
		p.Logger.Info("未配置 MCP 服务器")
		return nil
	}

	// 解析 MCP 服务器配置
	serversConfig, ok := mcpServers.([]any)
	if !ok {
		return fmt.Errorf("MCP 服务器配置格式错误")
	}

	for i, server := range serversConfig {
		serverMap, ok := server.(map[string]any)
		if !ok {
			p.Logger.Warn("MCP 服务器配置项格式错误", "index", i)
			continue
		}

		mcpConfig := &llmtool.MCPConfig{
			Name:        getStringFromMap(serverMap, "name"),
			Transport:   getStringFromMap(serverMap, "transport"),
			Command:     getStringFromMap(serverMap, "command"),
			Endpoint:    getStringFromMap(serverMap, "endpoint"),
			Description: getStringFromMap(serverMap, "description"),
		}

		// 读取 args
		if args, ok := serverMap["args"].([]any); ok {
			for _, arg := range args {
				if str, ok := arg.(string); ok {
					mcpConfig.Args = append(mcpConfig.Args, str)
				}
			}
		}

		// 读取 env
		if env, ok := serverMap["env"].(map[string]any); ok {
			mcpConfig.Env = make(map[string]string)
			for k, v := range env {
				if str, ok := v.(string); ok {
					mcpConfig.Env[strings.ToUpper(k)] = str
				}
			}
		}

		// 读取 headers
		if headers, ok := serverMap["headers"].(map[string]any); ok {
			mcpConfig.Headers = make(map[string]string)
			for k, v := range headers {
				if str, ok := v.(string); ok {
					mcpConfig.Headers[k] = str
				}
			}
		}

		// 读取 timeout
		if timeout, ok := serverMap["timeout"].(int); ok {
			mcpConfig.Timeout = time.Duration(timeout) * time.Second
		}

		// 验证配置
		if mcpConfig.Name == "" {
			p.Logger.Warn("MCP 服务器配置缺少名称", "index", i)
			continue
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
