package urlparser

import (
	"context"

	"github.com/jeanhua/AniaBot/common/bot"
	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/jeanhua/AniaBot/common/msgchain"
	"github.com/tmc/langchaingo/llms"
)

type work struct {
	GroupID message.QID
	URL     string
	MsgID   message.QID
}

func (p *URLParserPlugin) workLoop(bot bot.Bot) {
	for w := range p.pendding {
		content, err := fetchPageContent(context.Background(), p.token, w.URL)
		if err != nil {
			p.Logger.Println("fetchPageContent failed", err)
			continue
		}

		resp, err := p.llm.GenerateContent(context.Background(), []llms.MessageContent{
			llms.TextParts(llms.ChatMessageTypeSystem, "你是一个网页内容提炼员，你的任务是从网页内容中提取相关信息，100字左右。"),
			llms.TextParts(llms.ChatMessageTypeHuman, content),
		}, llms.WithMaxTokens(300))
		if err != nil {
			p.Logger.Println("GenerateContent failed", err)
			continue
		}
		p.Logger.Printf("[发->群 %d] %s", w.GroupID, resp.Choices[0].Content)
		builder := msgchain.Builder().Group()
		builder.Reply(w.MsgID)
		builder.Text(resp.Choices[0].Content)
		bot.SendGroupMsg(w.GroupID, builder.Build())
	}
}
