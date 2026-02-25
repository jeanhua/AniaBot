package groupnewsletter

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jeanhua/AniaBot/common/bot"
	"github.com/jeanhua/AniaBot/common/model/command"
	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/jeanhua/AniaBot/common/msgchain"
	"github.com/jeanhua/AniaBot/common/plugin"
	"github.com/spf13/viper"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
)

type GroupNewsletter struct {
	plugin.Meta
	llm        *openai.LLM
	config     newsletterConfig
	groupMsgs  map[uint]*groupMessageBuffer
	msgsMutex  sync.RWMutex
	notifyChan chan uint
}

type newsletterConfig struct {
	baseUrl       string
	apiKey        string
	model         string
	prompt        string
	maxToken      int
	msgThreshold  int
	maxMessages   int
	enabledGroups []uint
}

type groupMessageBuffer struct {
	messages []collectedMessage
	mu       sync.RWMutex
}

type collectedMessage struct {
	Time     int64  `json:"time"`
	UserId   uint   `json:"user_id"`
	Nickname string `json:"nickname"`
	Content  string `json:"content"`
	MsgId    uint   `json:"msg_id"`
}

func NewGroupNewsletterPlugin() *GroupNewsletter {
	return &GroupNewsletter{
		Meta: plugin.Meta{
			Name:      "群刊插件",
			HelpWords: "自动收集群消息并生成有趣的群刊，发送 /gn 查看当前收集状态，/gn gen 立即生成群刊",
		},
		groupMsgs:  make(map[uint]*groupMessageBuffer),
		notifyChan: make(chan uint, 100),
	}
}

func (p *GroupNewsletter) Start(ctx context.Context, cfg *viper.Viper) error {
	p.config.baseUrl = cfg.GetString("plugin.group_newsletter.model.base_url")
	p.config.apiKey = cfg.GetString("plugin.group_newsletter.model.api_key")
	p.config.model = cfg.GetString("plugin.group_newsletter.model.model")
	p.config.prompt = cfg.GetString("plugin.group_newsletter.model.prompt")
	p.config.maxToken = cfg.GetInt("plugin.group_newsletter.max_token")
	p.config.msgThreshold = cfg.GetInt("plugin.group_newsletter.msg_threshold")
	p.config.maxMessages = cfg.GetInt("plugin.group_newsletter.max_messages")

	enabledGroups := cfg.GetIntSlice("plugin.group_newsletter.enabled_groups")
	for _, id := range enabledGroups {
		if id > 0 {
			p.config.enabledGroups = append(p.config.enabledGroups, uint(id))
		}
	}

	if p.config.msgThreshold == 0 {
		p.config.msgThreshold = 100
	}

	if p.config.maxMessages == 0 {
		p.config.maxMessages = 500
	}

	if p.config.prompt == "" {
		p.config.prompt = defaultPrompt
	}

	if p.config.baseUrl != "" && p.config.apiKey != "" && p.config.model != "" {
		llm, err := openai.New(
			openai.WithBaseURL(p.config.baseUrl),
			openai.WithToken(p.config.apiKey),
			openai.WithModel(p.config.model),
		)
		if err != nil {
			log.Println("群刊插件初始化LLM失败:", err)
		} else {
			p.llm = llm
		}
	} else {
		log.Println("群刊插件: 未配置LLM，请检查config.yaml中的plugin.group_newsletter配置")
	}

	p.loadFromStorage()

	log.Printf("群刊插件初始化完成, 消息阈值: %d, 最大消息数: %d\n", p.config.msgThreshold, p.config.maxMessages)
	return nil
}

func (p *GroupNewsletter) Awake(_ context.Context, bot bot.Bot) error {
	go p.processLoop(bot)
	return nil
}

func (p *GroupNewsletter) OnGroupMsg(ctx context.Context, bot bot.Bot, cmd command.Command, msg message.Message) (bool, error) {
	if !p.isGroupEnabled(msg.GroupId) {
		return true, nil
	}

	if cmd.Mention && cmd.Name == "gn" {
		return p.handleCommand(bot, cmd, msg)
	}

	p.collectMessage(ctx, bot, msg)

	return true, nil
}

func (p *GroupNewsletter) handleCommand(bot bot.Bot, cmd command.Command, msg message.Message) (bool, error) {
	if p.llm == nil {
		builder := msgchain.Builder().Group()
		builder.Reply(msg.MessageId).Text("群刊插件未正确配置，请检查API配置")
		bot.SendGroupMsg(msg.GroupId, builder.Build())
		return false, nil
	}

	if len(cmd.Args) > 0 && cmd.Args[0] == "gen" {
		count := p.getMessageCount(msg.GroupId)
		if count == 0 {
			builder := msgchain.Builder().Group()
			builder.Reply(msg.MessageId).Text("当前没有收集到消息，无法生成群刊")
			bot.SendGroupMsg(msg.GroupId, builder.Build())
			return false, nil
		}
		go p.generateForGroup(context.Background(), bot, msg.GroupId, true)
		builder := msgchain.Builder().Group()
		builder.Reply(msg.MessageId).Text("正在生成群刊，请稍后...")
		bot.SendGroupMsg(msg.GroupId, builder.Build())
		return false, nil
	}

	count := p.getMessageCount(msg.GroupId)
	builder := msgchain.Builder().Group()
	builder.Reply(msg.MessageId).Text(fmt.Sprintf("当前已收集 %d 条消息，需要 %d 条触发群刊生成", count, p.config.msgThreshold))
	bot.SendGroupMsg(msg.GroupId, builder.Build())
	return false, nil
}

func (p *GroupNewsletter) isGroupEnabled(groupId uint) bool {
	if len(p.config.enabledGroups) == 0 {
		return true
	}
	for _, id := range p.config.enabledGroups {
		if id == groupId {
			return true
		}
	}
	return false
}

func (p *GroupNewsletter) collectMessage(_ context.Context, bot bot.Bot, msg message.Message) {
	p.msgsMutex.Lock()
	defer p.msgsMutex.Unlock()

	if _, ok := p.groupMsgs[msg.GroupId]; !ok {
		p.groupMsgs[msg.GroupId] = &groupMessageBuffer{
			messages: make([]collectedMessage, 0),
		}
	}

	buffer := p.groupMsgs[msg.GroupId]
	buffer.mu.Lock()
	defer buffer.mu.Unlock()

	var content strings.Builder
	for _, m := range msg.Message {
		content.WriteString(m.FriendlyText(
			message.WithGetGroupUserInfo(msg.GroupId, func(groupId, userId uint) (info *message.GroupUserInfo, success bool) {
				return bot.GetGroupUserInfo(groupId, userId)
			}),
		))
	}

	nickname := msg.Sender.Card
	if nickname == "" {
		nickname = msg.Sender.Nickname
	}

	collected := collectedMessage{
		Time:     time.Now().Unix(),
		UserId:   msg.Sender.UserId,
		Nickname: nickname,
		Content:  content.String(),
		MsgId:    msg.MessageId,
	}

	buffer.messages = append(buffer.messages, collected)

	if len(buffer.messages) > p.config.maxMessages {
		buffer.messages = buffer.messages[len(buffer.messages)-p.config.maxMessages:]
	}

	key := "group_" + strconv.FormatUint(uint64(msg.GroupId), 10)
	p.Storage.Set(context.Background(), key, buffer.messages)

	if len(buffer.messages) >= p.config.msgThreshold {
		select {
		case p.notifyChan <- msg.GroupId:
		default:
		}
	}
}

func (p *GroupNewsletter) getMessageCount(groupId uint) int {
	p.msgsMutex.RLock()
	defer p.msgsMutex.RUnlock()

	if buffer, ok := p.groupMsgs[groupId]; ok {
		buffer.mu.RLock()
		defer buffer.mu.RUnlock()
		return len(buffer.messages)
	}
	return 0
}

func (p *GroupNewsletter) processLoop(bot bot.Bot) {
	for {
		groupId := <-p.notifyChan
		p.generateForGroup(context.Background(), bot, groupId, false)
	}
}

func (p *GroupNewsletter) generateForGroup(ctx context.Context, bot bot.Bot, groupId uint, force bool) {
	p.msgsMutex.Lock()
	buffer, ok := p.groupMsgs[groupId]
	if !ok {
		p.msgsMutex.Unlock()
		return
	}

	buffer.mu.Lock()
	if !force && len(buffer.messages) < p.config.msgThreshold {
		buffer.mu.Unlock()
		p.msgsMutex.Unlock()
		return
	}

	if len(buffer.messages) == 0 {
		buffer.mu.Unlock()
		p.msgsMutex.Unlock()
		return
	}

	msgs := make([]collectedMessage, len(buffer.messages))
	copy(msgs, buffer.messages)
	buffer.messages = make([]collectedMessage, 0)
	buffer.mu.Unlock()
	p.msgsMutex.Unlock()

	p.saveToStorage(groupId)

	log.Printf("群 %d 开始生成群刊，共 %d 条消息\n", groupId, len(msgs))

	result, err := p.generateAI(ctx, msgs)
	if err != nil {
		log.Println("生成群刊失败:", err)
		builder := msgchain.Builder().Group()
		builder.Text("群刊生成失败，请稍后再试")
		bot.SendGroupMsg(groupId, builder.Build())
		return
	}

	name := "群刊_" + time.Now().Format("2006-01-02_15-04-05") + "_" + uuid.NewString()[:8] + ".md"

	builder := msgchain.Builder().Group()
	builder.Text("📰 叮！本期群刊已生成，请查收~")
	bot.SendGroupMsg(groupId, builder.Build())

	builder = msgchain.Builder().Group()
	builder.FileBase64(name, base64.StdEncoding.EncodeToString([]byte(result)))
	bot.SendGroupMsg(groupId, builder.Build())
}

func (p *GroupNewsletter) generateAI(ctx context.Context, msgs []collectedMessage) (string, error) {
	msgData, err := json.MarshalIndent(msgs, "", "  ")
	if err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(ctx, time.Minute*3)
	defer cancel()

	response, err := p.llm.GenerateContent(
		ctx,
		[]llms.MessageContent{
			llms.TextParts(llms.ChatMessageTypeSystem, p.config.prompt),
			llms.TextParts(llms.ChatMessageTypeHuman, string(msgData)),
		},
		llms.WithTemperature(1.2),
		llms.WithTopP(0.9),
	)
	if err != nil {
		return "", err
	}

	return response.Choices[0].Content, nil
}

func (p *GroupNewsletter) saveToStorage(groupId uint) {
	p.msgsMutex.RLock()
	defer p.msgsMutex.RUnlock()

	buffer, ok := p.groupMsgs[groupId]
	if !ok {
		return
	}

	buffer.mu.RLock()
	defer buffer.mu.RUnlock()

	key := "group_" + strconv.FormatUint(uint64(groupId), 10)
	p.Storage.Set(context.Background(), key, buffer.messages)
}

func (p *GroupNewsletter) loadFromStorage() {
	keys, err := p.Storage.ScanKeys(context.Background(), "group_*", 100)
	if err != nil {
		log.Println("加载存储消息失败:", err)
		return
	}

	for _, key := range keys {
		var msgs []collectedMessage
		if p.Storage.Get(context.Background(), key, &msgs) {
			if len(msgs) > 0 {
				groupIdStr := strings.TrimPrefix(key, "group_")
				groupId, err := strconv.ParseUint(groupIdStr, 10, 64)
				if err != nil {
					log.Println("解析群ID失败:", err)
					continue
				}
				p.groupMsgs[uint(groupId)] = &groupMessageBuffer{
					messages: msgs,
				}
				log.Printf("群刊插件: 从存储加载群 %d 的 %d 条消息\n", groupId, len(msgs))
			}
		}
	}
}

const defaultPrompt = `你是一个有趣的群刊编辑，负责整理群聊消息并生成有趣的群刊。

群刊要求：
1. 标题要吸引眼球，可以用夸张或幽默的方式
2. 内容要有趣，可以适当调侃群友
3. 可以引用群友的原话，但要标注来源
4. 可以对群友进行有趣的评价或"颁奖"
5. 发现群里的有趣话题、猎奇内容、搞笑对话
6. 语气要活泼有趣，可以用表情符号
7. 适当总结群里的热点话题

输出格式要求：
- 使用Markdown格式
- 包含标题、日期、正文
- 可以包含"今日之星"、"神回复"、"迷惑行为大赏"等有趣栏目

请根据以用户提供的群消息生成一份有趣的群刊`
