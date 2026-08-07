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
)

// stopRequest 停止指定会话的 AI 请求
func (p *AIChatPlugin) stopRequest(id message.QID, isGroup bool) bool {
	if cancel, ok := p.activeContexts.LoadAndDelete(sessionKey(id, isGroup)); ok {
		cancel.(context.CancelFunc)()
		return true
	}
	return false
}

// setActiveContext 设置活跃的请求上下文
func (p *AIChatPlugin) setActiveContext(id message.QID, isGroup bool, cancel context.CancelFunc) {
	p.activeContexts.Store(sessionKey(id, isGroup), cancel)
}

// clearActiveContext 清除活跃的请求上下文
func (p *AIChatPlugin) clearActiveContext(id message.QID, isGroup bool) {
	p.activeContexts.Delete(sessionKey(id, isGroup))
}

// buildScenePrompt 生成当前对话场景描述，注入到 system prompt 末尾，
// 让 AI 明确自己处于群聊还是私聊，以及消息 [nickname:昵称 id:用户ID] 前缀中
// id 的含义（用于 @人、填写定时任务 created_by 等场景）。
func (p *AIChatPlugin) buildScenePrompt(b bot.Bot, id message.QID, isGroup bool) string {
	var sb strings.Builder
	sb.WriteString("\n\n【当前对话场景】\n")
	if isGroup {
		sb.WriteString("你正在一个群聊中与多位成员对话，会话 ID：" + id.String())
		if b != nil {
			if info, ok := b.GetGroupDetail(id); ok && info != nil {
				if info.GroupName != "" {
					sb.WriteString("，群名：" + info.GroupName)
				}
				if info.MemberCount > 0 {
					sb.WriteString(fmt.Sprintf("，成员 %d 人", info.MemberCount))
				}
			}
		}
	} else {
		sb.WriteString("你正在与一位用户私聊，对方 ID：" + id.String())
	}
	sb.WriteString("\n用户消息以 [nickname:昵称 id:用户ID] 开头，id 即该发言者在当前平台的用户 ID。")
	if p.memoryManager != nil {
		sb.WriteString("\n\n【长期记忆】你拥有跨会话的长期记忆能力：对话中得知的用户称呼/偏好/重要信息、群里的约定或值得记住的事件，应主动调用 memory_save 保存；当对话涉及过去的事情或你不确定的背景时，先调用 memory_search 回忆；记忆有误或用户要求忘记时用 memory_forget 删除。记忆仅在当前会话内可见。")
	}
	if p.knowledgeManager != nil {
		sb.WriteString("\n\n【知识库】你拥有知识库检索能力：遇到需要查资料、或用户询问知识库中应有文档的问题时，先调用 kb_search 检索；若用户明确提供了值得长期保存的完整资料/教程，可调用 kb_add 记入当前会话的知识库。知识库含当前会话库与全局库，回答引用知识库内容时可说明出处。")
	}
	return sb.String()
}

// registerScopedTools 注册绑定当前会话的工具（定时任务 + 长期记忆）。
// 主会话与子代理的一次性会话共用（子代理拥有与主 AI 一致的会话级能力），
// 但不包含 subagent 工具自身——防止子代理递归委派。
func (p *AIChatPlugin) registerScopedTools(sessionExecutor *llmtool.SessionToolExecutor, id message.QID, isGroup bool) {
	// 注册定时任务管理工具，默认触发对象为当前会话（群聊/好友）
	if p.clockManager != nil {
		targetType := clockTargetFriend
		if isGroup {
			targetType = clockTargetGroup
		}
		for _, tool := range newClockTools(p.clockManager, targetType, id.String()) {
			sessionExecutor.RegisterSession(tool)
		}
	}
	// 注册长期记忆工具，scope 绑定当前会话（群聊 g:群号 / 私聊 f:QQ号），
	// 从机制上保证记忆不会跨会话泄露
	if p.memoryManager != nil {
		scope := "f:" + id.String()
		sessionDesc := "私聊（对方 ID " + id.String() + "）"
		if isGroup {
			scope = "g:" + id.String()
			sessionDesc = "群聊（会话 ID " + id.String() + "）"
		}
		for _, tool := range newMemoryTools(p.memoryManager, scope, sessionDesc) {
			sessionExecutor.RegisterSession(tool)
		}
	}
	// 注册知识库工具，scope 绑定当前会话；知识库检索同时覆盖该会话库与全局库
	if p.knowledgeManager != nil {
		scope := "f:" + id.String()
		sessionDesc := "私聊（对方 ID " + id.String() + "）"
		if isGroup {
			scope = "g:" + id.String()
			sessionDesc = "群聊（会话 ID " + id.String() + "）"
		}
		for _, tool := range newKnowledgeTools(p.knowledgeManager, scope, sessionDesc) {
			sessionExecutor.RegisterSession(tool)
		}
	}
	// 注册 skill 管理工具（skill_list / skill_install / skill_remove）：让 AI
	// 无需后台面板即可自行安装/卸载技能（配置门控，默认关闭）
	if p.cfg.SkillTool.Enable {
		for _, tool := range newSkillTools(p) {
			sessionExecutor.RegisterSession(tool)
		}
	}
	// 注册 MCP 管理工具（mcp_list / mcp_add / mcp_remove / mcp_reconnect）：让 AI
	// 自行管理 MCP 服务器，配置写入 files.mcp_json 持久化并即时热注册生效
	// （配置门控，默认关闭）
	if p.cfg.MCPTool.Enable {
		for _, tool := range newMCPTools(p) {
			sessionExecutor.RegisterSession(tool)
		}
	}
}

func (p *AIChatPlugin) mainMaxIterations() int {
	if p.cfg.MaxIterations <= 0 {
		return 20
	}
	return p.cfg.MaxIterations
}

// llmClientOptions 从插件配置构造 LLM 客户端可选参数（应用层重试 + 备用模型）。
// 供主对话 / 子代理 / 定时任务 / OCR 的所有客户端统一使用。
func (p *AIChatPlugin) llmClientOptions() []aichat.LLMClientOption {
	var opts []aichat.LLMClientOption
	if p.cfg.Retry.MaxAttempts > 1 {
		baseDelay := time.Duration(p.cfg.Retry.BaseDelaySec) * time.Second
		if baseDelay <= 0 {
			baseDelay = 2 * time.Second
		}
		opts = append(opts, aichat.WithRetry(p.cfg.Retry.MaxAttempts, baseDelay))
	}
	// Prompt 缓存：仅 anthropic 格式生效（cache_control 断点），
	// chat_completions / responses 为自动前缀缓存，无需配置
	opts = append(opts, aichat.WithPromptCache(aichat.PromptCacheConfig{
		Enable: p.cfg.PromptCache.Enable,
		TTL:    p.cfg.PromptCache.TTL,
	}))
	if p.cfg.Fallback.Model != "" {
		opts = append(opts, aichat.WithFallback(p.cfg.Fallback.BaseURL, p.cfg.Fallback.APIKey, p.cfg.Fallback.Model, p.cfg.Fallback.APIFormat))
	}
	return opts
}

// subagentLLMConfig 子代理模型配置：留空字段回退主模型配置。
func (p *AIChatPlugin) subagentLLMConfig() (baseURL, apiKey, model, format string) {
	baseURL, apiKey, model, format = p.cfg.BaseURL, p.cfg.APIKey, p.cfg.Model, p.cfg.APIFormat
	if p.cfg.Subagent.BaseURL != "" {
		baseURL = p.cfg.Subagent.BaseURL
	}
	if p.cfg.Subagent.APIKey != "" {
		apiKey = p.cfg.Subagent.APIKey
	}
	if p.cfg.Subagent.Model != "" {
		model = p.cfg.Subagent.Model
	}
	if p.cfg.Subagent.APIFormat != "" {
		format = p.cfg.Subagent.APIFormat
	}
	return baseURL, apiKey, model, format
}

// compressorLLMConfig 压缩器模型配置：留空字段回退主模型配置。
func (p *AIChatPlugin) compressorLLMConfig() (baseURL, apiKey, model, format string) {
	baseURL, apiKey, model, format = p.cfg.BaseURL, p.cfg.APIKey, p.cfg.Model, p.cfg.APIFormat
	if p.cfg.Compressor.BaseURL != "" {
		baseURL = p.cfg.Compressor.BaseURL
	}
	if p.cfg.Compressor.APIKey != "" {
		apiKey = p.cfg.Compressor.APIKey
	}
	if p.cfg.Compressor.Model != "" {
		model = p.cfg.Compressor.Model
	}
	if p.cfg.Compressor.APIFormat != "" {
		format = p.cfg.Compressor.APIFormat
	}
	return baseURL, apiKey, model, format
}

// buildCompressorClient 构造上下文压缩专用 LLM 客户端；配置三字段全空时
// 返回 nil（压缩复用主对话客户端）。构造失败仅记日志并返回 nil 降级。
func (p *AIChatPlugin) buildCompressorClient() *aichat.LLMClient {
	if p.cfg.Compressor.BaseURL == "" && p.cfg.Compressor.APIKey == "" && p.cfg.Compressor.Model == "" {
		return nil
	}
	baseURL, apiKey, model, format := p.compressorLLMConfig()
	client, err := aichat.NewLLMClient(baseURL, apiKey, model,
		append(p.llmClientOptions(), aichat.WithAPIFormat(format))...)
	if err != nil {
		p.Logger.Error("创建压缩器 LLM 客户端失败，压缩将复用主对话模型", "error", err.Error())
		return nil
	}
	return client
}

func (p *AIChatPlugin) getChat(b bot.Bot, id message.QID, isGroup bool, prompt string) *aichat.ChatBot {
	key := sessionKey(id, isGroup)
	if v, ok := p.chats.Load(key); ok {
		e := v.(*chatEntry)
		e.lastActive.Store(time.Now().Unix())
		return e.chat
	}
	// 会话未驻留（首次发言或被淘汰后）：重新创建并从持久层回放历史
	{
		// 每个会话创建独立的 SessionToolExecutor，动态加载的工具互不影响
		sessionExecutor := p.toolExecutor.NewSessionExecutor()
		p.registerScopedTools(sessionExecutor, id, isGroup)
		// 注册子代理委派工具（仅主会话；子代理的一次性会话不注册，防止递归委派）
		if p.cfg.Subagent.Enable {
			for _, tool := range newSubagentTools(p, b, id, isGroup) {
				sessionExecutor.RegisterSession(tool)
			}
		}
		// 注册 Agent 团队工具（仅主会话；团队成员的一次性会话经 registerScopedTools
		// 不注册团队工具，防止递归组建团队）
		if p.teamManager != nil {
			for _, tool := range newTeamTools(p, b, id, isGroup) {
				sessionExecutor.RegisterSession(tool)
			}
		}
		// 在 system prompt 末尾注入当前对话场景（群聊/私聊、群信息、消息 id 前缀含义）
		prompt += p.buildScenePrompt(b, id, isGroup)
		// 每个会话独立的历史持久化存储；g:/f: 前缀避免群聊与好友 id 相同导致历史串扰。
		// SQL 后端走行级存储（ania_chat_session/ania_chat_message），否则回退 KV 整段 JSON
		var historyStore aichat.HistoryStore
		switch {
		case p.historyDB != nil:
			historyStore = newSQLHistoryStore(p.historyDB, key, p.Logger)
		case p.PersistentStorage != nil:
			historyStore = newPersistentHistoryStore(p.PersistentStorage, "chat:"+key, p.Logger)
		}
		c, err := aichat.NewChatBot(
			p.cfg.BaseURL,
			p.cfg.APIKey,
			p.cfg.Model,
			prompt,
			p.cfg.MaxContextTokens,
			sessionExecutor,
			historyStore,
			aichat.WithClientOptions(append(p.llmClientOptions(), aichat.WithAPIFormat(p.cfg.APIFormat))...),
			aichat.WithCompressorClient(p.buildCompressorClient()),
		)
		if err != nil {
			p.Logger.Error("创建 ChatBot 失败", "error", err.Error())
			return nil
		}
		c.SetMaxIterations(p.mainMaxIterations())
		// 注入 SkillManager，让 system prompt 包含 available_skills
		if p.skillManager != nil {
			c.SetSkillManager(p.skillManager)
		}
		// 回放持久化的历史，使对话跨重启延续
		c.LoadHistory(context.Background())
		p.chats.Store(key, newChatEntry(c, id, isGroup))
		return c
	}
}
