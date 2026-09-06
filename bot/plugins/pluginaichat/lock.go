package pluginaichat

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strconv"
	"sync"
	"time"

	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/jeanhua/AniaBot/common/storage"
)

// lockRenewInterval 会话锁续期心跳间隔：取 TTL 的 1/3，一次续期失败仍有
// 两次机会在过期前补救。
const lockRenewInterval = LockExpTime / 3

// sessionLock 一次成功获取的会话锁句柄：
//   - 后台心跳周期性把 TTL 续回 LockExpTime。AI 响应耗时可配置到远超
//     LockExpTime（工具审批、子代理、多轮工具调用），若锁中途过期，后续
//     消息会拿到锁并与当前响应并发驱动同一个 ChatBot（messageWindow 等状态
//     非并发安全，会产生数据竞争与历史错乱）；
//   - 锁值携带随机令牌，release 时仅在令牌仍匹配时删除——即使锁意外过期被
//     他人接管，先到响应的 release 也不会误删新持有者的锁。
type sessionLock struct {
	p           *AIChatPlugin
	key         string
	token       string
	stopCh      chan struct{}
	stopped     chan struct{}
	releaseOnce sync.Once
}

// tryLock 尝试获取会话锁：成功返回锁句柄（后台自动续期），失败返回 nil。
// 同时占用一个并发槽位（rateCh），与并发限制共用语义。
func (p *AIChatPlugin) tryLock(id message.QID, isGroup bool) *sessionLock {
	select {
	case p.rateCh <- struct{}{}:
	default:
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	token := newLockToken()
	locked := p.lockStorage.SetString(ctx, sessionKey(id, isGroup), token, storage.WithCheckExist(), storage.WithTTL(LockExpTime))
	if !locked {
		select {
		case <-p.rateCh:
		default:
		}
		return nil
	}
	l := &sessionLock{
		p:       p,
		key:     sessionKey(id, isGroup),
		token:   token,
		stopCh:  make(chan struct{}),
		stopped: make(chan struct{}),
	}
	go l.renewLoop()
	return l
}

// release 释放锁（幂等，可安全 defer）：先停心跳并等其退出（否则删除后
// 心跳可能把 TTL 续回去，锁「复活」阻塞后续消息），再在令牌仍匹配时删除，
// 最后归还并发槽位。GetString 与 Del 之间存在微小检查窗口（存储接口未暴露
// 原子 compare-and-delete），但相比无条件删除已消除「误删他人锁」的实际风险。
func (l *sessionLock) release() {
	l.releaseOnce.Do(func() {
		close(l.stopCh)
		<-l.stopped
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		defer cancel()
		select {
		case <-l.p.rateCh:
		default:
		}
		if cur, ok := l.p.lockStorage.GetString(ctx, l.key); ok && cur == l.token {
			l.p.lockStorage.Del(ctx, l.key)
		}
	})
}

// renewLoop 心跳续期：锁持有期间周期性把 TTL 重置回 LockExpTime。
func (l *sessionLock) renewLoop() {
	defer close(l.stopped)
	ticker := time.NewTicker(lockRenewInterval)
	defer ticker.Stop()
	for {
		select {
		case <-l.stopCh:
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
			l.p.lockStorage.Expire(ctx, l.key, LockExpTime)
			cancel()
		}
	}
}

// newLockToken 生成锁持有者随机令牌
func newLockToken() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand 失败极罕见，退化用纳秒时间戳（配合 CheckExist 仍可工作）
		return strconv.FormatInt(time.Now().UnixNano(), 16)
	}
	return hex.EncodeToString(b)
}
