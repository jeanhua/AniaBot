package pluginaichat

import (
	"context"
	"time"

	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/jeanhua/AniaBot/common/storage"
)

func (p *AIChatPlugin) tryLock(id message.QID) bool {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	return p.lockStorage.SetString(ctx, id.String(), "1", storage.WithCheckExist(), storage.WithTTL(LockExpTime))
}

func (p *AIChatPlugin) unLock(id message.QID) {
	unlockCtx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	p.lockStorage.Del(unlockCtx, id.String())
}
