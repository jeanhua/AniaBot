package pluginaichat

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jeanhua/AniaBot/bot/component/aichat"
	"github.com/jeanhua/AniaBot/bot/component/llmtool"
	"github.com/jeanhua/AniaBot/common/bot"
	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/spf13/viper"
)

// botQQ 返回 bot 的 QQ 平台专属能力（事件来源为 QQ 适配器时可用），否则返回 nil。
func botQQ(b bot.Bot) bot.QQ {
	if qb, ok := b.(bot.QQ); ok {
		return qb
	}
	return nil
}

// parseQID 解析目标 ID：纯数字（QQ 群号/QQ号）规范化为 qq: 前缀 QID，
// 其他（多平台带前缀，如飞书 fs:oc_xxx）原样保留。
func parseQID(s string) message.QID {
	if n, err := strconv.ParseUint(s, 10, 64); err == nil {
		return message.FromUint64(n)
	}
	return message.FromString(strings.TrimSpace(s))
}

func (p *AIChatPlugin) extraMsg(b bot.Bot, msg message.Message) string {
	opts := []message.MsgOptFunc{
		message.WithGetMsgFunc(b.GetMsgDetail),
	}
	// 合并转发展开属 QQ 平台能力：仅事件来源为 QQ 时挂载，其余平台回退占位符
	if qb := botQQ(b); qb != nil {
		opts = append(opts, message.WithGetForwardMsgFunc(qb.GetForwardMsg))
	}
	// 合成消息（如子代理结果，UserId 为 0）不附加 [nickname:… id:…] 发送者前缀，
	// 其正文已自带身份标识
	if msg.Sender.UserId == message.FromUint64(0) {
		opts = append(opts, message.WithNoSenderPrefix())
	}
	return msg.FriendlyText(true, opts...)
}

// imageRef 已解析到的图片引用：哈希用于与消息文本中的 [图片 <hash>] 标记对应，
// URL 用于真正加载图片进上下文（多模态）或交给备用识别模型（OCR）。
type imageRef struct {
	Hash string
	URL  string
}

// imageRegistry 请求级图片哈希→URL 注册表。消息展示给 AI 的每张图片
// （当前消息、get_msg_history 历史记录、合并转发内容）都会登记，
// load_images 按哈希查找并只加载指定的图片，避免一次全部塞进上下文。
type imageRegistry struct {
	mu     sync.Mutex
	byHash map[string]string
}

func newImageRegistry() *imageRegistry {
	return &imageRegistry{byHash: make(map[string]string)}
}

// register 登记图片哈希→URL 映射；同一哈希只保留首次登记（先到先得）。
func (r *imageRegistry) register(hash, url string) {
	if hash == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.byHash[hash]; !ok {
		r.byHash[hash] = url
	}
}

// resolve 按哈希解析图片引用，返回找到的引用与未登记的哈希。
func (r *imageRegistry) resolve(hashes []string) (found []imageRef, missing []string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, hash := range hashes {
		url, ok := r.byHash[hash]
		if !ok {
			missing = append(missing, hash)
			continue
		}
		found = append(found, imageRef{Hash: hash, URL: url})
	}
	return found, missing
}

// imageMessageSource 消息图片登记所需的最小消息查询能力（bot.Bot 天然满足）。
type imageMessageSource interface {
	GetMsgDetail(msgId message.QID) (*message.Message, bool)
}

// imageForwardSource 合并转发展开能力（QQ 平台独有，bot.Bot 未必实现）。
type imageForwardSource interface {
	GetForwardMsg(msgId message.QID) (*[]message.Message, bool)
}

// registerMessageImages 递归登记消息及其引用/合并转发中的图片（与 FriendlyText
// 的展开范围一致），使 load_images 能按哈希加载历史记录与转发里的图片。
func registerMessageImages(reg *imageRegistry, src imageMessageSource, msgs ...message.Message) {
	if reg == nil {
		return
	}
	seen := make(map[message.QID]struct{})
	for _, msg := range msgs {
		reg.registerMessage(src, msg, seen)
	}
}

func (r *imageRegistry) registerMessage(src imageMessageSource, current message.Message, seen map[message.QID]struct{}) {
	if current.MessageId != "" {
		if _, ok := seen[current.MessageId]; ok {
			return
		}
		seen[current.MessageId] = struct{}{}
	}
	for _, segment := range current.Message {
		switch segment.Type {
		case message.SegmentImage:
			var image message.ImageMessage
			if message.ParseImage(segment, &image) {
				r.register(image.Hash(), image.Url)
			}
		case message.SegmentReply:
			var reply message.ReplyMessage
			if message.ParseReply(segment, &reply) {
				if detail, ok := src.GetMsgDetail(reply.Id); ok && detail != nil {
					r.registerMessage(src, *detail, seen)
				}
			}
		case message.SegmentForward:
			var fwd message.ForwardMessage
			if message.ParseForward(segment, &fwd) {
				if fwdSrc, ok := src.(imageForwardSource); ok {
					if detail, ok := fwdSrc.GetForwardMsg(fwd.Id); ok && detail != nil {
						for i := range *detail {
							r.registerMessage(src, (*detail)[i], seen)
						}
					}
				}
			}
		}
	}
}

// configureImageCallbacks 挂载消息图片的加载回调。registry 为本次请求的
// 图片哈希→URL 注册表（当前消息、历史记录、合并转发中的图片都会登记），
// load_images 按哈希解析并只加载指定的图片。usageSink 接收备用图片识别
// （OCR）产生的 LLM 用量，由调用方并入所属请求/会话的统计与配额。
func (p *AIChatPlugin) configureImageCallbacks(ctx context.Context, bot bot.Bot, callbacks *llmtool.CallBackFuncs, registry *imageRegistry, usageSink func(aichat.TokenUsage), msgs ...message.Message) {
	registerMessageImages(registry, bot, msgs...)
	var loadedImages []string
	// loadedHashes 记录已加载过的图片哈希，避免同一张图片重复进入上下文/重复 OCR
	loadedHashes := make(map[string]struct{})

	callbacks.LoadImages = func(hashes []string) (string, error) {
		if len(hashes) == 0 {
			return "请通过 hashes 参数传入要查看的图片哈希。图片在消息中以 [图片 <hash> url:<url>] 标识，可从当前消息、get_msg_history 历史记录或合并转发内容中获取", nil
		}

		found, missing := registry.resolve(hashes)
		if len(found) == 0 {
			msg := "没有找到可加载的图片："
			if len(missing) > 0 {
				msg += "未登记的哈希 " + strings.Join(missing, "、") + "（请确认哈希来自消息中的 [图片 <hash> url:<url>] 标记）"
			}
			return msg, nil
		}

		// 只加载本次尚未加载过的图片；已加载的哈希直接跳过
		var toLoad []imageRef
		already := make([]string, 0)
		for _, ref := range found {
			if _, ok := loadedHashes[ref.Hash]; ok {
				already = append(already, "[图片 "+ref.Hash+"]")
				continue
			}
			if ref.URL == "" {
				missing = append(missing, ref.Hash+"（无可用链接）")
				continue
			}
			loadedHashes[ref.Hash] = struct{}{}
			toLoad = append(toLoad, ref)
		}

		if len(toLoad) == 0 {
			msg := "这些图片已经加载过，无需重复调用（" + strings.Join(already, "、") + "）"
			if len(missing) > 0 {
				msg += "；未找到：" + strings.Join(missing, "、")
			}
			return msg, nil
		}

		if p.cfg.Multimodal {
			for _, ref := range toLoad {
				loadedImages = append(loadedImages, ref.URL)
			}
			// 列出每张图片的哈希标识，与消息文本中的 [图片 <hash>] 标记对应
			labels := make([]string, 0, len(toLoad))
			for _, ref := range toLoad {
				labels = append(labels, "[图片 "+ref.Hash+"]")
			}
			var extra string
			if len(missing) > 0 {
				extra = "；未找到：" + strings.Join(missing, "、")
			}
			return fmt.Sprintf("已加载 %d 张图片（%s），图片将在下一轮上下文中提供，请直接查看图片后回答%s", len(toLoad), strings.Join(labels, "、"), extra), nil
		}

		if p.ocrModel == nil {
			return "当前主模型不支持加载图片，且未配置备用图片识别模型，无法查看图片内容", nil
		}

		var result strings.Builder
		result.WriteString("主模型不支持多模态，以下是备用图片识别模型返回的图片描述：")
		for _, ref := range toLoad {
			description, usage, err := p.ocrModel.GetSingleImageDesc(ctx, "描述图片内容", ref.URL, p.buildOCRChatOptions())
			if err != nil {
				p.Logger.Error("备用图片识别请求失败", "hash", ref.Hash, "error", err.Error())
				result.WriteString(fmt.Sprintf("\n<图片 %s>识别失败：%s</图片 %s>", ref.Hash, err.Error(), ref.Hash))
				continue
			}
			if usageSink != nil {
				usageSink(usage)
			}
			result.WriteString(fmt.Sprintf("\n<图片 %s>\n%s\n</图片 %s>", ref.Hash, description, ref.Hash))
		}
		if len(missing) > 0 {
			result.WriteString("\n未找到：" + strings.Join(missing, "、"))
		}
		return result.String(), nil
	}
	callbacks.TakeLoadedImages = func() []string {
		images := loadedImages
		loadedImages = nil
		return images
	}
	callbacks.LoadLocalImage = func(path string) (string, error) {
		return p.loadLocalImageInto(ctx, path, &loadedImages, usageSink), nil
	}
}

// loadLocalImageInto 读取本地图片供 LLM 查看：主模型支持多模态时把 data URI 推入
// 待加载队列（loadedImages 由调用方持有，下一轮上下文提供），否则交由备用识别模型描述。
// usageSink 接收备用识别产生的 LLM 用量（多模态路径不产生产量，不会调用）。
// 与 file 工具一致，禁止读取配置文件以避免凭据等敏感信息经图片通道泄露。
func (p *AIChatPlugin) loadLocalImageInto(ctx context.Context, path string, loadedImages *[]string, usageSink func(aichat.TokenUsage)) string {
	if strings.Contains(path, "aniabot.db") {
		return "禁止读取aniabot数据库文件"
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Sprintf("读取本地图片失败: %v", err)
	}
	dataURI := "data:" + imageMIME(path) + ";base64," + base64.StdEncoding.EncodeToString(data)
	hash := message.ImageHash(dataURI)

	if p.cfg.Multimodal {
		// data URI 推入待加载队列，下一轮由 TakeLoadedImages 取出并入上下文；
		// data URI 不依赖外部链接，历史持久化后重启也不会失效
		*loadedImages = append(*loadedImages, dataURI)
		return fmt.Sprintf("已加载本地图片 %s（[图片 %s]），将在下一轮上下文中提供，请直接查看图片后回答", path, hash)
	}

	if p.ocrModel == nil {
		return "当前主模型不支持加载图片，且未配置备用图片识别模型，无法查看图片内容"
	}
	description, usage, err := p.ocrModel.GetSingleImageDesc(ctx, "描述图片内容", dataURI, p.buildOCRChatOptions())
	if err != nil {
		p.Logger.Error("备用图片识别请求失败", "path", path, "error", err.Error())
		return fmt.Sprintf("本地图片识别失败: %v", err)
	}
	if usageSink != nil {
		usageSink(usage)
	}
	return fmt.Sprintf("主模型不支持多模态，以下是备用图片识别模型返回的图片描述：\n<图片 %s>\n%s\n</图片 %s>", hash, description, hash)
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

		mcpConfig, err := mcpEntryToConfig(entry)
		if err != nil {
			p.Logger.Warn("MCP 服务器配置无效", "name", entry.Name, "error", err.Error())
			continue
		}

		p.mcpConfigs = append(p.mcpConfigs, mcpConfig)
		if mcpConfig.Endpoint != "" {
			p.Logger.Info("已加载 MCP 服务器配置", "name", mcpConfig.Name, "transport", mcpConfig.Transport, "endpoint", mcpConfig.Endpoint)
		} else {
			p.Logger.Info("已加载 MCP 服务器配置", "name", mcpConfig.Name, "command", mcpConfig.Command)
		}
	}

	p.Logger.Info("MCP 服务器配置加载完成", "count", len(p.mcpConfigs))
	return nil
}

// mcpEntryToConfig 将持久化条目转换为运行时 MCP 配置并做基本校验
// （HTTP 模式缺 endpoint / stdio 模式缺 command 均视为无效）。
func mcpEntryToConfig(entry *mcpServerEntry) (*llmtool.MCPConfig, error) {
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
			return nil, fmt.Errorf("缺少 endpoint")
		}
	} else if mcpConfig.Command == "" {
		return nil, fmt.Errorf("缺少 command")
	}
	return mcpConfig, nil
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
