package urlparser

import (
	"context"
	"time"

	"github.com/jeanhua/AniaBot/common/bot"
	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/jeanhua/AniaBot/common/msgchain"
	"github.com/jeanhua/AniaBot/common/storage"
	"github.com/tmc/langchaingo/llms"
)

type work struct {
	GroupID message.QID
	URL     string
	MsgID   message.QID
}

const (
	ctxTimeout = time.Minute * 5
)

func (p *URLParserPlugin) workLoop(bot bot.Bot) {
	for w := range p.pendding {
		ctx, cancel := context.WithTimeout(context.Background(), ctxTimeout)
		defer cancel()

		cache, ok := p.Storage.GetString(ctx, "cache:"+w.URL)
		if ok {
			p.Logger.Info("发送缓存内容", "groupId", w.GroupID, "content", cache)
			builder := msgchain.Builder().Group()
			builder.Reply(w.MsgID)
			builder.Text(cache)
			bot.SendGroupMsg(w.GroupID, builder.Build())
			continue
		}

		content, err := fetchPageContent(ctx, p.token, w.URL)
		if err != nil {
			p.Logger.Info("fetchPageContent failed", "url", w.URL, "err", err)
			continue
		}

		resp, err := p.llm.GenerateContent(ctx, []llms.MessageContent{
			llms.TextParts(llms.ChatMessageTypeSystem, "你是一个网页内容提炼员，你的任务是从网页内容中提取相关信息，100字左右。"),
			llms.TextParts(llms.ChatMessageTypeHuman, content),
		}, llms.WithMaxTokens(300))
		if err != nil {
			p.Logger.Info("GenerateContent failed", "url", w.URL, "err", err)
			continue
		}

		result := resp.Choices[0].Content
		p.Storage.SetString(ctx, "cache:"+w.URL, result, storage.WithTTL(time.Minute*time.Duration(p.cacheTTL)))

		p.Logger.Info("发送提炼内容", "groupId", w.GroupID, "content", result)
		builder := msgchain.Builder().Group()
		builder.Reply(w.MsgID)
		builder.Text(result)
		bot.SendGroupMsg(w.GroupID, builder.Build())
	}
}
