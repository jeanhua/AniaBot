package urlparser

import (
	"context"
	"errors"

	"github.com/jeanhua/AniaBot/common/bot"
	"github.com/jeanhua/AniaBot/common/model/command"
	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/jeanhua/AniaBot/common/plugin"
	"github.com/spf13/viper"
	"github.com/tmc/langchaingo/llms/openai"
)

type URLParserPlugin struct {
	plugin.Meta
	pendding chan work

	// 缓存时间，单位分钟
	cacheTTL int

	llm   *openai.LLM
	token string
}

func NewURLParserPlugin(maxWork int) *URLParserPlugin {
	return &URLParserPlugin{
		Meta: plugin.Meta{
			Name:      "URL解析插件",
			HelpWords: "自动解析群聊中的 URL 并提取相关信息",
			Order:     plugin.LevelNormal,
			ShowFor:   plugin.ShowForGroup,
			Author:    "jeanhua",
			Version:   "1.0.0",
		},
		pendding: make(chan work, maxWork),
	}
}

func (p *URLParserPlugin) Start(ctx context.Context, cfg *viper.Viper) error {
	p.cacheTTL = cfg.GetInt("plugin.url_parser.cache_ttl")
	if p.cacheTTL <= 0 {
		p.cacheTTL = 60
	}
	token := cfg.GetString("plugin.url_parser.token")
	if token == "" {
		return errors.New("url_parser.token is empty")
	}
	p.token = token
	baseUrl := cfg.GetString("plugin.url_parser.llm.base_url")
	if baseUrl == "" {
		return errors.New("url_parser.llm.base_url is empty")
	}
	apiKey := cfg.GetString("plugin.url_parser.llm.api_key")
	if apiKey == "" {
		return errors.New("url_parser.llm.api_key is empty")
	}
	model := cfg.GetString("plugin.url_parser.llm.model")
	if model == "" {
		return errors.New("url_parser.llm.model is empty")
	}
	llm, err := openai.New(
		openai.WithBaseURL(baseUrl),
		openai.WithToken(apiKey),
		openai.WithModel(model),
	)
	if err != nil {
		return err
	}
	p.llm = llm
	return nil
}

func (p *URLParserPlugin) Awake(ctx context.Context, bot bot.Bot) error {
	bot.Go("URLParser插件工作循环线程", func() {
		p.workLoop(bot)
	})
	return nil
}

func (p *URLParserPlugin) OnGroupMsg(ctx context.Context, bot bot.Bot, cmd command.Command, msg message.Message) (bool, error) {
	if cmd.Mention {
		return true, nil
	}
	text := getMsgText(msg)
	url := getUrl(text)
	if url != "" {
		select {
		case p.pendding <- work{
			GroupID: msg.GroupId,
			URL:     url,
			MsgID:   msg.MessageId,
		}:
		default:
		}
	}
	return true, nil
}
