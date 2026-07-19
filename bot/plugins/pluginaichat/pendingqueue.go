package pluginaichat

import (
	"sync"

	"github.com/jeanhua/AniaBot/common/model/message"
)

// maxPendingMessages 每个会话的排队消息上限，防止 AI 响应期间消息无限堆积
const maxPendingMessages = 20

// pendingQueue 单会话的排队消息队列，存放 AI 响应期间到达的消息
type pendingQueue struct {
	mu    sync.Mutex
	items []message.Message
}

// pendingKey 群聊与好友的 QID 数值空间可能重叠，加前缀区分队列
func pendingKey(id message.QID, isGroup bool) string {
	if isGroup {
		return "g:" + id.String()
	}
	return "f:" + id.String()
}

func (p *AIChatPlugin) getPendingQueue(id message.QID, isGroup bool) *pendingQueue {
	q, _ := p.pendingMsgs.LoadOrStore(pendingKey(id, isGroup), &pendingQueue{})
	return q.(*pendingQueue)
}

// enqueuePending 将消息加入排队队列。
// first 表示该消息是队列中的第一条（调用方可据此给用户一次性提示），
// ok 为 false 表示队列已满，消息被丢弃。
func (p *AIChatPlugin) enqueuePending(id message.QID, isGroup bool, msg message.Message) (first, ok bool) {
	q := p.getPendingQueue(id, isGroup)
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.items) >= maxPendingMessages {
		return false, false
	}
	first = len(q.items) == 0
	q.items = append(q.items, msg)
	return first, true
}

// drainPending 取出并清空当前所有排队消息
func (p *AIChatPlugin) drainPending(id message.QID, isGroup bool) []message.Message {
	q := p.getPendingQueue(id, isGroup)
	q.mu.Lock()
	defer q.mu.Unlock()
	items := q.items
	q.items = nil
	return items
}
