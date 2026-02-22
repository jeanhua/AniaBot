package githubrepoer

import (
	"context"
	"encoding/base64"
	"log"
	"net/url"
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

type GithubRepoer struct {
	plugin.Meta
	pendding  chan work
	llm       *openai.LLM
	llmConfig struct {
		baseUrl string
		apiKey  string
		model   string
		prompt  string
	}
}

func NewGithubRepoer(maxPendding int) *GithubRepoer {
	pd := make(chan work, maxPendding)
	return &GithubRepoer{
		Meta: plugin.Meta{
			Name:      "GithubRepoer插件",
			HelpWords: "发送 /gr [github项目链接] 即可获取项目分析报告, 可选参数--compress=true",
		},
		pendding: pd,
	}
}

// Start 插件初始化事件
func (p *GithubRepoer) Start(cfg *viper.Viper) {
	p.llmConfig.baseUrl = cfg.GetString("plugin.github_repoer.model.base_url")
	p.llmConfig.apiKey = cfg.GetString("plugin.github_repoer.model.api_key")
	p.llmConfig.model = cfg.GetString("plugin.github_repoer.model.model")
	p.llmConfig.prompt = cfg.GetString("plugin.github_repoer.model.prompt")

	llm, err := openai.New(
		openai.WithBaseURL(p.llmConfig.baseUrl),
		openai.WithToken(p.llmConfig.apiKey),
		openai.WithModel(p.llmConfig.model),
	)
	if err != nil {
		panic("无法创建model实例")
	}
	p.llm = llm
}

// Awake Bot启动完成事件
func (p *GithubRepoer) Awake(bot bot.Bot) {
	go p.workFunc(bot)
}

// OnGroupMsg 收到群聊消息触发事件
func (p *GithubRepoer) OnGroupMsg(bot bot.Bot, cmd command.Command, msg message.Message) bool {
	if cmd.Mention && cmd.Name == "gr" {
		if len(cmd.Args) == 0 {
			builder := msgchain.Builder().Group()
			builder.Mention(msg.Sender.UserId).Text(" 请输入完整指令，如 /gr https://github.com/jeanhua/AniaBot")
			bot.SendGroupMsg(msg.GroupId, builder.Build())
			return false
		}

		targetUrl := cmd.Args[0]
		if _, err := url.Parse(targetUrl); err != nil {
			builder := msgchain.Builder().Group()
			builder.Mention(msg.Sender.UserId).Text(" 请输入之正确的链接").Face(14)
			bot.SendGroupMsg(msg.GroupId, builder.Build())
			return false
		}

		compress := false
		for _, arg := range cmd.Args {
			if arg == "--compress=true" {
				compress = true
			}
		}

		select {
		case p.pendding <- work{
			target:   TargetGroup,
			userId:   msg.Sender.UserId,
			groupId:  msg.GroupId,
			msgId:    msg.MessageId,
			compress: compress,
			repoURL:  targetUrl,
		}:
		default:
			builder := msgchain.Builder().Group()
			builder.Mention(msg.Sender.UserId).Text(" 请求队列已满，请稍后再试哦").Face(14)
			bot.SendGroupMsg(msg.GroupId, builder.Build())
		}
		return false
	}
	return true
}

// OnFriendMsg 收到私聊消息触发事件
func (p *GithubRepoer) OnFriendMsg(bot bot.Bot, cmd command.Command, msg message.Message) bool {
	if cmd.Name == "gr" {
		if len(cmd.Args) == 0 {
			builder := msgchain.Builder().Friend()
			builder.Text("请输入完整指令，如 /gr https://github.com/jeanhua/AniaBot")
			bot.SendFriendMsg(msg.Sender.UserId, builder.Build())
			return false
		}

		targetUrl := cmd.Args[0]
		if _, err := url.Parse(targetUrl); err != nil {
			builder := msgchain.Builder().Friend()
			builder.Text("请输入之正确的链接").Face(14)
			bot.SendFriendMsg(msg.Sender.UserId, builder.Build())
			return false
		}

		compress := false
		for _, arg := range cmd.Args {
			if arg == "--compress=true" {
				compress = true
			}
		}

		select {
		case p.pendding <- work{
			target:   TargetFriend,
			userId:   msg.Sender.UserId,
			msgId:    msg.MessageId,
			compress: compress,
			repoURL:  targetUrl,
		}:
		default:
			builder := msgchain.Builder().Friend()
			builder.Text("请求队列已满，请稍后再试哦").Face(14)
			bot.SendFriendMsg(msg.Sender.UserId, builder.Build())
		}
		return false
	}
	return true
}

func (p *GithubRepoer) workFunc(bot bot.Bot) {
	for {
		w := <-p.pendding
		log.Println("正在生产github报告:", w.repoURL)
		info, err := getRepoInfo(w.repoURL, w.compress)
		if err != nil {
			onErr(bot, w)
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute*3)
		result, err := p.generateAI(ctx, info)
		cancel()
		if err != nil {
			onErr(bot, w)
			continue
		}

		name := "Github项目质量报告_" + uuid.NewString() + ".md"

		if w.target == TargetGroup {
			builder := msgchain.Builder().Group()
			builder.Reply(w.msgId).Mention(w.userId).Text(" 叮! 生成成功, 正在发送报告...").Face(6)
			bot.SendGroupMsg(w.groupId, builder.Build())

			builder = msgchain.Builder().Group()
			builder.FileBase64(name, base64.StdEncoding.EncodeToString([]byte(result)))
			bot.SendGroupMsg(w.groupId, builder.Build())
		} else {
			builder := msgchain.Builder().Friend()
			builder.Reply(w.msgId).Text("叮! 生成成功, 正在发送报告...").Face(6)
			bot.SendFriendMsg(w.userId, builder.Build())

			builder = msgchain.Builder().Friend()
			builder.FileBase64(name, base64.StdEncoding.EncodeToString([]byte(result)))
			bot.SendFriendMsg(w.userId, builder.Build())
		}
	}
}

func onErr(bot bot.Bot, w work) {
	if w.target == TargetGroup {
		builder := msgchain.Builder().Group()
		builder.Reply(w.msgId).Mention(w.userId).Text(" 请求失败，请稍后再试").Face(6)
		bot.SendGroupMsg(w.groupId, builder.Build())
	} else {
		builder := msgchain.Builder().Friend()
		builder.Reply(w.msgId).Text("请求失败，请稍后再试").Face(6)
		bot.SendFriendMsg(w.userId, builder.Build())
	}
}

func (p *GithubRepoer) generateAI(ctx context.Context, info string) (string, error) {
	response, err := p.llm.GenerateContent(
		ctx,
		[]llms.MessageContent{
			llms.TextParts(llms.ChatMessageTypeSystem, p.llmConfig.prompt),
			llms.TextParts(llms.ChatMessageTypeHuman, info),
		},
	)
	if err != nil {
		return "", err
	}
	return response.Choices[0].Content, nil
}
