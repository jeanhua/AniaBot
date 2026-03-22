package pluginaichat

import (
	"context"
	"errors"
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
	rateLimit   int
	rateCh      chan struct{}

	// 用于存储活跃的请求上下文，支持取消操作
	activeContexts sync.Map // map[message.QID]context.CancelFunc

	// 统计群聊未@次数
	noMentionCount sync.Map

	botConfig struct {
		baseURL string
		apiKey  string
		model   string
	}

	llmParameter struct {
		maxToken       *int
		temperature    *float64
		top_p          *float64
		top_k          *int
		prompt         string
		searchToken    string
		enableThinking bool
		thinkingMode   string
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

func (p *AIChatPlugin) OnGroupMsg(ctx context.Context, bot bot.Bot, cmd command.Command, msg message.Message) (bool, error) {
	if !cmd.Mention {
		cnt := 0
		if v, ok := p.noMentionCount.Load(msg.GroupId); ok {
			if iv, ok2 := v.(int); ok2 {
				cnt = iv
			}
		}
		cnt++
		p.noMentionCount.Store(msg.GroupId, cnt)
		if cnt > 30 {
			if c, ok := p.chats.Load(msg.GroupId); ok && c != nil {
				chat := c.(*aichat.ChatBot)
				chat.ClearHistory(ctx)
				p.Logger.Info("自动清理AI对话信息", "group", msg.GroupId, "reason", "超过30条未@消息")
				if cleared := chat.ClearDynamicTools(); cleared > 0 {
					p.Logger.Info("清理动态加载的 MCP 工具", "count", cleared)
				}
			}
			p.noMentionCount.Store(msg.GroupId, 0)
		}
		return true, nil
	}

	// 处理 /stop 命令
	if cmd.Name == "stop" {
		if p.stopRequest(msg.GroupId) {
			builder := msgchain.Builder().Group()
			builder.Text("用户停止AI响应")
			bot.SendGroupMsg(msg.GroupId, builder.Build())
			p.Logger.Info("用户停止 AI 响应", "group", msg.GroupId, "user", msg.Sender.UserId)
		} else {
			builder := msgchain.Builder().Group()
			builder.Text("当前没有正在进行的 AI 请求")
			bot.SendGroupMsg(msg.GroupId, builder.Build())
		}
		return false, nil
	}

	if !p.tryLock(msg.GroupId) {
		builder := msgchain.Builder().Group()
		builder.Text("正在等待响应中，不要着急哦~")
		bot.SendGroupMsg(msg.GroupId, builder.Build())
		return true, nil
	}
	defer p.unLock(msg.GroupId)
	defer p.clearActiveContext(msg.GroupId)
	p.noMentionCount.Store(msg.GroupId, 0)
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

	msgFuncs := MakeGroupCallback(bot, msg.GroupId, msg.Sender.UserId, p.Logger)

	chatOpts := p.thinkingOpts()
	if p.llmParameter.maxToken != nil {
		chatOpts = append(chatOpts, llms.WithMaxTokens(*p.llmParameter.maxToken))
	}
	if p.llmParameter.temperature != nil {
		chatOpts = append(chatOpts, llms.WithTemperature(*p.llmParameter.temperature))
	}
	if p.llmParameter.top_p != nil {
		chatOpts = append(chatOpts, llms.WithTopP(*p.llmParameter.top_p))
	}
	if p.llmParameter.top_k != nil {
		chatOpts = append(chatOpts, llms.WithTopK(*p.llmParameter.top_k))
	}
	resp, usage, err := chat.Chat(chatCtx, extraText, msgFuncs, chatOpts...)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			builder.Text("AI 响应已被停止")
			bot.SendGroupMsg(msg.GroupId, builder.Build())
			return false, nil
		} else if errors.Is(err, context.DeadlineExceeded) {
			builder.Text("请求超时")
			bot.SendGroupMsg(msg.GroupId, builder.Build())
			return false, nil
		}
		builder.Text("无法解析的错误信息，请查看日志")
		p.Logger.Error("AI请求错误", "error", err.Error())
		bot.SendGroupMsg(msg.GroupId, builder.Build())
		return false, nil
	}

	if len(strings.TrimSpace(resp)) == 0 {
		p.Logger.Info("AI请求没有返回什么东西")
		return true, nil
	}

	p.Logger.Info("AI请求token消耗", "group", msg.GroupId, "prompt_tokens", usage.PromptTokens, "completion_tokens", usage.CompletionTokens, "total_tokens", usage.TotalTokens)
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
			builder.Text("用户停止AI响应")
			bot.SendFriendMsg(msg.Sender.UserId, builder.Build())
			p.Logger.Info("用户停止 AI 响应", "user", msg.Sender.UserId)
		} else {
			builder := msgchain.Builder().Friend()
			builder.Text("当前没有正在进行的 AI 请求")
			bot.SendFriendMsg(msg.Sender.UserId, builder.Build())
		}
		return false, nil
	}

	if !p.tryLock(msg.Sender.UserId) {
		builder := msgchain.Builder().Friend()
		builder.Text("正在等待响应中，不要着急哦~")
		bot.SendFriendMsg(msg.Sender.UserId, builder.Build())
		return true, nil
	}
	defer p.unLock(msg.Sender.UserId)
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

	msgFuncs := MakeFriendCallback(bot, msg.Sender.UserId, p.Logger)

	friendOpts := p.thinkingOpts()
	if p.llmParameter.maxToken != nil {
		friendOpts = append(friendOpts, llms.WithMaxTokens(*p.llmParameter.maxToken))
	}
	if p.llmParameter.temperature != nil {
		friendOpts = append(friendOpts, llms.WithTemperature(*p.llmParameter.temperature))
	}
	if p.llmParameter.top_p != nil {
		friendOpts = append(friendOpts, llms.WithTopP(*p.llmParameter.top_p))
	}
	if p.llmParameter.top_k != nil {
		friendOpts = append(friendOpts, llms.WithTopK(*p.llmParameter.top_k))
	}
	resp, usage, err := chat.Chat(chatCtx, extraText,
		msgFuncs, friendOpts...)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			builder.Text("AI 响应已被停止")
			bot.SendFriendMsg(msg.Sender.UserId, builder.Build())
			return false, nil
		} else if errors.Is(err, context.DeadlineExceeded) {
			builder.Text("请求超时")
			bot.SendFriendMsg(msg.Sender.UserId, builder.Build())
			return false, nil
		}
		builder.Text("无法解析的错误信息，请查看日志")
		p.Logger.Error("AI请求错误", "error", err.Error())
		bot.SendFriendMsg(msg.Sender.UserId, builder.Build())
		return false, nil
	}

	if len(strings.TrimSpace(resp)) == 0 {
		p.Logger.Info("AI请求没有返回什么东西")
		return true, nil
	}

	p.Logger.Info("AI请求token消耗", "user", msg.Sender.UserId, "prompt_tokens", usage.PromptTokens, "completion_tokens", usage.CompletionTokens, "total_tokens", usage.TotalTokens)
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

	p.rateLimit = cfg.GetInt("plugin.ai_chat_bot.rate_limit")
	if p.rateLimit <= 0 {
		p.rateLimit = 2
	}
	p.rateCh = make(chan struct{}, p.rateLimit)

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

	if cfg.IsSet("plugin.ai_chat_bot.max_token") {
		v := cfg.GetInt("plugin.ai_chat_bot.max_token")
		p.llmParameter.maxToken = &v
	}
	if cfg.IsSet("plugin.ai_chat_bot.temperature") {
		v := cfg.GetFloat64("plugin.ai_chat_bot.temperature")
		p.llmParameter.temperature = &v
	}
	if cfg.IsSet("plugin.ai_chat_bot.top_p") {
		v := cfg.GetFloat64("plugin.ai_chat_bot.top_p")
		p.llmParameter.top_p = &v
	}
	if cfg.IsSet("plugin.ai_chat_bot.top_k") {
		v := cfg.GetInt("plugin.ai_chat_bot.top_k")
		p.llmParameter.top_k = &v
	}
	p.llmParameter.enableThinking = cfg.GetBool("plugin.ai_chat_bot.thinking.enable")
	p.llmParameter.thinkingMode = cfg.GetString("plugin.ai_chat_bot.thinking.mode")
	if p.llmParameter.thinkingMode == "" {
		p.llmParameter.thinkingMode = "auto"
	}
	if p.llmParameter.enableThinking {
		p.Logger.Info("已启用深度思考模式", "mode", p.llmParameter.thinkingMode)
	}

	if cfg.IsSet("plugin.ai_chat_bot.search.token") {
		p.llmParameter.searchToken = cfg.GetString("plugin.ai_chat_bot.search.token")
	} else {
		p.Logger.Warn("Jina AI Token 未设置，将无法使用网页浏览和搜索功能")
	}

	p.ocrEnable = cfg.GetBool("plugin.ai_chat_bot.ocr.enable")
	if p.ocrEnable {
		p.Logger.Info("已启用OCR LLM")
		ocrBaseUrl := cfg.GetString("plugin.ai_chat_bot.ocr.base_url")
		ocrAPIKey := cfg.GetString("plugin.ai_chat_bot.ocr.api_key")
		ocrModel := cfg.GetString("plugin.ai_chat_bot.ocr.model")
		ocrPrompt := cfg.GetString("plugin.ai_chat_bot.ocr.prompt")

		p.ocrParameter.maxToken = cfg.GetInt("plugin.ai_chat_bot.ocr.max_token")
		p.ocrParameter.temperature = cfg.GetFloat64("plugin.ai_chat_bot.ocr.temperature")
		p.ocrParameter.top_p = cfg.GetFloat64("plugin.ai_chat_bot.ocr.top_p")
		p.ocrParameter.top_k = cfg.GetInt("plugin.ai_chat_bot.ocr.top_k")

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
