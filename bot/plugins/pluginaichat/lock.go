package pluginaichat

import (
	"context"
	"time"

	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/jeanhua/AniaBot/common/storage"
)

func (p *AIChatPlugin) tryLock(id message.QID, isGroup bool) bool {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	select {
	case p.rateCh <- struct{}{}:
	default:
		return false
	}
	locked := p.lockStorage.SetString(ctx, sessionKey(id, isGroup), "1", storage.WithCheckExist(), storage.WithTTL(LockExpTime))
	if !locked {
		select {
		case <-p.rateCh:
		default:
		}
	}
	return locked
}

func (p *AIChatPlugin) unLock(id message.QID, isGroup bool) {
	unlockCtx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	select {
	case <-p.rateCh:
	default:
	}
	p.lockStorage.Del(unlockCtx, sessionKey(id, isGroup))
}
