package pluginaichat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jeanhua/AniaBot/bot/component/aichat"
	"github.com/jeanhua/AniaBot/bot/component/functool"
	"github.com/jeanhua/AniaBot/bot/component/llmtool"
	"github.com/jeanhua/AniaBot/bot/component/querylog"
	"github.com/jeanhua/AniaBot/common/aniaerror"
	"github.com/jeanhua/AniaBot/common/bot"
	"github.com/jeanhua/AniaBot/common/model/command"
	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/jeanhua/AniaBot/common/msgchain"
	"github.com/jeanhua/AniaBot/common/plugin"
	"github.com/jeanhua/AniaBot/common/plugininfo"
	"github.com/jeanhua/AniaBot/common/storage"
	"github.com/spf13/viper"
)

type AIChatPlugin struct {
	plugin.Meta
	// cfg 插件配置，由框架在 Start 前自动填充（见 ConfigSchema）
	cfg   aiChatConfig
	chats sync.Map

	lockStorage storage.Storage
	rateCh      chan struct{}

	activeContexts sync.Map

	// pendingMsgs AI 响应期间到达的消息排队队列，按会话（群/好友）隔离
	pendingMsgs sync.Map

	noMentionCount sync.Map

	// 按群聊/好友独立的 prompt 覆盖配置
	promptOverrides struct {
		groups  map[message.QID]string
		friends map[message.QID]string
	}

	// ocrModel 备用图片识别模型；为 nil 表示未启用或初始化失败
	ocrModel *aichat.ChatBot

	mcpConfigs   []*llmtool.MCPConfig
	toolExecutor *llmtool.ToolExecuter

	skillManager *llmtool.SkillManager
	// skillsDir skill 所在目录（供面板管理接口使用）
	skillsDir string

	// clockManager AI 定时任务调度器；为 nil 表示功能未启用
	clockManager *clockManager

	// memoryManager 长期记忆管理器；为 nil 表示功能未启用
	memoryManager *memoryManager

	// queryLogger Query 日志记录器（面板「Query 日志」页数据源）；为 nil 表示功能未启用
	queryLogger *querylog.Logger
}

const (
	LockExpTime     = time.Minute * 10
	promptConfigKey = "files.prompt_json"
)

type promptOverrideConfig struct {
	Groups  map[string]string `json:"groups"`
	Friends map[string]string `json:"friends"`
}

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
	if cmd.Name == "clock" {
		return p.handleClockCommand(ctx, bot, cmd, msg)
	}

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
			if c, ok := p.chats.Load(sessionKey(msg.GroupId, true)); ok && c != nil {
				chat := c.(*aichat.ChatBot)
				// 与 mention 路径的 chat.Chat 共用 per-group 锁，避免自动清理与进行中的对话
				// 并发访问 messageWindow.messages 及 SessionToolExecutor.sessionTools
				// （并发 map 读写会触发不可恢复的 fatal error 导致整个进程崩溃）
				if p.tryLock(msg.GroupId, true) {
					defer p.unLock(msg.GroupId, true)
					chat.ClearHistory(ctx)
					p.Logger.Info("自动清理AI对话信息", "group", msg.GroupId, "reason", "超过30条未@消息")
					if cleared := chat.ClearDynamicTools(); cleared > 0 {
						p.Logger.Info("清理动态加载的 MCP 工具", "count", cleared)
					}
				}
			}
			p.noMentionCount.Store(msg.GroupId, 0)
		}
		return true, nil
	}

	if cmd.Name == "stop" {
		// 停止当前请求的同时丢弃排队消息，避免停止后又自动回复
		p.drainPending(msg.GroupId, true)
		if p.stopRequest(msg.GroupId, true) {
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

	if !p.tryLock(msg.GroupId, true) {
		// 当前正在响应：消息进入排队队列，响应结束后自动合并处理
		first, ok := p.enqueuePending(msg.GroupId, true, msg)
		if !ok {
			builder := msgchain.Builder().Group()
			builder.Text("排队消息太多啦，稍后再试试吧~")
			bot.SendGroupMsg(msg.GroupId, builder.Build())
		} else if first {
			builder := msgchain.Builder().Group()
			builder.Text("正在回复上一条消息，你的消息已排队，稍后回复你~")
			bot.SendGroupMsg(msg.GroupId, builder.Build())
		}
		return true, nil
	}
	defer p.unLock(msg.GroupId, true)
	defer p.clearActiveContext(msg.GroupId, true)
	p.noMentionCount.Store(msg.GroupId, 0)
	chat := p.getChat(bot, msg.GroupId, true, p.getPromptForID(msg.GroupId, true))
	if chat == nil {
		builder := msgchain.Builder().Group()
		builder.Text("无法创建对话，请检查日志信息哦")
		bot.SendGroupMsg(msg.GroupId, builder.Build())
		return true, nil
	}

	chatCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	p.setActiveContext(msg.GroupId, true, cancel)

	// 先取出可能遗留的排队消息（上一响应结束瞬间到达、未来得及处理的），与本次消息合并；
	// 之后每轮响应结束继续排空队列，直到没有新消息为止
	batch := append(p.drainPending(msg.GroupId, true), msg)
	for len(batch) > 0 {
		if !p.processChatBatch(chatCtx, bot, msg.GroupId, true, chat, batch) {
			return false, nil
		}
		batch = p.drainPending(msg.GroupId, true)
	}
	return true, nil
}

func (p *AIChatPlugin) OnFriendMsg(ctx context.Context, bot bot.Bot, cmd command.Command, msg message.Message) (bool, error) {
	if cmd.Name == "clock" {
		return p.handleClockCommand(ctx, bot, cmd, msg)
	}

	if cmd.Name == "stop" {
		// 停止当前请求的同时丢弃排队消息，避免停止后又自动回复
		p.drainPending(msg.Sender.UserId, false)
		if p.stopRequest(msg.Sender.UserId, false) {
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

	if !p.tryLock(msg.Sender.UserId, false) {
		// 当前正在响应：消息进入排队队列，响应结束后自动合并处理
		first, ok := p.enqueuePending(msg.Sender.UserId, false, msg)
		if !ok {
			builder := msgchain.Builder().Friend()
			builder.Text("排队消息太多啦，稍后再试试吧~")
			bot.SendFriendMsg(msg.Sender.UserId, builder.Build())
		} else if first {
			builder := msgchain.Builder().Friend()
			builder.Text("正在回复上一条消息，你的消息已排队，稍后回复你~")
			bot.SendFriendMsg(msg.Sender.UserId, builder.Build())
		}
		return true, nil
	}
	defer p.unLock(msg.Sender.UserId, false)
	defer p.clearActiveContext(msg.Sender.UserId, false)

	chat := p.getChat(bot, msg.Sender.UserId, false, p.getPromptForID(msg.Sender.UserId, false))
	if chat == nil {
		builder := msgchain.Builder().Friend()
		builder.Text("无法创建对话，请检查日志信息哦")
		bot.SendFriendMsg(msg.Sender.UserId, builder.Build())
		return true, nil
	}

	chatCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	p.setActiveContext(msg.Sender.UserId, false, cancel)

	// 先取出可能遗留的排队消息（上一响应结束瞬间到达、未来得及处理的），与本次消息合并；
	// 之后每轮响应结束继续排空队列，直到没有新消息为止
	batch := append(p.drainPending(msg.Sender.UserId, false), msg)
	for len(batch) > 0 {
		if !p.processChatBatch(chatCtx, bot, msg.Sender.UserId, false, chat, batch) {
			return false, nil
		}
		batch = p.drainPending(msg.Sender.UserId, false)
	}
	return true, nil
}

// processChatBatch 处理一批消息：可能只有一条（直接触发），也可能包含 AI 响应期间
// 排队的多条消息。多条时合并为一轮请求，让 AI 一次性回应。返回 false 表示应终止
// 处理循环（请求被取消或出错，已通知用户并丢弃剩余排队消息）。
func (p *AIChatPlugin) processChatBatch(ctx context.Context, b bot.Bot, id message.QID, isGroup bool, chat *aichat.ChatBot, batch []message.Message) bool {
	lastMsg := batch[len(batch)-1]

	// 合并本批消息文本；多条时加引导说明，让 AI 知道这些是响应期间积攒的消息
	var extraText string
	if len(batch) == 1 {
		extraText = p.extraMsg(b, lastMsg)
	} else {
		var sb strings.Builder
		fmt.Fprintf(&sb, "【以下是 %d 条在你响应期间收到的消息，请逐一回应】\n", len(batch))
		for i := range batch {
			sb.WriteString(p.extraMsg(b, batch[i]))
			sb.WriteString("\n")
		}
		extraText = sb.String()
	}

	if strings.Contains(extraText, "#新对话") {
		if err := chat.ClearHistory(ctx); err != nil {
			p.Logger.Error("无法清理AI聊天信息", "error", err)
			return false
		}
		p.Logger.Info("清理AI对话信息成功")

		if cleared := chat.ClearDynamicTools(); cleared > 0 {
			p.Logger.Info("清理动态加载的 MCP 工具", "count", cleared)
		}
	}

	var msgFuncs llmtool.CallBackFuncs
	if isGroup {
		msgFuncs = MakeGroupCallback(b, id, lastMsg.Sender.UserId, p.Logger)
	} else {
		msgFuncs = MakeFriendCallback(b, id, p.Logger)
	}
	p.configureImageCallbacks(ctx, b, &msgFuncs, batch...)

	chatOpts := p.buildChatOptions()
	recorder := p.beginQuery(chat, id, isGroup, batch, extraText)
	resp, usage, err := chat.Chat(ctx, extraText, msgFuncs, chatOpts)
	p.finishQuery(recorder, chat, usage, resp, err)
	if err != nil {
		// 出错或取消时丢弃剩余排队消息，避免连续报错刷屏
		p.drainPending(id, isGroup)
		switch {
		case errors.Is(err, context.Canceled):
			p.sendPlainText(b, id, isGroup, "AI 响应已被停止")
		case errors.Is(err, context.DeadlineExceeded):
			p.sendPlainText(b, id, isGroup, "请求超时")
		default:
			p.Logger.Error("AI请求错误", "error", err.Error())
			p.sendPlainText(b, id, isGroup, "无法解析的错误信息，请查看日志")
		}
		return false
	}

	if len(strings.TrimSpace(resp)) == 0 {
		p.Logger.Info("AI请求没有返回什么东西")
		return true
	}

	p.Logger.Info("AI请求token消耗", "id", id, "is_group", isGroup, "batch", len(batch), "prompt_tokens", usage.PromptTokens, "completion_tokens", usage.CompletionTokens, "total_tokens", usage.TotalTokens)

	if isGroup {
		// 群聊中 @ 本批所有发言者（去重），让排队消息的每个人都收到回应
		builder := msgchain.Builder().Group()
		seen := make(map[message.QID]struct{}, len(batch))
		for i := range batch {
			uid := batch[i].Sender.UserId
			if _, ok := seen[uid]; ok {
				continue
			}
			seen[uid] = struct{}{}
			builder.Mention(uid)
		}
		builder.Text(" " + resp)
		if _, success := b.SendGroupMsg(id, builder.Build()); success {
			p.Logger.Info("发送文本", "group", id, "batch", len(batch), "text", resp)
		}
	} else {
		builder := msgchain.Builder().Friend()
		builder.Text(resp)
		if _, success := b.SendFriendMsg(id, builder.Build()); success {
			p.Logger.Info("发送文本", "user", id, "batch", len(batch), "text", resp)
		}
	}
	return true
}

// sendPlainText 发送纯文本提示信息（不 @ 任何人）
func (p *AIChatPlugin) sendPlainText(b bot.Bot, id message.QID, isGroup bool, text string) {
	if isGroup {
		builder := msgchain.Builder().Group()
		builder.Text(text)
		b.SendGroupMsg(id, builder.Build())
	} else {
		builder := msgchain.Builder().Friend()
		builder.Text(text)
		b.SendFriendMsg(id, builder.Build())
	}
}

func (p *AIChatPlugin) buildChatOptions() aichat.ChatOptions {
	opts := p.thinkingOpts()
	if p.cfg.MaxToken != nil {
		opts.MaxToken = p.cfg.MaxToken
	}
	if p.cfg.Temperature != nil {
		opts.Temperature = p.cfg.Temperature
	}
	if p.cfg.TopP != nil {
		opts.TopP = p.cfg.TopP
	}
	if p.cfg.TopK != nil {
		opts.TopK = p.cfg.TopK
	}
	return opts
}

func (p *AIChatPlugin) buildOCRChatOptions() aichat.ChatOptions {
	opts := aichat.ChatOptions{}
	if p.cfg.OCR.MaxToken != nil {
		opts.MaxToken = p.cfg.OCR.MaxToken
	}
	if p.cfg.OCR.Temperature != nil {
		opts.Temperature = p.cfg.OCR.Temperature
	}
	if p.cfg.OCR.TopP != nil {
		opts.TopP = p.cfg.OCR.TopP
	}
	if p.cfg.OCR.TopK != nil {
		opts.TopK = p.cfg.OCR.TopK
	}
	return opts
}

func (p *AIChatPlugin) Start(ctx context.Context, cfg *viper.Viper) error {
	p.lockStorage = p.Storage.Clone("lock")
	p.lockStorage.Clear(ctx)

	// 插件配置已由框架自动填充到 p.cfg（见 ConfigSchema），这里只做校验与兜底
	rateLimit := p.cfg.RateLimit
	if rateLimit <= 0 {
		rateLimit = 2
	}
	p.rateCh = make(chan struct{}, rateLimit)

	if p.cfg.BaseURL == "" {
		p.Logger.Error("初始化失败：未配置 Base Url")
		return aniaerror.ParameterInitializeError
	}
	if p.cfg.Model == "" {
		p.Logger.Error("初始化失败：未配置 Model")
		return aniaerror.ParameterInitializeError
	}
	if p.cfg.APIKey == "" {
		p.Logger.Error("初始化失败：未配置 API KEY")
		return aniaerror.ParameterInitializeError
	}
	if p.cfg.Prompt == "" {
		p.Logger.Warn("未配置 Prompt，将使用预设的默认提示词")
		p.cfg.Prompt = defaultPrompt
	}

	// 加载群聊/好友独立 prompt 覆盖配置（框架级共享键，仍走 viper）
	p.loadPromptOverrides(cfg)

	if p.cfg.Thinking.Mode == "" {
		p.cfg.Thinking.Mode = "auto"
	}
	if p.cfg.Thinking.Enable {
		p.Logger.Info("已启用深度思考模式", "mode", p.cfg.Thinking.Mode)
	}

	if p.cfg.Search.Token == "" {
		p.Logger.Warn("Jina AI Token 未设置，将无法使用网页浏览和搜索功能")
	}

	if p.cfg.Multimodal {
		p.Logger.Info("主对话模型已配置为支持多模态，将按需直接加载图片")
	}

	if p.cfg.OCR.Enable {
		p.Logger.Info("已启用备用图片识别 LLM")
		ocrllm, err := aichat.NewChatBot(p.cfg.OCR.BaseURL, p.cfg.OCR.APIKey, p.cfg.OCR.Model, p.cfg.OCR.Prompt, 0, nil, nil)
		if err != nil {
			p.Logger.Error("无法初始化备用图片识别 LLM", "error", err.Error())
		} else {
			p.ocrModel = ocrllm
		}
	}

	if err := p.loadMCPConfigs(cfg); err != nil {
		p.Logger.Warn("加载 MCP 配置失败", "error", err.Error())
	}

	p.Logger.Info("初始化工具执行器...")
	skillsDir := p.cfg.SkillsDir
	if skillsDir == "" {
		skillsDir = "./skills"
	}
	p.skillsDir = skillsDir
	bashConfig := functool.BashConfig{
		Enable:    p.cfg.Bash.Enable,
		Shell:     p.cfg.Bash.Shell,
		Env:       p.cfg.Bash.Env,
		Whitelist: p.cfg.Bash.Whitelist,
		Blacklist: p.cfg.Bash.Blacklist,
	}
	if bashConfig.Enable {
		p.Logger.Info("已启用bash工具", "shell", bashConfig.Shell, "whitelist", bashConfig.Whitelist, "blacklist", bashConfig.Blacklist)
	}
	fileConfig := functool.FileConfig{Enable: p.cfg.File.Enable}
	if fileConfig.Enable {
		p.Logger.Info("已启用file工具（可读取宿主机本地文件并发送，请注意安全风险）")
	}
	localImageConfig := functool.LocalImageConfig{Enable: p.cfg.LocalImage.Enable}
	if localImageConfig.Enable {
		p.Logger.Info("已启用local_image工具（可读取宿主机本地图片供AI查看，请注意安全风险）")
	}
	var err error
	p.toolExecutor, p.skillManager, err = functool.CreateToolsWithSkill(
		p.cfg.Search.Token,
		p.mcpConfigs,
		skillsDir,
		bashConfig,
		fileConfig,
		localImageConfig,
		p.cfg.Skills,
	)
	if err != nil {
		p.Logger.Error("创建工具执行器失败", "error", err.Error())
		return aniaerror.ParameterInitializeError
	}
	p.Logger.Info("工具执行器初始化完成")

	// AI 定时任务（clock）：AI / 用户动态管理的持久化定时任务，独立于框架 cron
	if p.cfg.Clock.Enable {
		defaultTimeoutSec := p.cfg.Clock.DefaultTimeoutSec
		if defaultTimeoutSec <= 0 {
			defaultTimeoutSec = 120
		}
		maxLog := p.cfg.Clock.MaxLogEntries
		if maxLog <= 0 {
			maxLog = 500
		}
		p.clockManager = newClockManager(p, time.Duration(defaultTimeoutSec)*time.Second, maxLog)
		p.Logger.Info("已启用AI定时任务功能", "tasks", len(p.clockManager.List()), "default_timeout_sec", defaultTimeoutSec)
	} else {
		p.Logger.Info("AI定时任务功能未启用（plugin.ai_chat_bot.clock.enable=false）")
	}

	// AI 长期记忆：由 AI 通过 memory_save/search/forget 工具自行管理的跨会话记忆，
	// 按群聊/好友 scope 隔离，持久化到 PersistentStorage（memory: 命名空间）
	if p.cfg.Memory.Enable {
		maxEntries := p.cfg.Memory.MaxEntries
		if maxEntries <= 0 {
			maxEntries = 200
		}
		p.memoryManager = newMemoryManager(p.PersistentStorage, p.Logger.WithGroup("memory"), maxEntries)
		p.Logger.Info("已启用AI长期记忆功能", "max_entries", maxEntries)
	} else {
		p.Logger.Info("AI长期记忆功能未启用（plugin.ai_chat_bot.memory.enable=false）")
	}

	// AI 子代理：主 AI 可通过 subagent_run 工具把复杂子任务委派给一次性子代理
	// （全新上下文 + 全部工具能力），执行结果返回给主 AI，避免污染主对话上下文
	if p.cfg.Subagent.Enable {
		p.Logger.Info("已启用子代理功能",
			"timeout_sec", p.subagentTimeout().Seconds(),
			"max_iterations", p.subagentMaxIterations(),
			"max_result_len", p.subagentMaxResultLen())
	} else {
		p.Logger.Info("子代理功能未启用（plugin.ai_chat_bot.subagent.enable=false）")
	}

	// Query 日志：记录每次 AI 回复的完整执行过程（面板「Query 日志」页数据源）
	p.initQueryLogger()

	return nil
}

// Awake Bot 启动完成后启动定时任务调度器。
func (p *AIChatPlugin) Awake(ctx context.Context, bot bot.Bot) error {
	if p.clockManager != nil {
		p.clockManager.Start(bot)
	}
	return nil
}

func (p *AIChatPlugin) loadPromptOverrides(cfg *viper.Viper) {
	p.promptOverrides.groups = make(map[message.QID]string)
	p.promptOverrides.friends = make(map[message.QID]string)

	raw := cfg.GetString(promptConfigKey)
	if strings.TrimSpace(raw) == "" {
		p.Logger.Info("未配置 Prompt 覆盖，跳过加载", "key", promptConfigKey)
		return
	}

	var overrideCfg promptOverrideConfig
	if err := json.Unmarshal([]byte(raw), &overrideCfg); err != nil {
		p.Logger.Warn("解析 Prompt 覆盖配置失败", "error", err.Error())
		return
	}

	for k, v := range overrideCfg.Groups {
		id, err := strconv.ParseUint(k, 10, 64)
		if err != nil {
			p.Logger.Warn("Prompt 覆盖配置: 无效的群聊ID", "id", k, "error", err.Error())
			continue
		}
		p.promptOverrides.groups[message.FromUint64(id)] = v
	}
	for k, v := range overrideCfg.Friends {
		id, err := strconv.ParseUint(k, 10, 64)
		if err != nil {
			p.Logger.Warn("Prompt 覆盖配置: 无效的好友ID", "id", k, "error", err.Error())
			continue
		}
		p.promptOverrides.friends[message.FromUint64(id)] = v
	}

	count := len(p.promptOverrides.groups) + len(p.promptOverrides.friends)
	if count > 0 {
		p.Logger.Info("已加载 Prompt 覆盖配置", "groups", len(p.promptOverrides.groups), "friends", len(p.promptOverrides.friends))
	}
}

func (p *AIChatPlugin) getPromptForID(id message.QID, isGroup bool) string {
	if isGroup {
		if prompt, ok := p.promptOverrides.groups[id]; ok {
			return prompt
		}
	} else {
		if prompt, ok := p.promptOverrides.friends[id]; ok {
			return prompt
		}
	}
	return p.cfg.Prompt
}
