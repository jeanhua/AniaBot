package githubrepoer

import (
	"context"
	"encoding/base64"
	"errors"
	"net/url"
	"time"

	"github.com/google/uuid"
	"github.com/jeanhua/AniaBot/common/aniaerror"
	"github.com/jeanhua/AniaBot/common/bot"
	"github.com/jeanhua/AniaBot/common/model/command"
	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/jeanhua/AniaBot/common/msgchain"
	"github.com/jeanhua/AniaBot/common/plugin"
	"github.com/jeanhua/AniaBot/custom/component/md2img"
	"github.com/spf13/viper"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
)

type GithubRepoer struct {
	plugin.Meta
	pendding  chan work
	llm       *openai.LLM
	maxToken  int
	fmt       string
	llmConfig struct {
		baseUrl string
		apiKey  string
		model   string
		prompt  string
	}
}

var helpWords = `
指令示例 /gr [github项目链接] [...参数选项] , 参数如下:
1. --include=[pattern] 只包含此pattern的文件 e.g. --include=**/*.go,**/*.md
2. --exclude=[pattern] 排除此pattern的文件 e.g. --exclude=**/*_test.go
3. --compress 去除所有函数实现，只保留签名
4. --del-comment 去除所有的注释
5. --del-emptyline 去除所有空行`

func NewGithubRepoer(maxWork int) *GithubRepoer {
	pd := make(chan work, maxWork)
	return &GithubRepoer{
		Meta: plugin.Meta{
			Name:      "GithubRepoer插件",
			HelpWords: "发送 /gr [github项目链接] 即可获取项目分析报告, 发送 /gr help 获取参数详情",
		},
		pendding: pd,
	}
}

// Start 插件初始化事件
func (p *GithubRepoer) Start(ctx context.Context, cfg *viper.Viper) error {
	p.llmConfig.baseUrl = cfg.GetString("plugin.github_repoer.model.base_url")
	p.llmConfig.apiKey = cfg.GetString("plugin.github_repoer.model.api_key")
	p.llmConfig.model = cfg.GetString("plugin.github_repoer.model.model")
	p.llmConfig.prompt = cfg.GetString("plugin.github_repoer.model.prompt")
	p.maxToken = cfg.GetInt("plugin.github_repoer.max_token")
	p.fmt = cfg.GetString("plugin.github_repoer.fmt")
	if p.fmt == "" {
		p.fmt = "md"
	}

	llm, err := openai.New(
		openai.WithBaseURL(p.llmConfig.baseUrl),
		openai.WithToken(p.llmConfig.apiKey),
		openai.WithModel(p.llmConfig.model),
	)
	if err != nil {
		return aniaerror.ParameterInitializeError
	}
	p.llm = llm
	return nil
}

// Awake Bot启动完成事件
func (p *GithubRepoer) Awake(ctx context.Context, bot bot.Bot) error {
	go p.workFunc(bot)
	return nil
}

// OnGroupMsg 收到群聊消息触发事件
func (p *GithubRepoer) OnGroupMsg(ctx context.Context, bot bot.Bot, cmd command.Command, msg message.Message) (bool, error) {
	if cmd.Mention && cmd.Name == "gr" {
		if len(cmd.Args) == 0 {
			builder := msgchain.Builder().Group()
			builder.Mention(msg.Sender.UserId).Text(" 请输入完整指令，如 /gr https://github.com/jeanhua/AniaBot")
			bot.SendGroupMsg(msg.GroupId, builder.Build())
			return false, nil
		}

		if cmd.Args[0] == "help" {
			builder := msgchain.Builder().Group()
			builder.Mention(msg.Sender.UserId).Text(helpWords)
			bot.SendGroupMsg(msg.GroupId, builder.Build())
			return false, nil
		}

		targetUrl := cmd.Args[0]
		if _, err := url.Parse(targetUrl); err != nil {
			builder := msgchain.Builder().Group()
			builder.Mention(msg.Sender.UserId).Text(" 请输入之正确的链接").Face(14)
			bot.SendGroupMsg(msg.GroupId, builder.Build())
			return false, nil
		}

		w := parseCmd(cmd.Args)
		w.target = TargetGroup
		w.userId = msg.Sender.UserId
		w.groupId = msg.GroupId
		w.msgId = msg.MessageId
		w.repoURL = targetUrl

		select {
		case p.pendding <- w:
			builder := msgchain.Builder().Group()
			builder.Reply(msg.MessageId).Mention(msg.Sender.UserId).Text(" 正在生成中，请稍后").Face(178)
			bot.SendGroupMsg(msg.GroupId, builder.Build())
		default:
			builder := msgchain.Builder().Group()
			builder.Mention(msg.Sender.UserId).Text(" 请求队列已满，请稍后再试哦").Face(14)
			bot.SendGroupMsg(msg.GroupId, builder.Build())
		}
		return false, nil
	}
	return true, nil
}

// OnFriendMsg 收到私聊消息触发事件
func (p *GithubRepoer) OnFriendMsg(ctx context.Context, bot bot.Bot, cmd command.Command, msg message.Message) (bool, error) {
	if cmd.Name == "gr" {
		if len(cmd.Args) == 0 {
			builder := msgchain.Builder().Friend()
			builder.Text("请输入完整指令，如 /gr https://github.com/jeanhua/AniaBot")
			bot.SendFriendMsg(msg.Sender.UserId, builder.Build())
			return false, nil
		}

		if cmd.Args[0] == "help" {
			builder := msgchain.Builder().Friend()
			builder.Text(helpWords)
			bot.SendFriendMsg(msg.Sender.UserId, builder.Build())
			return false, nil
		}

		targetUrl := cmd.Args[0]
		if _, err := url.Parse(targetUrl); err != nil {
			builder := msgchain.Builder().Friend()
			builder.Text("请输入之正确的链接").Face(14)
			bot.SendFriendMsg(msg.Sender.UserId, builder.Build())
			return false, nil
		}

		w := parseCmd(cmd.Args)
		w.target = TargetFriend
		w.userId = msg.Sender.UserId
		w.groupId = msg.GroupId
		w.msgId = msg.MessageId
		w.repoURL = targetUrl

		select {
		case p.pendding <- w:
			builder := msgchain.Builder().Friend()
			builder.Text("正在生成中，请稍后").Face(178)
			bot.SendFriendMsg(msg.Sender.UserId, builder.Build())
		default:
			builder := msgchain.Builder().Friend()
			builder.Text("请求队列已满，请稍后再试哦").Face(14)
			bot.SendFriendMsg(msg.Sender.UserId, builder.Build())
		}
		return false, nil
	}
	return true, nil
}

func (p *GithubRepoer) workFunc(bot bot.Bot) {
	for {
		w := <-p.pendding
		p.Logger.Info("正在生产github报告", "repoURL", w.repoURL, "compress", w.compress, "delComment", w.delComment, "delEmptyLine", w.delEmptyLine, "maxToken", p.maxToken, "include", w.include, "exclude", w.exclude)
		info, err := getRepoInfo(p.RestyClient, w.repoURL, w.compress, w.delComment, w.delEmptyLine, p.maxToken, w.include, w.exclude)
		if err != nil {
			onErr(bot, w, err)
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute*3)
		result, err := p.generateAI(ctx, info)
		cancel()
		if err != nil {
			onErr(bot, w, err)
			continue
		}

		name := "Github项目质量报告_" + uuid.NewString() + ".md"

		if w.target == TargetGroup {
			builder := msgchain.Builder().Group()
			builder.Reply(w.msgId).Mention(w.userId).Text(" 叮! 生成成功, 正在发送报告...").Face(6)
			bot.SendGroupMsg(w.groupId, builder.Build())

			p.sendResult(bot, w.target, w.groupId, w.userId, result, name)
		} else {
			builder := msgchain.Builder().Friend()
			builder.Reply(w.msgId).Text("叮! 生成成功, 正在发送报告...").Face(6)
			bot.SendFriendMsg(w.userId, builder.Build())

			p.sendResult(bot, w.target, w.groupId, w.userId, result, name)
		}
	}
}

func onErr(bot bot.Bot, w work, err error) {
	noticeText := "请求失败，请稍后再试"
	if errors.Is(err, OutOfContextError) {
		noticeText = "项目过大，请使用参数选项减少无关代码，参考 /gr help"
	}
	if w.target == TargetGroup {
		builder := msgchain.Builder().Group()
		builder.Reply(w.msgId).Mention(w.userId).Text(" " + noticeText).Face(6)
		bot.SendGroupMsg(w.groupId, builder.Build())
	} else {
		builder := msgchain.Builder().Friend()
		builder.Reply(w.msgId).Text(noticeText).Face(6)
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
		llms.WithTemperature(1.2),
		llms.WithTopP(0.9),
		llms.WithTopK(100),
	)
	if err != nil {
		return "", err
	}
	return response.Choices[0].Content, nil
}

func (p *GithubRepoer) sendResult(bot bot.Bot, target wTarget, groupId, userId message.QID, result, name string) {
	if p.fmt == "jpg" {
		imgData, err := md2img.GetImage(result)
		if target == TargetGroup {
			if err != nil {
				p.Logger.Error("md转图片失败", "error", err)
				bot.SendGroupMsg(groupId, msgchain.Builder().Group().
					Text("转换失败，请查看原始md文件").Face(14).
					Build())
			}
			bot.SendGroupMsg(groupId, msgchain.Builder().Group().
				ImageBase64(base64.StdEncoding.EncodeToString(imgData)).
				Build())
		} else {
			if err != nil {
				p.Logger.Error("md转图片失败", "error", err)
				bot.SendFriendMsg(userId, msgchain.Builder().Friend().
					Text("转换失败，请查看原始md文件").Face(14).
					Build())
			}
			bot.SendFriendMsg(userId, msgchain.Builder().Friend().
				ImageBase64(base64.StdEncoding.EncodeToString(imgData)).
				Build())
		}
	} else {
		if target == TargetGroup {
			bot.SendGroupMsg(groupId, msgchain.Builder().Group().
				FileBase64(name, base64.StdEncoding.EncodeToString([]byte(result))).
				Build())
		} else {
			bot.SendFriendMsg(userId, msgchain.Builder().Friend().
				FileBase64(name, base64.StdEncoding.EncodeToString([]byte(result))).
				Build())
		}
	}
}
