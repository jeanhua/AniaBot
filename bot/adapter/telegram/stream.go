package telegram

import (
	"context"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/jeanhua/AniaBot/common/bot"
	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/jeanhua/AniaBot/common/msgchain"
)

const (
	// streamPatchInterval 流式更新节流间隔（同飞书）：Telegram 消息编辑也有频率限制，
	// 过快的增量合并到最近一次编辑，End 时强制发送最终内容。
	streamPatchInterval = 600 * time.Millisecond
	// maxEditTextLen Telegram 单条消息内容上限（编辑与发送相同）。
	maxEditTextLen = 4096
)

// SendGroupStream 实现 adapter.StreamSenderExt：以消息创建流式群聊回复，Patch 经
// editMessageText 更新内容。
func (a *telegramAdapter) SendGroupStream(groupId message.QID, chain msgchain.GroupChain) (bot.StreamHandle, bool) {
	return a.sendStream(groupId, chain.GetGroupMsg())
}

// SendFriendStream 实现 adapter.StreamSenderExt：流式私聊回复。
func (a *telegramAdapter) SendFriendStream(userId message.QID, chain msgchain.FriendChain) (bot.StreamHandle, bool) {
	return a.sendStream(userId, chain.GetFriendMsg())
}

// sendStream 创建流式消息：文本段拼接为初始内容，@ 段经 resolveMention 展开为
// prefix（"@username " 文本）；后续 Patch/End 时 prefix 始终重新带上（aichat 的
// Patch 只传 AI 增量文本，否则首条消息里的 @username 会在第一次编辑时消失）。
// reply 段作为回复目标（reply_parameters）。
func (a *telegramAdapter) sendStream(target message.QID, segs []message.OB11Segment) (bot.StreamHandle, bool) {
	chatID, ok := parseChatID(target)
	if !ok || a.client == nil {
		return nil, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var textSb, prefixSb strings.Builder
	var replyTo *int
	for _, s := range segs {
		switch s.Type {
		case message.SegmentText:
			if t, ok := s.Data["text"].(string); ok {
				textSb.WriteString(t)
			}
		case message.SegmentMention:
			prefixSb.WriteString(a.resolveMention(ctx, chatID, s))
		case message.SegmentReply:
			if replyTo == nil {
				if id, ok := s.Data["id"].(string); ok {
					if _, mid, ok2 := parseMsgID(id); ok2 {
						replyTo = &mid
					}
				}
			}
		}
	}
	prefix := prefixSb.String()
	text := prefix + textSb.String()
	if text == "" {
		return nil, false
	}
	text = truncateRunes(text, maxEditTextLen)
	id, ok := a.sendText(ctx, chatID, text, replyTo)
	if !ok {
		return nil, false
	}
	return &telegramStreamHandle{a: a, chatID: chatID, msgID: id, prefix: prefix}, true
}

// telegramStreamHandle Telegram 流式消息句柄：Patch 经 editMessageText 更新内容
// （节流）；End 强制最终内容（幂等），配置 markdownv2 时最终编辑尝试按
// MarkdownV2 渲染，失败（未转义特殊字符等 400）降级纯文本重发保证最终内容落地。
type telegramStreamHandle struct {
	a      *telegramAdapter
	chatID int64
	msgID  int
	// prefix 初始消息中不可丢弃的 @username 文本：编辑替换整个消息内容，需重新带上
	prefix string

	mu        sync.Mutex
	content   string
	lastPatch time.Time
	closed    bool
}

// Patch 更新消息内容：距上次成功编辑超过节流间隔时立即发送，否则仅记录最新内容
// （后续 Patch 或 End 时一并发送）。流式中间编辑始终纯文本（流式过程标记不完整，
// 带 parse_mode 会因未闭合标记被拒）。
func (h *telegramStreamHandle) Patch(text string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return nil
	}
	h.content = text
	if time.Since(h.lastPatch) >= streamPatchInterval {
		return h.patchLocked(false)
	}
	return nil
}

// End 强制发送最终内容（幂等，结束后不可再 Patch）。
func (h *telegramStreamHandle) End() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return
	}
	_ = h.patchLocked(true)
	h.closed = true
}

// patchLocked 以当前内容编辑消息；final 为 true 时尝试 MarkdownV2 渲染
// （仅最终内容完整时才可能渲染成功），400 解析失败降级纯文本重发。
// 调用方需持有 h.mu。
func (h *telegramStreamHandle) patchLocked(final bool) error {
	if h.a == nil || h.a.client == nil || h.msgID == 0 {
		return nil
	}
	content := truncateRunes(h.prefix+h.content, maxEditTextLen)
	params := map[string]any{
		"chat_id":    h.chatID,
		"message_id": h.msgID,
		"text":       content,
	}
	if final && h.a.mdEnabled() {
		params["parse_mode"] = "MarkdownV2"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err := h.a.client.call(ctx, "editMessageText", params, nil)
	// 最终 MarkdownV2 编辑失败（未转义字符/截断切断闭合标记等 400）→ 纯文本重发，
	// 避免消息停留在节流窗口内未发出的旧内容
	if err != nil && isBadRequest(err) {
		delete(params, "parse_mode")
		err = h.a.client.call(ctx, "editMessageText", params, nil)
	}
	if err != nil {
		h.a.logger.Warn("Telegram 流式回复更新失败", "messageId", h.msgID, "error", err)
		return err
	}
	h.lastPatch = time.Now()
	return nil
}

// truncateRunes 按 rune 截断到最大字节数（不切断多字节字符）。
func truncateRunes(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	var sb strings.Builder
	for _, r := range s {
		if sb.Len()+utf8.RuneLen(r) > maxBytes {
			break
		}
		sb.WriteRune(r)
	}
	return sb.String()
}
