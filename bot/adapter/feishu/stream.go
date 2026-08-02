package feishu

import (
	"context"
	"sync"
	"time"

	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

// streamPatchInterval 流式更新节流间隔：飞书消息编辑接口有频率限制，
// 过快的增量合并到最近一次 Patch，End 时强制发送最终内容。
const streamPatchInterval = 600 * time.Millisecond

// feishuStreamHandle 飞书流式消息句柄：以 interactive 卡片创建消息后，
// Patch 通过 im.message.patch 更新卡片内容（节流）；End 强制最终内容（幂等）。
// prefix 为初始消息中不可丢弃的提及（@）markdown：Patch 替换整个卡片内容，
// 每次更新都需重新带上，否则 @ 高亮会从卡片上消失。
type feishuStreamHandle struct {
	a      *feishuAdapter
	msgID  string
	prefix string

	mu        sync.Mutex
	content   string
	lastPatch time.Time
	closed    bool
}

// Patch 更新消息内容：距上次成功 Patch 超过节流间隔时立即发送，否则仅记录
// 最新内容（后续 Patch 或 End 时一并发送）。
func (h *feishuStreamHandle) Patch(text string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return nil
	}
	h.content = text
	if time.Since(h.lastPatch) >= streamPatchInterval {
		return h.patchLocked()
	}
	return nil
}

// End 强制发送最终内容（幂等，结束后不可再 Patch）。
func (h *feishuStreamHandle) End() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return
	}
	_ = h.patchLocked()
	h.closed = true
}

// patchLocked 以当前内容更新卡片；调用方需持有 h.mu。
func (h *feishuStreamHandle) patchLocked() error {
	if h.a == nil || h.a.client == nil || h.msgID == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := h.a.client.Im.V1.Message.Patch(ctx, larkim.NewPatchMessageReqBuilder().
		MessageId(h.msgID).
		Body(larkim.NewPatchMessageReqBodyBuilder().
			Content(buildCardJSON(h.prefix+h.content)).
			Build()).
		Build())
	if err != nil {
		h.a.logger.Warn("飞书流式回复更新失败", "messageId", h.msgID, "error", err)
		return err
	}
	h.lastPatch = time.Now()
	return nil
}
