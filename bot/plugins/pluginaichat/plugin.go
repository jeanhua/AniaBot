package pluginaichat

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/jeanhua/AniaBot/bot/component"
	"github.com/jeanhua/AniaBot/bot/component/functool"
	"github.com/jeanhua/AniaBot/common/aniaerror"
	"github.com/jeanhua/AniaBot/common/bot"
	"github.com/jeanhua/AniaBot/common/model/command"
	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/jeanhua/AniaBot/common/msgchain"
	"github.com/jeanhua/AniaBot/common/plugin"
	"github.com/jeanhua/AniaBot/common/storage"
	"github.com/spf13/viper"
	"github.com/tmc/langchaingo/llms"
)

type AIChatPlugin struct {
	plugin.Meta
	chats sync.Map

	lockStorage storage.Storage

	botConfig struct {
		baseURL string
		apiKey  string
		model   string
	}

	llmParameter struct {
		maxToken    int
		temperature float64
		top_p       float64
		top_k       int
		prompt      string
		searchToken string
	}

	ocrEnable    bool
	ocrModel     *component.ChatBot
	ocrParameter struct {
		maxToken    int
		temperature float64
		top_p       float64
		top_k       int
		prompt      string
	}
}

const (
	LockExpTime = time.Minute * 10
)

func NewAIChatPlugin() *AIChatPlugin {
	return &AIChatPlugin{
		Meta: plugin.Meta{
			Name:      "AI对话插件",
			HelpWords: "@我聊天哦，带上 #新对话 标签可以创建新对话",
			Order:     plugin.LevelPostHandle,
		},
	}
}

func (p *AIChatPlugin) tryLock(ctx context.Context, id uint) bool {
	return p.lockStorage.SetString(ctx, fmt.Sprintf("%d", id), "1", storage.WithCheckExist(), storage.WithTTL(LockExpTime))
}

func (p *AIChatPlugin) unLock(ctx context.Context, id uint) {
	p.lockStorage.Del(ctx, fmt.Sprintf("%d", id))
}

func (p *AIChatPlugin) getChat(id uint) *component.ChatBot {
	chat, ok := p.chats.Load(id)
	if !ok {
		c, err := component.NewChatBot(
			p.botConfig.baseURL,
			p.botConfig.apiKey,
			p.botConfig.model,
			p.llmParameter.prompt,
			30,
			p.llmParameter.searchToken,
		)
		if err != nil {
			return nil
		}
		p.chats.Store(id, c)
		return c
	}
	return chat.(*component.ChatBot)
}

func (p *AIChatPlugin) OnGroupMsg(ctx context.Context, bot bot.Bot, cmd command.Command, msg message.Message) (bool, error) {
	if !cmd.Mention {
		return true, nil
	}
	if !p.tryLock(ctx, msg.GroupId) {
		builder := msgchain.Builder().Group()
		builder.Text("正在等待响应中，不要着急哦~")
		bot.SendGroupMsg(msg.GroupId, builder.Build())
		return true, nil
	}
	defer p.unLock(ctx, msg.GroupId)
	chat := p.getChat(msg.GroupId)
	if chat == nil {
		builder := msgchain.Builder().Group()
		builder.Text("无法创建对话，请检查日志信息哦")
		bot.SendGroupMsg(msg.GroupId, builder.Build())
		return true, nil
	}

	builder := msgchain.Builder().Group()
	extraText := extraMsg(ctx, bot, msg, p.ocrModel)
	if strings.Contains(extraText, "#新对话") {
		err := chat.ClearHistory(ctx)
		if err != nil {
			log.Println("无法清理AI聊天信息", err.Error())
			return false, nil
		} else {
			log.Println("清理AI对话信息成功")
		}
	}

	msgFuncs := functool.OptionFuncs{
		SendText: func(s string) bool {
			builder := msgchain.Builder().Group()
			builder.Mention(msg.Sender.UserId)
			builder.Text(" " + s)
			_, success := bot.SendGroupMsg(msg.GroupId, builder.Build())
			if success {
				log.Printf("[发->群:%d]: %s", msg.GroupId, s)
			}
			return success
		},
		SendImage: func(url string) bool {
			builder := msgchain.Builder().Group()
			builder.ImageUrl(url)
			_, success := bot.SendGroupMsg(msg.GroupId, builder.Build())
			if success {
				log.Printf("[发->群:%d]: [图片:%s]", msg.GroupId, url)
			}
			return success
		},
	}

	resp, err := chat.Chat(ctx, extraText, msgFuncs,
		llms.WithMaxTokens(p.llmParameter.maxToken),
		llms.WithTemperature(p.llmParameter.temperature),
		llms.WithTopP(p.llmParameter.top_p),
		llms.WithTopK(p.llmParameter.top_k),
	)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			builder.Text("请求超时")
			bot.SendGroupMsg(msg.GroupId, builder.Build())
			return true, nil
		}
		builder.Text("无法解析的错误信息，请查看日志")
		log.Println(err.Error())
		bot.SendGroupMsg(msg.GroupId, builder.Build())
		return true, nil
	}

	if resp == "" {
		log.Println("AI请求没有返回什么东西")
		return true, nil
	}

	builder.Mention(msg.Sender.UserId)
	builder.Text(" " + resp)
	if _, success := bot.SendGroupMsg(msg.GroupId, builder.Build()); success {
		log.Printf("[发->群:%d]: %s", msg.GroupId, resp)
	}
	return true, nil
}

func (p *AIChatPlugin) OnFriendMsg(ctx context.Context, bot bot.Bot, cmd command.Command, msg message.Message) (bool, error) {
	if !p.tryLock(ctx, msg.Sender.UserId) {
		builder := msgchain.Builder().Friend()
		builder.Text("正在等待响应中，不要着急哦~")
		bot.SendFriendMsg(msg.Sender.UserId, builder.Build())
		return true, nil
	}
	defer p.unLock(ctx, msg.Sender.UserId)

	chat := p.getChat(msg.Sender.UserId)
	if chat == nil {
		builder := msgchain.Builder().Friend()
		builder.Text("无法创建对话，请检查日志信息哦")
		bot.SendFriendMsg(msg.Sender.UserId, builder.Build())
		return true, nil
	}

	builder := msgchain.Builder().Friend()
	extraText := extraMsg(ctx, bot, msg, p.ocrModel)
	if strings.Contains(extraText, "#新对话") {
		err := chat.ClearHistory(ctx)
		if err != nil {
			log.Println("无法清理AI聊天信息", err.Error())
			return false, nil
		} else {
			log.Println("清理AI对话信息成功")
		}
	}

	msgFuncs := functool.OptionFuncs{
		SendText: func(s string) bool {
			builder := msgchain.Builder().Friend()
			builder.Text(s)
			_, success := bot.SendFriendMsg(msg.Sender.UserId, builder.Build())
			if success {
				log.Printf("[发->好友:%d]: %s", msg.Sender.UserId, s)
			}
			return success
		},
		SendImage: func(url string) bool {
			builder := msgchain.Builder().Friend()
			builder.ImageUrl(url)
			_, success := bot.SendFriendMsg(msg.Sender.UserId, builder.Build())
			if success {
				log.Printf("[发->好友:%d]: [图片:%s]", msg.Sender.UserId, url)
			}
			return success
		},
	}

	resp, err := chat.Chat(ctx, extraText,
		msgFuncs,
		llms.WithMaxTokens(p.llmParameter.maxToken),
		llms.WithTemperature(p.llmParameter.temperature),
		llms.WithTopP(p.llmParameter.top_p),
		llms.WithTopK(p.llmParameter.top_k),
	)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			builder.Text("请求超时")
			bot.SendFriendMsg(msg.Sender.UserId, builder.Build())
			return true, nil
		}
		builder.Text("无法解析的错误信息，请查看日志")
		log.Println(err.Error())
		bot.SendFriendMsg(msg.Sender.UserId, builder.Build())
		return true, nil
	}

	if resp == "" {
		log.Println("AI请求没有返回什么东西")
		return true, nil
	}

	builder.Text(resp)
	if _, success := bot.SendFriendMsg(msg.Sender.UserId, builder.Build()); success {
		log.Printf("[发->好友:%d]: %s", msg.Sender.UserId, resp)
	}
	return true, nil
}

func (p *AIChatPlugin) Start(ctx context.Context, cfg *viper.Viper) error {
	p.lockStorage = p.Storage.Clone("lock")
	p.lockStorage.Clear(ctx)

	p.botConfig.baseURL = cfg.GetString("plugin.ai_chat_bot.base_url")
	p.botConfig.model = cfg.GetString("plugin.ai_chat_bot.model")
	p.botConfig.apiKey = cfg.GetString("plugin.ai_chat_bot.api_key")
	p.llmParameter.prompt = cfg.GetString("plugin.ai_chat_bot.prompt")

	if p.botConfig.baseURL == "" {
		log.Println("初始化失败：未配置 Base Url")
		return aniaerror.ParameterInitializeError
	}
	if p.botConfig.model == "" {
		log.Println("初始化失败：未配置 Model")
		return aniaerror.ParameterInitializeError
	}
	if p.botConfig.apiKey == "" {
		log.Println("初始化失败：未配置 API KEY")
		return aniaerror.ParameterInitializeError
	}
	if p.llmParameter.prompt == "" {
		log.Println("未配置 Prompt，将使用预设的默认提示词")
		p.llmParameter.prompt = "你是一个ai对话机器人，在QQ上和别人聊天，说话不要长篇大论"
	}

	p.llmParameter.maxToken = cfg.GetInt("plugin.ai_chat_bot.max_token")
	p.llmParameter.temperature = cfg.GetFloat64("plugin.ai_chat_bot.temperature")
	p.llmParameter.top_p = cfg.GetFloat64("plugin.ai_chat_bot.top_p")
	p.llmParameter.top_k = cfg.GetInt("plugin.ai_chat_bot.top_k")

	p.ocrEnable = cfg.GetBool("plugin.ai_chat_bot.ocr.enable")
	if p.ocrEnable {
		log.Println("已启用OCR LLM")
		ocrBaseUrl := cfg.GetString("plugin.ai_chat_bot.ocr.base_url")
		ocrAPIKey := cfg.GetString("plugin.ai_chat_bot.ocr.api_key")
		ocrModel := cfg.GetString("plugin.ai_chat_bot.ocr.model")
		ocrPrompt := cfg.GetString("plugin.ai_chat_bot.ocr.prompt")
		searchToken := cfg.GetString("plugin.ai_chat_bot.search.token")

		p.ocrParameter.maxToken = cfg.GetInt("plugin.ai_chat_bot.ocr.max_token")
		p.ocrParameter.temperature = cfg.GetFloat64("plugin.ai_chat_bot.ocr.temperature")
		p.ocrParameter.top_p = cfg.GetFloat64("plugin.ai_chat_bot.ocr.top_p")
		p.ocrParameter.top_k = cfg.GetInt("plugin.ai_chat_bot.ocr.top_k")
		p.llmParameter.searchToken = searchToken

		ocrllm, err := component.NewChatBot(ocrBaseUrl, ocrAPIKey, ocrModel, ocrPrompt, 10, searchToken)
		if err != nil {
			log.Println("无法初始化OCR LLM", err.Error())
			p.ocrEnable = false
		} else {
			p.ocrModel = ocrllm
		}
	}
	return nil
}

func extraMsg(ctx context.Context, bot bot.Bot, msg message.Message, ocrLLM *component.ChatBot, opt ...llms.CallOption) string {
	var str strings.Builder
	nickname := msg.Sender.Card
	if nickname == "" {
		nickname = msg.Sender.Nickname
	}
	str.WriteString(fmt.Sprintf("[nickname:%s id:%d]:", nickname, msg.Sender.UserId))
	for _, m := range msg.Message {
		str.WriteString(
			m.FriendlyText(
				message.WithIgnoreMentionId(msg.SelfId),
				message.WithGetMsgFunc(bot.GetMsgDetail),
				message.WithGetGroupUserInfo(msg.GroupId, bot.GetGroupUserInfo),
				message.WithGetForwardMsgFunc(bot.GetForwardMsg),
				message.WithGetImageOCRFunc(func(url string) string {
					if ocrLLM == nil {
						return "OCR服务未开启，无法解析图片"
					}
					resp, err := ocrLLM.ChatWithImage(ctx, "描述图片内容", url, opt...)
					if err != nil {
						log.Println("OCR请求失败:", err.Error())
						return "OCR请求失败，无法解析的图片内容"
					} else {
						return resp
					}
				}),
			),
		)
	}
	return str.String()
}
