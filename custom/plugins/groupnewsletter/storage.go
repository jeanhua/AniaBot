package groupnewsletter

import (
	"context"
	"log"
	"strconv"
	"strings"
	"time"
)

const storageKeyPrefix = "group_"

func storageKey(groupId uint) string {
	return storageKeyPrefix + strconv.FormatUint(uint64(groupId), 10)
}

// saveLoop 消费 saveChan，批量去重后异步持久化，不阻塞消息收集
func (p *GroupNewsletter) saveLoop() {
	// 用 ticker 做批量合并：每 2 秒最多触发一次同一群的写入
	pending := make(map[uint]struct{})
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	flush := func() {
		for groupId := range pending {
			p.saveGroupToStorage(groupId)
		}
		pending = make(map[uint]struct{})
	}

	for {
		select {
		case <-p.pluginCtx.Done():
			// 退出前把剩余的全部落盘
		drain:
			for {
				select {
				case groupId := <-p.saveChan:
					pending[groupId] = struct{}{}
				default:
					break drain
				}
			}
			flush()
			log.Println("[群刊] saveLoop 退出")
			return

		case groupId := <-p.saveChan:
			pending[groupId] = struct{}{}

		case <-ticker.C:
			if len(pending) > 0 {
				flush()
			}
		}
	}
}

func (p *GroupNewsletter) saveGroupToStorage(groupId uint) {
	p.msgsMu.RLock()
	buffer, ok := p.groupMsgs[groupId]
	p.msgsMu.RUnlock()

	if !ok {
		return
	}

	buffer.mu.RLock()
	msgs := make([]collectedMessage, len(buffer.messages))
	copy(msgs, buffer.messages)
	buffer.mu.RUnlock()

	if ok := p.Storage.Set(context.Background(), storageKey(groupId), msgs); !ok {
		log.Printf("[群刊] 持久化群 %d 消息失败", groupId)
	}
}

func (p *GroupNewsletter) loadFromStorage() {
	keys, err := p.Storage.ScanKeys(context.Background(), storageKeyPrefix+"*", 100)
	if err != nil {
		log.Printf("[群刊] 加载存储消息失败: %v", err)
		return
	}

	for _, key := range keys {
		var msgs []collectedMessage
		if !p.Storage.Get(context.Background(), key, &msgs) || len(msgs) == 0 {
			continue
		}

		idStr := strings.TrimPrefix(key, storageKeyPrefix)
		groupId64, err := strconv.ParseUint(idStr, 10, 64)
		if err != nil {
			log.Printf("[群刊] 解析群 ID 失败，key=%s: %v", key, err)
			continue
		}

		groupId := uint(groupId64)
		p.groupMsgs[groupId] = &groupMessageBuffer{messages: msgs}
		log.Printf("[群刊] 从存储恢复群 %d 的 %d 条消息", groupId, len(msgs))
	}
}
