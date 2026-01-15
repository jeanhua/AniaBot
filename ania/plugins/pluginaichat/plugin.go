package pluginaichat

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/jeanhua/AniaBot/ania/component"
	"github.com/jeanhua/AniaBot/ania/utils"
	"github.com/jeanhua/AniaBot/common/bot"
	"github.com/jeanhua/AniaBot/common/model/command"
	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/jeanhua/AniaBot/common/msgchain"
	"github.com/jeanhua/AniaBot/common/plugin"
	"github.com/spf13/viper"
)

type AIChatPlugin struct {
	plugin.Meta
	chats     sync.Map
	chatsLock sync.Map

	botConfig struct {
		baseURL string
		apiKey  string
		model   string
		prompt  string
	}
}

func NewAIChatPlugin() *AIChatPlugin {
	return &AIChatPlugin{
		Meta: plugin.Meta{
			Name:      "AI对话插件",
			HelpWords: "@我聊天哦",
			Order:     1000,
		},
	}
}

func (p *AIChatPlugin) lock(id uint) bool {
	_, loaded := p.chatsLock.LoadOrStore(id, 1)
	return !loaded
}

func (p *AIChatPlugin) unLock(id uint) {
	p.chatsLock.Delete(id)
}

func (p *AIChatPlugin) getChat(id uint) *component.ChatBot {
	chat, ok := p.chats.Load(id)
	if !ok {
		c, err := component.NewChatBot(
			p.botConfig.baseURL,
			p.botConfig.apiKey,
			p.botConfig.model,
			p.botConfig.prompt,
		)
		if err != nil {
			return nil
		}
		p.chats.Store(id, c)
		return c
	}
	return chat.(*component.ChatBot)
}

func (p *AIChatPlugin) OnGroupMsg(bot bot.Bot, cmd *command.Command, msg message.Message) bool {
	if !utils.HasMention(msg) {
		return true
	}
	if !p.lock(msg.GroupId) {
		builder := msgchain.Builder.Group()
		builder.Text("正在等待响应中，不要着急哦~")
		bot.SendGroupMsg(msg.GroupId, builder.Build())
		return true
	}
	defer p.unLock(msg.GroupId)
	chat := p.getChat(msg.GroupId)
	if chat == nil {
		builder := msgchain.Builder.Group()
		builder.Text("无法创建对话，请检查日志信息哦")
		bot.SendGroupMsg(msg.GroupId, builder.Build())
		return true
	}

	builder := msgchain.Builder.Group()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*3)
	defer cancel()
	resp, err := chat.Chat(ctx, extraMsg(bot, msg))
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			builder.Text("请求超时")
			bot.SendGroupMsg(msg.GroupId, builder.Build())
			return true
		}
		builder.Text("无法解析的错误信息，请查看日志")
		log.Println(err.Error())
		bot.SendGroupMsg(msg.GroupId, builder.Build())
		return true
	}

	if resp == "" {
		log.Println("AI请求没有返回什么东西")
		return true
	}

	builder.Mention(msg.Sender.UserId)
	builder.Text(" " + resp)
	bot.SendGroupMsg(msg.GroupId, builder.Build())
	return true
}

func (p *AIChatPlugin) OnFriendMsg(bot bot.Bot, cmd *command.Command, msg message.Message) bool {
	if !p.lock(msg.Sender.UserId) {
		builder := msgchain.Builder.Friend()
		builder.Text("正在等待响应中，不要着急哦~")
		bot.SendFriendMsg(msg.Sender.UserId, builder.Build())
		return true
	}
	defer p.unLock(msg.Sender.UserId)

	chat := p.getChat(msg.Sender.UserId)
	if chat == nil {
		builder := msgchain.Builder.Friend()
		builder.Text("无法创建对话，请检查日志信息哦")
		bot.SendFriendMsg(msg.Sender.UserId, builder.Build())
		return true
	}

	builder := msgchain.Builder.Friend()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*3)
	defer cancel()
	resp, err := chat.Chat(ctx, extraMsg(bot, msg))
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			builder.Text("请求超时")
			bot.SendFriendMsg(msg.Sender.UserId, builder.Build())
			return true
		}
		builder.Text("无法解析的错误信息，请查看日志")
		log.Println(err.Error())
		bot.SendFriendMsg(msg.Sender.UserId, builder.Build())
		return true
	}

	if resp == "" {
		log.Println("AI请求没有返回什么东西")
		return true
	}

	builder.Text(resp)
	bot.SendFriendMsg(msg.Sender.UserId, builder.Build())
	return true
}

func (p *AIChatPlugin) Start(cfg *viper.Viper) {
	p.botConfig.baseURL = cfg.GetString("plugin.ai_chat_bot.base_url")
	p.botConfig.model = cfg.GetString("plugin.ai_chat_bot.model")
	p.botConfig.apiKey = cfg.GetString("plugin.ai_chat_bot.api_key")
	p.botConfig.prompt = cfg.GetString("plugin.ai_chat_bot.prompt")

	if p.botConfig.baseURL == "" {
		log.Println("初始化失败：未配置 Base Url")
	}
	if p.botConfig.model == "" {
		log.Println("初始化失败：未配置 Model")
	}
	if p.botConfig.apiKey == "" {
		log.Println("初始化失败：未配置 API KEY")
	}
	if p.botConfig.prompt == "" {
		log.Println("未配置 Prompt，将使用预设的默认提示词")
		p.botConfig.prompt = "你是一个ai对话机器人，在QQ上和别人聊天，说话不要长篇大论"
	}
}

func extraMsg(bot bot.Bot, msg message.Message) string {
	var str strings.Builder
	nickname := msg.Sender.Card
	if nickname == "" {
		nickname = msg.Sender.Nickname
	}
	str.WriteString(fmt.Sprintf("[%s %d]:", nickname, msg.Sender.UserId))
	for _, m := range msg.Message {
		str.WriteString(m.FriendlyText(bot.GetMsgDetail, true))
	}
	return str.String()
}
