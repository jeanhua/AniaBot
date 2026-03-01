package core

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/jeanhua/AniaBot/bot/utils"
	"github.com/jeanhua/AniaBot/common/adapter"
	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/jeanhua/AniaBot/common/msgchain"
	"github.com/jeanhua/AniaBot/common/plugin"
	"github.com/jeanhua/AniaBot/common/storage"
	"github.com/robfig/cron/v3"
	"github.com/spf13/viper"
)

type AniaBot struct {
	ctx    context.Context
	cancel context.CancelFunc

	adapter adapter.Adapter
	plugins []plugin.Plugin
	admin   uint
	cfg     *viper.Viper

	storage     storage.Storage
	restyClient *resty.Client

	pluginSet map[string]struct{}
}

const (
	StartEventTimeout     = time.Minute
	StartCronEventTimeout = time.Minute
	AwakeEventTimeout     = time.Minute

	MsgEventTimeout    = time.Minute * 5
	NoticeEventTimeout = time.Minute * 5
)

type Option func(*AniaBot)

func WithStorage(storage storage.Storage) Option {
	return func(ania *AniaBot) {
		ania.storage = storage
	}
}

func WithConfig(config *viper.Viper) Option {
	return func(ania *AniaBot) {
		ania.cfg = config
	}
}

func WithResty(restyClient *resty.Client) Option {
	return func(ania *AniaBot) {
		ania.restyClient = restyClient
	}
}

func NewAniaBot(adapter adapter.Adapter, option ...Option) *AniaBot {
	ctx, cancel := context.WithCancel(context.Background())
	ania := &AniaBot{
		ctx:       ctx,
		cancel:    cancel,
		adapter:   adapter,
		pluginSet: map[string]struct{}{},
		plugins:   make([]plugin.Plugin, 0),
	}
	for _, op := range option {
		op(ania)
	}
	return ania
}

func (ania *AniaBot) Run() {
	sort.SliceStable(ania.plugins, func(i, j int) bool {
		return ania.plugins[i].GetMeta().Order < ania.plugins[j].GetMeta().Order
	})
	trigger := adapter.TriggerWrapper{
		OnGroupMsg:          ania.onGroupEvent,
		OnFriendMsg:         ania.onFriendEvent,
		OnGroupUpload:       ania.onGroupUploadEvent,
		OnGroupAdmin:        ania.onGroupAdminEvent,
		OnGroupDecrease:     ania.onGroupDecreaseEvent,
		OnGroupIncrease:     ania.onGroupIncreaseEvent,
		OnGroupBan:          ania.onGroupBanEvent,
		OnFriendAdd:         ania.onFriendAddEvent,
		OnGroupRecall:       ania.onGroupRecallEvent,
		OnFriendRecall:      ania.onFriendRecallEvent,
		OnPoke:              ania.onPokeEvent,
		OnLuckyKing:         ania.onLuckyKingEvent,
		OnHonor:             ania.onHonorEvent,
		OnGroupMsgEmojiLike: ania.onGroupMsgEmojiLikeEvent,
		OnEssence:           ania.onEssenceEvent,
		OnGroupCard:         ania.onGroupCardEvent,
	}
	ania.adapter.SetTrigger(trigger)
	// config
	if ania.cfg == nil {
		cfg := viper.New()
		cfg.AddConfigPath("./")
		cfg.SetConfigName("config.dev")
		if err := cfg.ReadInConfig(); err == nil {
			Logger().Println("使用开发环境配置: config.dev.yaml")
		} else {
			cfg.SetConfigName("config")
			if err := cfg.ReadInConfig(); err != nil {
				Logger().Fatalf("无法读取配置文件: %v", err)
			}
			Logger().Println("使用默认配置: config.yaml")
		}
		ania.cfg = cfg
	}

	// storage
	if ania.storage == nil {
		ania.storage = NewAniaRedisStorage(context.Background(),
			ania.cfg.GetString("bot.store.redis"),
			ania.cfg.GetString("bot.store.password"),
			ania.cfg.GetInt("bot.store.db"))
	}

	// resty
	if ania.restyClient == nil {
		ania.restyClient = resty.New()
	}

	ania.admin = ania.cfg.GetUint("bot.admin_id")

	// 初始化事件
	Logger().Println("开始初始化插件...")
	for _, p := range ania.plugins {
		Logger().Println("初始化插件: ", p.GetMeta().Name)
		safeExecute("初始化", p, func(p plugin.Plugin) {
			// DI
			encodeName := base64.StdEncoding.EncodeToString([]byte(p.GetMeta().Name))
			p.SetStorage(ania.storage.Clone(encodeName))
			p.SetRestyClient(ania.restyClient)
			p.SetLogger(log.New(os.Stderr, fmt.Sprintf("[%s] ", p.GetMeta().Name), log.Ltime))

			startCtx, cancel := context.WithTimeout(ania.ctx, StartEventTimeout)
			p.Start(startCtx, ania.cfg)
			cancel()
		})
	}

	// 初始化cron
	c := cron.New()
	Logger().Println("开始初始化cron...")
	for _, p := range ania.plugins {
		safeExecute("初始化cron", p, func(p plugin.Plugin) {
			startCtx, cancel := context.WithTimeout(ania.ctx, StartCronEventTimeout)
			p.StartCron(startCtx, ania, c)
			cancel()
		})
	}
	Logger().Println("初始化cron完成")
	c.Start()
	defer c.Stop()

	awakeTimer := time.AfterFunc(time.Second, func() {
		Logger().Println("Awake...")
		for _, p := range ania.plugins {
			safeExecute("Awake", p, func(p plugin.Plugin) {
				awakeCtx, cancel := context.WithTimeout(ania.ctx, AwakeEventTimeout)
				p.Awake(awakeCtx, ania)
				cancel()
			})
		}
	})
	defer awakeTimer.Stop()
	ania.adapter.Serve(ania.cfg)
}

func (ania *AniaBot) onGroupEvent(msg message.Message) {
	defer func() {
		if err := recover(); err != nil {
			Logger().Println("群聊消息事件触发错误: ", err)
		}
	}()
	if msg.Sender.UserId == msg.SelfId {
		return
	}

	cmd := utils.ParseCommand(msg)
	if cmd.Name == "help" && cmd.Mention {
		var pluginInfo strings.Builder
		pluginInfo.WriteString("\n欢迎使用AniaBot，已加载插件:")
		idx := 1
		for _, p := range ania.plugins {
			if p.GetMeta().AdminOnly && msg.Sender.UserId != ania.admin {
				continue
			}
			pName := p.GetMeta().Name
			pHelpWords := p.GetMeta().HelpWords
			pluginInfo.WriteString(fmt.Sprintf("\n%d. %s: %s", idx, pName, pHelpWords))
			idx += 1
		}
		c := msgchain.Builder().Group()
		c.Mention(msg.Sender.UserId)
		c.Text(pluginInfo.String())
		_, ok := ania.SendGroupMsg(msg.GroupId, c.Build())
		if !ok {
			Logger().Println("Bot消息发送失败，无法响应 /help")
		}
		return
	}

	for _, p := range ania.plugins {
		next := safeExecuteWithReturn("群聊消息事件", p, func(p plugin.Plugin) bool {
			msgCtx, cancel := context.WithTimeout(ania.ctx, MsgEventTimeout)
			next, err := p.OnGroupMsg(msgCtx, ania, cmd, msg)
			logError(err, p, "群聊消息事件")
			cancel()
			return next
		})
		if !next {
			break
		}
	}
}

func (ania *AniaBot) onFriendEvent(msg message.Message) {
	defer func() {
		if err := recover(); err != nil {
			Logger().Println("私聊消息事件触发错误: ", err)
		}
	}()
	if msg.Sender.UserId == msg.SelfId {
		return
	}

	cmd := utils.ParseCommand(msg)
	if cmd.Name == "help" {
		var pluginInfo strings.Builder
		pluginInfo.WriteString("欢迎使用AniaBot，已加载插件:")
		idx := 1
		for _, p := range ania.plugins {
			if p.GetMeta().AdminOnly && msg.Sender.UserId != ania.admin {
				continue
			}
			pName := p.GetMeta().Name
			pHelpWords := p.GetMeta().HelpWords
			pluginInfo.WriteString(fmt.Sprintf("\n%d. %s: %s", idx, pName, pHelpWords))
			idx += 1
		}
		c := msgchain.Builder().Friend()
		c.Text(pluginInfo.String())
		_, ok := ania.SendFriendMsg(msg.Sender.UserId, c.Build())
		if !ok {
			Logger().Println("Bot消息发送失败，无法响应 /help")
		}
		return
	}

	for _, p := range ania.plugins {
		next := safeExecuteWithReturn("私聊消息事件", p, func(p plugin.Plugin) bool {
			msgCtx, cancel := context.WithTimeout(ania.ctx, MsgEventTimeout)
			next, err := p.OnFriendMsg(msgCtx, ania, cmd, msg)
			logError(err, p, "私聊消息事件")
			cancel()
			return next
		})
		if !next {
			break
		}
	}
}

func (ania *AniaBot) AddPlugin(plugins ...plugin.Plugin) {
	for _, p := range plugins {
		meta := p.GetMeta()
		if _, ok := ania.pluginSet[meta.Name]; ok {
			panic("插件名称相同，请检查插件是否重复加载")
		}
		ania.pluginSet[meta.Name] = struct{}{}
		ania.plugins = append(ania.plugins, p)
		Logger().Println("已添加插件: ", p.GetMeta().Name)
	}
}

func (ania *AniaBot) Stop() {
	ania.cancel()
}

func (ania *AniaBot) SendGroupMsg(groupId uint, chain msgchain.GroupChain) (msgId uint, success bool) {
	return ania.adapter.SendGroupMsg(groupId, chain)
}

func (ania *AniaBot) SendFriendMsg(userID uint, chain msgchain.FriendChain) (msgId uint, success bool) {
	return ania.adapter.SendFriendMsg(userID, chain)
}

func (ania *AniaBot) SendGroupAIVoiceMsg(groupId uint, character, msg string) (msgId uint, success bool) {
	return ania.adapter.SendGroupAIVoiceMsg(groupId, character, msg)
}

func (ania *AniaBot) SendPokeMsg(userId uint, groupId *uint) {
	ania.adapter.SendPokeMsg(userId, groupId)
}

func (ania *AniaBot) GetMsgDetail(msgId uint) (*message.Message, bool) {
	return ania.adapter.GetMsgDetail(msgId)
}

func (ania *AniaBot) SendGroupForwardMsg(groupId uint, chain msgchain.GroupForwardChain) (msgId uint, success bool) {
	return ania.adapter.SendGroupForwardMsg(groupId, chain)
}

func (ania *AniaBot) SendFriendForwardMsg(userId uint, chain msgchain.FriendForwardChain) (msgId uint, success bool) {
	return ania.adapter.SendFriendForwardMsg(userId, chain)
}

func (ania *AniaBot) GetForwardMsg(msgId string) (msgs *[]message.Message, success bool) {
	return ania.adapter.GetForwardMsg(msgId)
}

func (ania *AniaBot) GetGroupUserInfo(groupId, userId uint) (*message.GroupUserInfo, bool) {
	return ania.adapter.GetGroupUserInfo(groupId, userId)
}

func (ania *AniaBot) GetNCrkey() ([]message.NCrkey, bool) {
	return ania.adapter.GetNCrkey()
}

func (ania *AniaBot) GetFriendList() (*[]message.Friend, bool) {
	return ania.adapter.GetFriendList()
}
