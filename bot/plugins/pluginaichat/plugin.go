package pluginaichat

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/jeanhua/AniaBot/bot/component/aichat"
	"github.com/jeanhua/AniaBot/bot/component/functool"
	"github.com/jeanhua/AniaBot/bot/component/llmtool"
	"github.com/jeanhua/AniaBot/common/aniaerror"
	"github.com/jeanhua/AniaBot/common/bot"
	"github.com/jeanhua/AniaBot/common/model/command"
	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/jeanhua/AniaBot/common/msgchain"
	"github.com/jeanhua/AniaBot/common/plugin"
	"github.com/jeanhua/AniaBot/common/plugininfo"
	"github.com/jeanhua/AniaBot/common/storage"
	"github.com/spf13/viper"
	"github.com/tmc/langchaingo/llms"
)

type AIChatPlugin struct {
	plugin.Meta
	chats sync.Map

	lockStorage storage.Storage

	// 用于存储活跃的请求上下文，支持取消操作
	activeContexts sync.Map // map[message.QID]context.CancelFunc

	botConfig struct {
		baseURL string
		apiKey  string
		model   string
	}

	llmParameter struct {
		maxToken    int
		temperature float64
		top_p       float64
		top_k       int
		prompt      string
		searchToken string
	}

	ocrEnable    bool
	ocrModel     *aichat.ChatBot
	ocrParameter struct {
		maxToken    int
		temperature float64
		top_p       float64
		top_k       int
		prompt      string
	}

	mcpConfigs   []*llmtool.MCPConfig
	toolExecutor *llmtool.ToolExecuter // 共享的工具执行器

	skillManager *llmtool.SkillManager
}

const (
	LockExpTime = time.Minute * 10
)

func NewAIChatPlugin() *AIChatPlugin {
	return &AIChatPlugin{
		Meta: plugin.Meta{
			Name:      "AI对话插件",
			HelpWords: "@我聊天哦，带上 #新对话 标签可以创建新对话，发送 /stop 可以停止 AI 响应",
			Order:     plugin.LevelPostHandle,
			ShowFor:   plugininfo.ShowForGroup | plugininfo.ShowForFriend,
			Author:    "jeanhua",
			Version:   "1.0.0",
		},
	}
}

func (p *AIChatPlugin) tryLock(ctx context.Context, id message.QID) bool {
	return p.lockStorage.SetString(ctx, id.String(), "1", storage.WithCheckExist(), storage.WithTTL(LockExpTime))
}

func (p *AIChatPlugin) unLock(ctx context.Context, id message.QID) {
	p.lockStorage.Del(ctx, id.String())
}

// stopRequest 停止指定 ID 的 AI 请求
func (p *AIChatPlugin) stopRequest(id message.QID) bool {
	if cancel, ok := p.activeContexts.LoadAndDelete(id); ok {
		cancel.(context.CancelFunc)()
		return true
	}
	return false
}

// setActiveContext 设置活跃的请求上下文
func (p *AIChatPlugin) setActiveContext(id message.QID, cancel context.CancelFunc) {
	p.activeContexts.Store(id, cancel)
}

// clearActiveContext 清除活跃的请求上下文
func (p *AIChatPlugin) clearActiveContext(id message.QID) {
	p.activeContexts.Delete(id)
}

func (p *AIChatPlugin) getChat(id message.QID) *aichat.ChatBot {
	chat, ok := p.chats.Load(id)
	if !ok {
		c, err := aichat.NewChatBot(
			p.botConfig.baseURL,
			p.botConfig.apiKey,
			p.botConfig.model,
			p.llmParameter.prompt,
			30,
			p.toolExecutor,
		)
		if err != nil {
			p.Logger.Error("创建 ChatBot 失败", "error", err.Error())
			return nil
		}
		// 注入 SkillManager，让 system prompt 包含 available_skills
		if p.skillManager != nil {
			c.SetSkillManager(p.skillManager)
		}
		p.chats.Store(id, c)
		return c
	}
	return chat.(*aichat.ChatBot)
}

func (p *AIChatPlugin) OnGroupMsg(ctx context.Context, bot bot.Bot, cmd command.Command, msg message.Message) (bool, error) {
	if !cmd.Mention {
		return true, nil
	}

	// 处理 /stop 命令
	if cmd.Name == "stop" {
		if p.stopRequest(msg.GroupId) {
			builder := msgchain.Builder().Group()
			builder.Text("已停止 AI 响应")
			bot.SendGroupMsg(msg.GroupId, builder.Build())
			p.Logger.Info("用户停止 AI 响应", "group", msg.GroupId, "user", msg.Sender.UserId)
		} else {
			builder := msgchain.Builder().Group()
			builder.Text("当前没有正在进行的 AI 请求")
			bot.SendGroupMsg(msg.GroupId, builder.Build())
		}
		return false, nil
	}

	if !p.tryLock(ctx, msg.GroupId) {
		builder := msgchain.Builder().Group()
		builder.Text("正在等待响应中，不要着急哦~")
		bot.SendGroupMsg(msg.GroupId, builder.Build())
		return true, nil
	}
	defer p.unLock(ctx, msg.GroupId)
	defer p.clearActiveContext(msg.GroupId)
	chat := p.getChat(msg.GroupId)
	if chat == nil {
		builder := msgchain.Builder().Group()
		builder.Text("无法创建对话，请检查日志信息哦")
		bot.SendGroupMsg(msg.GroupId, builder.Build())
		return true, nil
	}

	// 创建可取消的上下文
	chatCtx, cancel := context.WithCancel(ctx)
	p.setActiveContext(msg.GroupId, cancel)

	builder := msgchain.Builder().Group()
	extraText := p.extraMsg(ctx, bot, msg, p.ocrModel)
	if strings.Contains(extraText, "#新对话") {
		err := chat.ClearHistory(ctx)
		if err != nil {
			p.Logger.Error("无法清理AI聊天信息", "error", err)
			return false, nil
		} else {
			p.Logger.Info("清理AI对话信息成功")
		}

		// 清理动态加载的 MCP 工具
		if cleared := chat.ClearDynamicTools(); cleared > 0 {
			p.Logger.Info("清理动态加载的 MCP 工具", "count", cleared)
		}
	}

	msgFuncs := llmtool.CallBackFuncs{
		SendText: func(s string) (string, error) {
			builder := msgchain.Builder().Group()
			builder.Mention(msg.Sender.UserId)
			builder.Text(" " + s)
			_, success := bot.SendGroupMsg(msg.GroupId, builder.Build())
			if success {
				p.Logger.Info("发送文本", "group", msg.GroupId, "user", msg.Sender.UserId, "text", s)
			}
			return "发送成功", nil
		},
		SendImage: func(url string) (string, error) {
			builder := msgchain.Builder().Group()
			builder.ImageUrl(url)
			_, success := bot.SendGroupMsg(msg.GroupId, builder.Build())
			if success {
				p.Logger.Info("发送图片", "group", msg.GroupId, "user", msg.Sender.UserId, "image", url)
			}
			return "发送成功", nil
		},
		SendFile: func(fileName, content string) (string, error) {
			builder := msgchain.Builder().Group()
			builder.FileBase64(fileName, base64.StdEncoding.EncodeToString([]byte(content)))
			_, success := bot.SendGroupMsg(msg.GroupId, builder.Build())
			if success {
				p.Logger.Info("发送文件", "group", msg.GroupId, "user", msg.Sender.UserId, "file", fileName)
			}
			return "发送成功", nil
		},
	}

	resp, err := chat.Chat(chatCtx, extraText, msgFuncs,
		llms.WithMaxTokens(p.llmParameter.maxToken),
		llms.WithTemperature(p.llmParameter.temperature),
		llms.WithTopP(p.llmParameter.top_p),
		llms.WithTopK(p.llmParameter.top_k),
	)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			builder.Text("AI 响应已被停止")
			bot.SendGroupMsg(msg.GroupId, builder.Build())
			return true, nil
		}
		if errors.Is(err, context.DeadlineExceeded) {
			builder.Text("请求超时")
			bot.SendGroupMsg(msg.GroupId, builder.Build())
			return true, nil
		}
		builder.Text("无法解析的错误信息，请查看日志")
		p.Logger.Error("AI请求错误", "error", err.Error())
		bot.SendGroupMsg(msg.GroupId, builder.Build())
		return true, nil
	}

	if resp == "" {
		p.Logger.Info("AI请求没有返回什么东西")
		return true, nil
	}

	builder.Mention(msg.Sender.UserId)
	builder.Text(" " + resp)
	if _, success := bot.SendGroupMsg(msg.GroupId, builder.Build()); success {
		p.Logger.Info("发送文本", "group", msg.GroupId, "user", msg.Sender.UserId, "text", resp)
	}
	return true, nil
}

func (p *AIChatPlugin) OnFriendMsg(ctx context.Context, bot bot.Bot, cmd command.Command, msg message.Message) (bool, error) {
	// 处理 /stop 命令
	if cmd.Name == "stop" {
		if p.stopRequest(msg.Sender.UserId) {
			builder := msgchain.Builder().Friend()
			builder.Text("已停止 AI 响应")
			bot.SendFriendMsg(msg.Sender.UserId, builder.Build())
			p.Logger.Info("用户停止 AI 响应", "user", msg.Sender.UserId)
		} else {
			builder := msgchain.Builder().Friend()
			builder.Text("当前没有正在进行的 AI 请求")
			bot.SendFriendMsg(msg.Sender.UserId, builder.Build())
		}
		return false, nil
	}

	if !p.tryLock(ctx, msg.Sender.UserId) {
		builder := msgchain.Builder().Friend()
		builder.Text("正在等待响应中，不要着急哦~")
		bot.SendFriendMsg(msg.Sender.UserId, builder.Build())
		return true, nil
	}
	defer p.unLock(ctx, msg.Sender.UserId)
	defer p.clearActiveContext(msg.Sender.UserId)

	chat := p.getChat(msg.Sender.UserId)
	if chat == nil {
		builder := msgchain.Builder().Friend()
		builder.Text("无法创建对话，请检查日志信息哦")
		bot.SendFriendMsg(msg.Sender.UserId, builder.Build())
		return true, nil
	}

	// 创建可取消的上下文
	chatCtx, cancel := context.WithCancel(ctx)
	p.setActiveContext(msg.Sender.UserId, cancel)

	builder := msgchain.Builder().Friend()
	extraText := p.extraMsg(ctx, bot, msg, p.ocrModel)
	if strings.Contains(extraText, "#新对话") {
		err := chat.ClearHistory(ctx)
		if err != nil {
			p.Logger.Error("无法清理AI聊天信息", "error", err.Error())
			return false, nil
		} else {
			p.Logger.Info("清理AI对话信息成功")
		}

		// 清理动态加载的 MCP 工具
		if cleared := chat.ClearDynamicTools(); cleared > 0 {
			p.Logger.Info("清理动态加载的 MCP 工具", "count", cleared)
		}
	}

	msgFuncs := llmtool.CallBackFuncs{
		SendText: func(s string) (string, error) {
			builder := msgchain.Builder().Friend()
			builder.Text(s)
			_, success := bot.SendFriendMsg(msg.Sender.UserId, builder.Build())
			if success {
				p.Logger.Info("发送文本", "user", msg.Sender.UserId, "text", s)
			}
			return "发送成功", nil
		},
		SendImage: func(url string) (string, error) {
			builder := msgchain.Builder().Friend()
			builder.ImageUrl(url)
			_, success := bot.SendFriendMsg(msg.Sender.UserId, builder.Build())
			if success {
				p.Logger.Info("发送图片", "user", msg.Sender.UserId, "image", url)
			}
			return "发送成功", nil
		},
		SendFile: func(fileName, content string) (string, error) {
			builder := msgchain.Builder().Friend()
			builder.FileBase64(fileName, base64.StdEncoding.EncodeToString([]byte(content)))
			_, success := bot.SendFriendMsg(msg.Sender.UserId, builder.Build())
			if success {
				p.Logger.Info("发送文件", "user", msg.Sender.UserId, "file", fileName)
			}
			return "发送成功", nil
		},
	}

	resp, err := chat.Chat(chatCtx, extraText,
		msgFuncs,
		llms.WithMaxTokens(p.llmParameter.maxToken),
		llms.WithTemperature(p.llmParameter.temperature),
		llms.WithTopP(p.llmParameter.top_p),
		llms.WithTopK(p.llmParameter.top_k),
	)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			builder.Text("AI 响应已被停止")
			bot.SendFriendMsg(msg.Sender.UserId, builder.Build())
			return true, nil
		}
		if errors.Is(err, context.DeadlineExceeded) {
			builder.Text("请求超时")
			bot.SendFriendMsg(msg.Sender.UserId, builder.Build())
			return true, nil
		}
		builder.Text("无法解析的错误信息，请查看日志")
		p.Logger.Error("AI请求错误", "error", err.Error())
		bot.SendFriendMsg(msg.Sender.UserId, builder.Build())
		return true, nil
	}

	if resp == "" {
		p.Logger.Info("AI请求没有返回什么东西")
		return true, nil
	}

	builder.Text(resp)
	if _, success := bot.SendFriendMsg(msg.Sender.UserId, builder.Build()); success {
		p.Logger.Info("发送文本", "user", msg.Sender.UserId, "text", resp)
	}
	return true, nil
}

func (p *AIChatPlugin) Start(ctx context.Context, cfg *viper.Viper) error {
	p.lockStorage = p.Storage.Clone("lock")
	p.lockStorage.Clear(ctx)

	p.botConfig.baseURL = cfg.GetString("plugin.ai_chat_bot.base_url")
	p.botConfig.model = cfg.GetString("plugin.ai_chat_bot.model")
	p.botConfig.apiKey = cfg.GetString("plugin.ai_chat_bot.api_key")
	p.llmParameter.prompt = cfg.GetString("plugin.ai_chat_bot.prompt")

	if p.botConfig.baseURL == "" {
		p.Logger.Error("初始化失败：未配置 Base Url")
		return aniaerror.ParameterInitializeError
	}
	if p.botConfig.model == "" {
		p.Logger.Error("初始化失败：未配置 Model")
		return aniaerror.ParameterInitializeError
	}
	if p.botConfig.apiKey == "" {
		p.Logger.Error("初始化失败：未配置 API KEY")
		return aniaerror.ParameterInitializeError
	}
	if p.llmParameter.prompt == "" {
		p.Logger.Warn("未配置 Prompt，将使用预设的默认提示词")
		p.llmParameter.prompt = "你是一个ai对话机器人，在QQ上和别人聊天，说话不要长篇大论"
	}

	p.llmParameter.maxToken = cfg.GetInt("plugin.ai_chat_bot.max_token")
	p.llmParameter.temperature = cfg.GetFloat64("plugin.ai_chat_bot.temperature")
	p.llmParameter.top_p = cfg.GetFloat64("plugin.ai_chat_bot.top_p")
	p.llmParameter.top_k = cfg.GetInt("plugin.ai_chat_bot.top_k")

	p.ocrEnable = cfg.GetBool("plugin.ai_chat_bot.ocr.enable")
	if p.ocrEnable {
		p.Logger.Info("已启用OCR LLM")
		ocrBaseUrl := cfg.GetString("plugin.ai_chat_bot.ocr.base_url")
		ocrAPIKey := cfg.GetString("plugin.ai_chat_bot.ocr.api_key")
		ocrModel := cfg.GetString("plugin.ai_chat_bot.ocr.model")
		ocrPrompt := cfg.GetString("plugin.ai_chat_bot.ocr.prompt")
		searchToken := cfg.GetString("plugin.ai_chat_bot.search.token")

		p.ocrParameter.maxToken = cfg.GetInt("plugin.ai_chat_bot.ocr.max_token")
		p.ocrParameter.temperature = cfg.GetFloat64("plugin.ai_chat_bot.ocr.temperature")
		p.ocrParameter.top_p = cfg.GetFloat64("plugin.ai_chat_bot.ocr.top_p")
		p.ocrParameter.top_k = cfg.GetInt("plugin.ai_chat_bot.ocr.top_k")
		p.llmParameter.searchToken = searchToken

		ocrllm, err := aichat.NewChatBot(ocrBaseUrl, ocrAPIKey, ocrModel, ocrPrompt, 10, nil)
		if err != nil {
			p.Logger.Error("无法初始化OCR LLM", "error", err.Error())
			p.ocrEnable = false
		} else {
			p.ocrModel = ocrllm
		}
	}

	// 读取 MCP 服务器配置
	if err := p.loadMCPConfigs(cfg); err != nil {
		p.Logger.Warn("加载 MCP 配置失败", "error", err.Error())
	}

	// 创建共享的工具执行器（只创建一次，所有对话共享）
	p.Logger.Info("初始化工具执行器...")
	skillsDir := cfg.GetString("plugin.ai_chat_bot.skills_dir") // 配置项，默认 "./skills"
	if skillsDir == "" {
		skillsDir = "./skills"
	}
	p.toolExecutor, p.skillManager = functool.CreateToolsWithSkill(
		p.llmParameter.searchToken,
		p.mcpConfigs,
		skillsDir,
	)
	if p.toolExecutor == nil {
		p.Logger.Error("创建工具执行器失败")
		return aniaerror.ParameterInitializeError
	}
	p.Logger.Info("工具执行器初始化完成")

	return nil
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
			Command:     getStringFromMap(serverMap, "command"),
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

		// 读取 timeout
		if timeout, ok := serverMap["timeout"].(int); ok {
			mcpConfig.Timeout = time.Duration(timeout) * time.Second
		}

		// 验证配置
		if mcpConfig.Name == "" {
			p.Logger.Warn("MCP 服务器配置缺少名称", "index", i)
			continue
		}

		if mcpConfig.Command == "" {
			p.Logger.Warn("MCP 服务器配置缺少 command", "name", mcpConfig.Name)
			continue
		}

		p.mcpConfigs = append(p.mcpConfigs, mcpConfig)
		p.Logger.Info("已加载 MCP 服务器配置", "name", mcpConfig.Name, "command", mcpConfig.Command)
	}

	p.Logger.Info("MCP 服务器配置加载完成", "count", len(p.mcpConfigs))
	return nil
}

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
