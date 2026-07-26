package pluginaichat

import (
	"context"
	"fmt"
	"strings"

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
// 让 AI 明确自己处于群聊还是私聊，以及消息 [nickname:昵称 id:QQ号] 前缀中
// id 的含义（用于 @人、填写定时任务 created_by 等场景）。
func (p *AIChatPlugin) buildScenePrompt(b bot.Bot, id message.QID, isGroup bool) string {
	var sb strings.Builder
	sb.WriteString("\n\n【当前对话场景】\n")
	if isGroup {
		sb.WriteString("你正在一个QQ群聊中与多位成员对话，群号：" + id.String())
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
		sb.WriteString("你正在与一位QQ用户私聊，对方QQ：" + id.String())
	}
	sb.WriteString("\n用户消息以 [nickname:昵称 id:QQ号] 开头，id 即该发言者的QQ号。")
	if p.memoryManager != nil {
		sb.WriteString("\n\n【长期记忆】你拥有跨会话的长期记忆能力：对话中得知的用户称呼/偏好/重要信息、群里的约定或值得记住的事件，应主动调用 memory_save 保存；当对话涉及过去的事情或你不确定的背景时，先调用 memory_search 回忆；记忆有误或用户要求忘记时用 memory_forget 删除。记忆仅在当前会话内可见。")
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
		sessionDesc := "私聊（对方QQ " + id.String() + "）"
		if isGroup {
			scope = "g:" + id.String()
			sessionDesc = "群聊（群号 " + id.String() + "）"
		}
		for _, tool := range newMemoryTools(p.memoryManager, scope, sessionDesc) {
			sessionExecutor.RegisterSession(tool)
		}
	}
}

func (p *AIChatPlugin) getChat(b bot.Bot, id message.QID, isGroup bool, prompt string) *aichat.ChatBot {
	key := sessionKey(id, isGroup)
	chat, ok := p.chats.Load(key)
	if !ok {
		// 每个会话创建独立的 SessionToolExecutor，动态加载的工具互不影响
		sessionExecutor := p.toolExecutor.NewSessionExecutor()
		p.registerScopedTools(sessionExecutor, id, isGroup)
		// 注册子代理委派工具（仅主会话；子代理的一次性会话不注册，防止递归委派）
		if p.cfg.Subagent.Enable {
			for _, tool := range newSubagentTools(p, b, id, isGroup) {
				sessionExecutor.RegisterSession(tool)
			}
		}
		// 在 system prompt 末尾注入当前对话场景（群聊/私聊、群信息、消息 id 前缀含义）
		prompt += p.buildScenePrompt(b, id, isGroup)
		// 每个会话独立的历史持久化存储；g:/f: 前缀避免群聊与好友 id 相同导致历史串扰
		var historyStore aichat.HistoryStore
		if p.PersistentStorage != nil {
			histKey := "chat:" + key
			// 旧版历史键不带 g:/f: 前缀，首次访问时迁移到新键
			migrateLegacyHistory(p.PersistentStorage, "chat:"+id.String(), histKey)
			historyStore = newPersistentHistoryStore(p.PersistentStorage, histKey, p.Logger)
		}
		c, err := aichat.NewChatBot(
			p.cfg.BaseURL,
			p.cfg.APIKey,
			p.cfg.Model,
			prompt,
			p.cfg.MaxContextTokens,
			sessionExecutor,
			historyStore,
		)
		if err != nil {
			p.Logger.Error("创建 ChatBot 失败", "error", err.Error())
			return nil
		}
		// 注入 SkillManager，让 system prompt 包含 available_skills
		if p.skillManager != nil {
			c.SetSkillManager(p.skillManager)
		}
		// 回放持久化的历史，使对话跨重启延续
		c.LoadHistory(context.Background())
		p.chats.Store(key, c)
		return c
	}
	return chat.(*aichat.ChatBot)
}
