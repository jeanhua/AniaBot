package groupnewsletter

import (
	"context"
	"sync"

	"github.com/jeanhua/AniaBot/common/bot"
	"github.com/jeanhua/AniaBot/common/model/command"
	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/jeanhua/AniaBot/common/plugin"
	"github.com/spf13/viper"
)

type GroupNewsletter struct {
	plugin.Meta

	llm    llmClient
	config newsletterConfig

	// 消息 buffer，按 groupId 索引
	groupMsgs map[uint]*groupMessageBuffer
	msgsMu    sync.RWMutex

	// 消息阈值触发通知
	notifyChan chan uint

	// 防止同一群重复生成
	generating map[uint]struct{}
	generateMu sync.Mutex

	// 异步持久化队列
	saveChan chan uint

	// 插件自身生命周期，不依赖框架传入的短生命周期 ctx
	pluginCtx context.Context
	cancel    context.CancelFunc
}

func NewGroupNewsletterPlugin() *GroupNewsletter {
	return &GroupNewsletter{
		Meta: plugin.Meta{
			Name:      "群刊插件",
			HelpWords: "自动收集群消息并生成有趣的群刊，发送 /gn 查看当前收集状态，/gn gen 立即生成群刊",
		},
		groupMsgs:  make(map[uint]*groupMessageBuffer),
		notifyChan: make(chan uint, 100),
		generating: make(map[uint]struct{}),
		saveChan:   make(chan uint, 200),
	}
}

func (p *GroupNewsletter) Start(_ context.Context, cfg *viper.Viper) error {
	p.config = loadConfig(cfg)

	if err := p.initLLM(); err != nil {
		p.Logger.Printf("初始化 LLM 失败: %v", err)
	}

	p.pluginCtx, p.cancel = context.WithCancel(context.Background())

	p.loadFromStorage()

	p.Logger.Printf("初始化完成，消息阈值: %d，最大消息数: %d",
		p.config.msgThreshold, p.config.maxMessages)
	return nil
}

func (p *GroupNewsletter) Stop() {
	if p.cancel != nil {
		p.cancel()
	}
}

func (p *GroupNewsletter) Awake(_ context.Context, b bot.Bot) error {
	go p.processLoop(b)
	go p.saveLoop()
	return nil
}

func (p *GroupNewsletter) OnGroupMsg(ctx context.Context, b bot.Bot, cmd command.Command, msg message.Message) (bool, error) {
	if !p.isGroupEnabled(msg.GroupId) {
		return true, nil
	}

	if cmd.Mention && cmd.Name == "gn" {
		return p.handleCommand(b, cmd, msg)
	}

	p.collectMessage(ctx, b, msg)
	return true, nil
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

// processLoop 监听阈值通知，串行处理生成任务
func (p *GroupNewsletter) processLoop(b bot.Bot) {
	for {
		select {
		case <-p.pluginCtx.Done():
			p.Logger.Println("processLoop 退出")
			return
		case groupId := <-p.notifyChan:
			p.generateForGroup(p.pluginCtx, b, groupId, false)
		}
	}
}
