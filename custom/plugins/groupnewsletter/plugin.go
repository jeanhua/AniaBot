package groupnewsletter

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/jeanhua/AniaBot/common/bot"
	"github.com/jeanhua/AniaBot/common/model/command"
	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/jeanhua/AniaBot/common/msgchain"
	"github.com/jeanhua/AniaBot/common/plugin"
	"github.com/jeanhua/AniaBot/custom/component/md2img"
	"github.com/spf13/viper"
)

type GroupNewsletter struct {
	plugin.Meta

	llm    llmClient
	config newsletterConfig

	// 消息 buffer，按 groupId 索引
	groupMsgs map[message.QID]*groupMessageBuffer
	msgsMu    sync.RWMutex

	// 消息阈值触发通知
	notifyChan chan message.QID

	// 防止同一群重复生成
	generating map[message.QID]struct{}
	generateMu sync.Mutex

	// 异步持久化队列
	saveChan chan message.QID

	// 插件自身生命周期，不依赖框架传入的短生命周期 ctx
	pluginCtx context.Context
	cancel    context.CancelFunc

	// md2img 组件
	md2img *md2img.Md2Img
}

func NewGroupNewsletterPlugin() *GroupNewsletter {
	return &GroupNewsletter{
		Meta: plugin.Meta{
			Name:      "群刊插件",
			HelpWords: "自动收集群消息并生成有趣的群刊，发送 /gn 查看当前收集状态，/gn gen 立即生成群刊",
			Order:     plugin.LevelNormal,
			ShowFor:   plugin.ShowForGroup,
			Author:    "jeanhua",
			Version:   "1.0.0",
		},
		groupMsgs:  make(map[message.QID]*groupMessageBuffer),
		notifyChan: make(chan message.QID, 100),
		generating: make(map[message.QID]struct{}),
		saveChan:   make(chan message.QID, 200),
	}
}

func (p *GroupNewsletter) Start(_ context.Context, cfg *viper.Viper) error {
	p.config = loadConfig(cfg)

	for _, groupId := range p.config.enabledGroups {
		p.Logger.Info("注册群聊", "groupId", groupId)
	}

	if err := p.initLLM(); err != nil {
		p.Logger.Error("初始化 LLM 失败", "error", err)
	}

	p.pluginCtx, p.cancel = context.WithCancel(context.Background())

	p.loadFromStorage()

	// 初始化 md2img 组件
	p.md2img = md2img.NewMd2Img(cfg.GetString("component.md2img.apipoint"), p.RestyClient)

	p.Logger.Info("初始化完成", "msgThreshold", p.config.msgThreshold, "maxMessages", p.config.maxMessages)
	return nil
}

func (p *GroupNewsletter) Stop() {
	if p.cancel != nil {
		p.cancel()
	}
}

func (p *GroupNewsletter) Awake(_ context.Context, b bot.Bot) error {
	b.Go("GroupNewsletter插件处理循环线程", func() {
		p.processLoop(b)
	})
	b.Go("GroupNewsletter插件保存循环线程", func() {
		p.saveLoop()
	})
	return nil
}

func (p *GroupNewsletter) OnGroupMsg(ctx context.Context, b bot.Bot, cmd command.Command, msg message.Message) (bool, error) {
	if !p.isGroupEnabled(msg.GroupId) {
		if cmd.Mention && cmd.Name == "gn" {
			builder := msgchain.Builder().Group()
			builder.Mention(msg.Sender.UserId)
			builder.Text(" 本群未启用群刊插件，无法处理此命令")
			b.SendGroupMsg(msg.GroupId, builder.Build())
			return false, nil
		}
		return true, nil
	}

	if cmd.Mention && cmd.Name == "gn" {
		return p.handleCommand(b, cmd, msg)
	}

	p.collectMessage(ctx, b, msg)
	return true, nil
}

func (p *GroupNewsletter) OnFriendMsg(ctx context.Context, bot bot.Bot, cmd command.Command, msg message.Message) (bool, error) {
	if msg.Sender.UserId != p.SystemConfig.AdminId {
		return true, nil
	}

	if cmd.Name == "gn" {
		strBuilder := &strings.Builder{}
		strBuilder.WriteString("群刊插件已收集消息:")
		p.msgsMu.RLock()
		for groupId := range p.groupMsgs {
			strBuilder.WriteString("\n")
			info, ok := bot.GetGroupDetail(groupId)
			if ok {
				strBuilder.WriteString(fmt.Sprintf("[%s %d]", info.GroupName, groupId))
			} else {
				strBuilder.WriteString(fmt.Sprintf("[%d]", groupId))
			}
			strBuilder.WriteString(": ")
			strBuilder.WriteString(fmt.Sprintf("%d 条", p.getMessageCount(groupId)))
		}
		p.msgsMu.RUnlock()

		builder := msgchain.Builder().Friend()
		builder.Text(strBuilder.String())
		bot.SendFriendMsg(msg.Sender.UserId, builder.Build())
		return false, nil
	}

	return true, nil
}

func (p *GroupNewsletter) isGroupEnabled(groupId message.QID) bool {
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

// processLoop 监听阈值通知，串行处理生成任务
func (p *GroupNewsletter) processLoop(b bot.Bot) {
	for {
		select {
		case <-p.pluginCtx.Done():
			p.Logger.Info("processLoop 退出")
			return
		case groupId := <-p.notifyChan:
			p.generateForGroup(p.pluginCtx, b, groupId, false)
		}
	}
}
