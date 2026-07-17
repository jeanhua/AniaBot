package pluginaichat

import (
	"context"

	"github.com/jeanhua/AniaBot/bot/component/aichat"
	"github.com/jeanhua/AniaBot/common/model/message"
)

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

func (p *AIChatPlugin) getChat(id message.QID, isGroup bool, prompt string) *aichat.ChatBot {
	chat, ok := p.chats.Load(id)
	if !ok {
		// 每个会话创建独立的 SessionToolExecutor，动态加载的工具互不影响
		sessionExecutor := p.toolExecutor.NewSessionExecutor()
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
		// 每个会话独立的历史持久化存储；g:/f: 前缀避免群聊与好友 id 相同导致历史串扰
		var historyStore aichat.HistoryStore
		if p.PersistentStorage != nil {
			historyStore = newPersistentHistoryStore(p.PersistentStorage, "chat:"+id.String(), p.Logger)
		}
		c, err := aichat.NewChatBot(
			p.botConfig.baseURL,
			p.botConfig.apiKey,
			p.botConfig.model,
			prompt,
			p.botConfig.maxContextTokens,
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
		p.chats.Store(id, c)
		return c
	}
	return chat.(*aichat.ChatBot)
}
