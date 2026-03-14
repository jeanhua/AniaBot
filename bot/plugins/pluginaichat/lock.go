package pluginaichat

import (
	"context"

	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/jeanhua/AniaBot/common/storage"
)

func (p *AIChatPlugin) tryLock(ctx context.Context, id message.QID) bool {
	return p.lockStorage.SetString(ctx, id.String(), "1", storage.WithCheckExist(), storage.WithTTL(LockExpTime))
}

func (p *AIChatPlugin) unLock(ctx context.Context, id message.QID) {
	p.lockStorage.Del(ctx, id.String())
}
