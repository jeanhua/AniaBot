package pluginaichat

import (
	"context"
	"strconv"

	"github.com/jeanhua/AniaBot/bot/component/aichat"
	"github.com/jeanhua/AniaBot/common/model/message"
)

// noMentionPersistEvery 未@计数落盘间隔：每累计到该倍数才写一次持久层，
// 避免繁忙群里每条消息都触发一次 DB 写；达到阈值本身也会强制落盘，保证
// 重启后「已达阈值但会话未驻留未及清空」的群能在下次 @ 时补清。
const noMentionPersistEvery = 10

// noMentionKeyPrefix 未@计数持久化子命名空间（Clone 后键为群 QID 字符串，
// 如 "qq:123" / "tg:-100..."）。
const noMentionKeyPrefix = "nomention:"

// incrNoMention 自增某群的未@消息计数并返回新值：首次遇到该群时先从持久层
// 恢复历史计数（跨重启累计），随后按间隔/阈值落盘。恢复与自增在同一把锁内
// 完成，避免并发消息在恢复完成前先自增导致计数被覆盖丢失。
func (p *AIChatPlugin) incrNoMention(groupID message.QID) int {
	if p.cfg.Session.NoMentionClear <= 0 {
		return 0
	}
	p.noMentionMu.Lock()
	defer p.noMentionMu.Unlock()
	p.restoreNoMentionLocked(groupID)
	cnt := p.noMentionValueLocked(groupID) + 1
	p.noMentionCount.Store(groupID, cnt)
	p.persistNoMentionLocked(groupID, cnt)
	return cnt
}

// noMentionValue 读取某群的未@消息计数（首次读取时从持久层恢复）。
func (p *AIChatPlugin) noMentionValue(groupID message.QID) int {
	p.noMentionMu.Lock()
	defer p.noMentionMu.Unlock()
	p.restoreNoMentionLocked(groupID)
	return p.noMentionValueLocked(groupID)
}

func (p *AIChatPlugin) noMentionValueLocked(groupID message.QID) int {
	if v, ok := p.noMentionCount.Load(groupID); ok {
		if iv, ok2 := v.(int); ok2 {
			return iv
		}
	}
	return 0
}

// resetNoMention 清零某群的未@计数并删除持久化记录（@ 一次或成功清空后调用）。
func (p *AIChatPlugin) resetNoMention(groupID message.QID) {
	p.noMentionMu.Lock()
	defer p.noMentionMu.Unlock()
	p.noMentionCount.Store(groupID, 0)
	if p.noMentionStore != nil {
		p.noMentionStore.Del(context.Background(), groupID.String())
	}
}

// persistNoMentionLocked 按 checkpoint 间隔落盘；达到阈值也强制落盘。
// 调用方需持有 noMentionMu。
func (p *AIChatPlugin) persistNoMentionLocked(groupID message.QID, cnt int) {
	if p.noMentionStore == nil || cnt <= 0 {
		return
	}
	if cnt%noMentionPersistEvery != 0 && cnt < p.cfg.Session.NoMentionClear {
		return
	}
	p.noMentionStore.SetString(context.Background(), groupID.String(), strconv.Itoa(cnt))
}

// restoreNoMentionLocked 每个群首次计数前从持久层恢复历史计数，使计数跨重启。
// 调用方需持有 noMentionMu；每个群只查一次 DB（noMentionLoaded 标记）。
func (p *AIChatPlugin) restoreNoMentionLocked(groupID message.QID) {
	if p.noMentionStore == nil {
		return
	}
	if _, ok := p.noMentionLoaded.LoadOrStore(groupID, struct{}{}); ok {
		return
	}
	if s, ok := p.noMentionStore.GetString(context.Background(), groupID.String()); ok {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			p.noMentionCount.Store(groupID, n)
		}
	}
}

// tryClearNoMentionChat 未@计数达到阈值后的清理入口（未 @ 消息路径调用，此时
// 会话锁未持有）：会话已驻留且拿到会话锁时立即清空历史并返回 true（调用方清零
// 计数）；会话未驻留（重启后尚未创建、被闲置回收淘汰）或 AI 正在响应拿不到锁时
// 返回 false，调用方保留计数，待下次 @ 或会话重建后由 applyNoMentionClear 补清。
func (p *AIChatPlugin) tryClearNoMentionChat(ctx context.Context, groupID message.QID) bool {
	v, ok := p.chats.Load(sessionKey(groupID, true))
	if !ok {
		return false
	}
	e, ok2 := v.(*chatEntry)
	if !ok2 || e.chat == nil {
		return false
	}
	lock := p.tryLock(groupID, true)
	if lock == nil {
		// AI 长响应中拿不到锁：保留计数，响应结束后继续累计直到成功清理，
		// 否则长响应期间积累的 30+ 条消息永远不会触发历史清理
		return false
	}
	defer lock.release()
	return p.clearChatHistory(ctx, groupID, e.chat)
}

// applyNoMentionClear 会话创建/重建后的补清入口（@ 消息路径调用，会话锁已持有）：
// 若未@计数已达到阈值，说明上一段会话已在无@闲聊中过期，立即清空（含持久化历史，
// 重启后不再恢复），让本轮对话从新上下文开始；无论是否清空，@ 都会重置连续计数。
func (p *AIChatPlugin) applyNoMentionClear(ctx context.Context, groupID message.QID, chat *aichat.ChatBot) {
	if p.cfg.Session.NoMentionClear > 0 && chat != nil && p.noMentionValue(groupID) >= p.cfg.Session.NoMentionClear {
		p.clearChatHistory(ctx, groupID, chat)
	}
	p.resetNoMention(groupID)
}

// clearChatHistory 清空指定群会话的对话历史（内存窗口 + 持久化存储）并卸载
// 动态加载的 MCP 工具。chat 为 nil 时返回 false（调用方据此保留计数待补清）。
func (p *AIChatPlugin) clearChatHistory(ctx context.Context, groupID message.QID, chat *aichat.ChatBot) bool {
	if chat == nil {
		return false
	}
	chat.ClearHistory(ctx)
	p.Logger.Info("自动清理AI对话信息", "group", groupID, "reason", "未@消息达到阈值")
	if cleared := chat.ClearDynamicTools(); cleared > 0 {
		p.Logger.Info("清理动态加载的 MCP 工具", "count", cleared)
	}
	return true
}
