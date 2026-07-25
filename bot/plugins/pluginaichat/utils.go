package pluginaichat

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jeanhua/AniaBot/bot/component/aichat"
	"github.com/jeanhua/AniaBot/bot/component/llmtool"
	"github.com/jeanhua/AniaBot/common/bot"
	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/spf13/viper"
)

func (p *AIChatPlugin) extraMsg(bot bot.Bot, msg message.Message) string {
	return msg.FriendlyText(false,
		message.WithGetMsgFunc(bot.GetMsgDetail),
		message.WithGetForwardMsgFunc(bot.GetForwardMsg),
	)
}

func collectImageURLs(bot bot.Bot, msgs ...message.Message) []string {
	urls := make([]string, 0)
	seenURLs := make(map[string]struct{})
	seenMessages := make(map[message.QID]struct{})

	var collect func(message.Message)
	collect = func(current message.Message) {
		if current.MessageId != "" {
			if _, ok := seenMessages[current.MessageId]; ok {
				return
			}
			seenMessages[current.MessageId] = struct{}{}
		}
		for _, segment := range current.Message {
			switch segment.Type {
			case message.SegmentImage:
				var image message.ImageMessage
				if message.ParseImage(segment, &image) && image.Url != "" {
					if _, ok := seenURLs[image.Url]; !ok {
						seenURLs[image.Url] = struct{}{}
						urls = append(urls, image.Url)
					}
				}
			case message.SegmentReply:
				var reply message.ReplyMessage
				if message.ParseReply(segment, &reply) {
					if detail, ok := bot.GetMsgDetail(reply.Id); ok && detail != nil {
						collect(*detail)
					}
				}
			}
		}
	}

	for _, msg := range msgs {
		collect(msg)
	}
	return urls
}

func (p *AIChatPlugin) configureImageCallbacks(ctx context.Context, bot bot.Bot, callbacks *llmtool.CallBackFuncs, msgs ...message.Message) {
	imageURLs := collectImageURLs(bot, msgs...)
	var loadedImages []string
	loaded := false

	callbacks.LoadImages = func() (string, error) {
		if loaded {
			return "当前消息中的图片已经加载，无需重复调用", nil
		}
		loaded = true
		if len(imageURLs) == 0 {
			return "当前消息及其引用消息中没有可加载的图片", nil
		}

		if p.cfg.Multimodal {
			loadedImages = append(loadedImages, imageURLs...)
			return fmt.Sprintf("已加载 %d 张图片，图片将在下一轮上下文中提供，请直接查看图片后回答", len(imageURLs)), nil
		}

		if p.ocrModel == nil {
			return "当前主模型不支持加载图片，且未配置备用图片识别模型，无法查看图片内容", nil
		}

		var result strings.Builder
		result.WriteString("主模型不支持多模态，以下是备用图片识别模型返回的图片描述：")
		for i, imageURL := range imageURLs {
			description, err := p.ocrModel.GetSingleImageDesc(ctx, "描述图片内容", imageURL, p.buildOCRChatOptions())
			if err != nil {
				p.Logger.Error("备用图片识别请求失败", "index", i+1, "error", err.Error())
				result.WriteString(fmt.Sprintf("\n<图片 %d>识别失败：%s</图片 %d>", i+1, err.Error(), i+1))
				continue
			}
			result.WriteString(fmt.Sprintf("\n<图片 %d>\n%s\n</图片 %d>", i+1, description, i+1))
		}
		return result.String(), nil
	}
	callbacks.TakeLoadedImages = func() []string {
		images := loadedImages
		loadedImages = nil
		return images
	}
	callbacks.LoadLocalImage = func(path string) (string, error) {
		return p.loadLocalImageInto(ctx, path, &loadedImages), nil
	}
}

// loadLocalImageInto 读取本地图片供 LLM 查看：主模型支持多模态时把 data URI 推入
// 待加载队列（loadedImages 由调用方持有，下一轮上下文提供），否则交由备用识别模型描述。
// 与 file 工具一致，禁止读取配置文件以避免凭据等敏感信息经图片通道泄露。
func (p *AIChatPlugin) loadLocalImageInto(ctx context.Context, path string, loadedImages *[]string) string {
	if strings.Contains(path, "aniabot.db") {
		return "禁止读取aniabot数据库文件"
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Sprintf("读取本地图片失败: %v", err)
	}
	dataURI := "data:" + imageMIME(path) + ";base64," + base64.StdEncoding.EncodeToString(data)

	if p.cfg.Multimodal {
		// data URI 推入待加载队列，下一轮由 TakeLoadedImages 取出并入上下文；
		// data URI 不依赖外部链接，历史持久化后重启也不会失效
		*loadedImages = append(*loadedImages, dataURI)
		return fmt.Sprintf("已加载本地图片 %s，将在下一轮上下文中提供，请直接查看图片后回答", path)
	}

	if p.ocrModel == nil {
		return "当前主模型不支持加载图片，且未配置备用图片识别模型，无法查看图片内容"
	}
	description, err := p.ocrModel.GetSingleImageDesc(ctx, "描述图片内容", dataURI, p.buildOCRChatOptions())
	if err != nil {
		p.Logger.Error("备用图片识别请求失败", "path", path, "error", err.Error())
		return fmt.Sprintf("本地图片识别失败: %v", err)
	}
	return "主模型不支持多模态，以下是备用图片识别模型返回的图片描述：\n" + description
}

// imageMIME 根据文件扩展名推断图片 MIME 类型，无法识别时回退到 image/png。
func imageMIME(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".bmp":
		return "image/bmp"
	default:
		return "image/png"
	}
}

type mcpFileConfig struct {
	Servers []*mcpServerEntry `json:"servers"`
}

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

const mcpConfigKey = "files.mcp_json"

// loadMCPConfigs 从配置中心读取 MCP 服务器配置（原 aniabot.mcp.json 的原始 JSON 文本）。
func (p *AIChatPlugin) loadMCPConfigs(cfg *viper.Viper) error {
	raw := cfg.GetString(mcpConfigKey)
	if strings.TrimSpace(raw) == "" {
		p.Logger.Info("未配置 MCP 服务器，跳过 MCP 加载", "key", mcpConfigKey)
		return nil
	}

	var fileCfg mcpFileConfig
	if err := json.Unmarshal([]byte(raw), &fileCfg); err != nil {
		return fmt.Errorf("解析 MCP 配置失败: %w", err)
	}

	if len(fileCfg.Servers) == 0 {
		p.Logger.Info("MCP 配置中未配置任何服务器")
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

func (p *AIChatPlugin) thinkingOpts() aichat.ChatOptions {
	if !p.cfg.Thinking.Enable {
		return aichat.ChatOptions{}
	}

	effort := p.cfg.Thinking.Mode
	if effort == "" || effort == "auto" {
		return aichat.ChatOptions{}
	}

	validEfforts := map[string]bool{"low": true, "medium": true, "high": true}
	if !validEfforts[effort] {
		effort = "low"
	}

	return aichat.ChatOptions{
		ReasoningEffort: &effort,
	}
}
