package core

import (
	"context"
	_ "embed"
	"encoding/base64"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"sync/atomic"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/jeanhua/AniaBot/bot/utils"
	"github.com/jeanhua/AniaBot/common/adapter"
	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/jeanhua/AniaBot/common/msgchain"
	"github.com/jeanhua/AniaBot/common/plugin"
	"github.com/jeanhua/AniaBot/common/plugininfo"
	"github.com/jeanhua/AniaBot/common/storage"
	"github.com/robfig/cron/v3"
	"github.com/spf13/viper"
)

type AniaBot struct {
	ctx    context.Context
	cancel context.CancelFunc

	adapter adapter.Adapter
	plugins []plugin.Plugin
	cfg     *viper.Viper

	storage     storage.Storage
	restyClient *resty.Client

	logger *slog.Logger

	// 插件名称集合
	pluginSet map[string]struct{}

	// goroutine数目
	goroutineNum atomic.Int32
}

const (
	StartEventTimeout     = time.Minute
	StartCronEventTimeout = time.Minute
	AwakeEventTimeout     = time.Minute

	MsgEventTimeout    = time.Minute * 5
	NoticeEventTimeout = time.Minute * 5
)

//go:embed logo.txt
var LogoASCII string

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

func WithLogger(logger *slog.Logger) Option {
	return func(ania *AniaBot) {
		ania.logger = logger
		inlogger = logger
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
	if ania.logger == nil {
		ania.logger = Logger()
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
			Logger().Info("使用开发环境配置: config.dev.yaml")
		} else {
			cfg.SetConfigName("config")
			if err := cfg.ReadInConfig(); err != nil {
				Logger().Error("无法读取配置文件: %v", "error", err)
				os.Exit(1)
			}
			Logger().Info("使用默认配置: config.yaml")
		}
		ania.cfg = cfg
	}

	// storage
	if ania.storage == nil {
		ania.storage = NewAniaRedisStorage(context.Background(),
			ania.cfg.GetString("bot.store.redis"),
			ania.cfg.GetString("bot.store.password"),
			ania.cfg.GetInt("bot.store.db"),
			Logger().WithGroup("Redis"))
	}

	// resty
	if ania.restyClient == nil {
		ania.restyClient = resty.New()
	}

	// 初始化事件
	Logger().Info("开始初始化插件...")
	for _, p := range ania.plugins {
		Logger().Info("初始化插件: ", "name", p.GetMeta().Name)
		safeExecute("初始化", p, func(p plugin.Plugin) {
			// DI
			encodeName := base64.StdEncoding.EncodeToString([]byte(p.GetMeta().Name))
			p.SetStorage(ania.storage.Clone(encodeName))
			p.SetRestyClient(ania.restyClient)
			p.SetLogger(Logger().WithGroup(p.GetMeta().Name))
			p.SetConfig(plugin.SystemConfig{
				AdminId: message.QID(ania.cfg.GetInt64("bot.admin_id")),
			})

			// start
			startCtx, cancel := context.WithTimeout(ania.ctx, StartEventTimeout)
			p.Start(startCtx, ania.cfg)
			cancel()
		})
	}

	// 初始化cron
	c := cron.New()
	Logger().Info("开始初始化cron...")
	for _, p := range ania.plugins {
		safeExecute("初始化cron", p, func(p plugin.Plugin) {
			startCtx, cancel := context.WithTimeout(ania.ctx, StartCronEventTimeout)
			p.StartCron(startCtx, ania, c)
			cancel()
		})
	}
	Logger().Info("初始化cron完成")
	c.Start()
	defer c.Stop()

	fmt.Println(LogoASCII)
	Logger().Info("Bot启动完成...")

	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	go func() {
		<-timer.C
		Logger().Info("Awake...")
		for _, p := range ania.plugins {
			safeExecute("Awake", p, func(p plugin.Plugin) {
				awakeCtx, cancel := context.WithTimeout(ania.ctx, AwakeEventTimeout)
				p.Awake(awakeCtx, ania)
				cancel()
			})
		}
	}()

	ania.adapter.Serve(ania.cfg)
}

func (ania *AniaBot) GetPluginList() []plugininfo.PluginInfo {
	pluginList := make([]plugininfo.PluginInfo, 0, len(ania.plugins))
	for _, p := range ania.plugins {
		pluginList = append(pluginList, plugininfo.PluginInfo{
			Name:      p.GetMeta().Name,
			HelpWords: p.GetMeta().HelpWords,
			AdminOnly: p.GetMeta().AdminOnly,
			ShowFor:   p.GetMeta().ShowFor,
			Author:    p.GetMeta().Author,
			Version:   p.GetMeta().Version,
		})
	}
	return pluginList
}

func (ania *AniaBot) onGroupEvent(msg message.Message) {
	defer func() {
		if err := recover(); err != nil {
			Logger().Error("群聊消息事件触发错误: ", err)
		}
	}()
	if msg.Sender.UserId == msg.SelfId {
		return
	}

	cmd := utils.ParseCommand(msg)

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
			Logger().Error("私聊消息事件触发错误: ", err)
		}
	}()
	if msg.Sender.UserId == msg.SelfId {
		return
	}

	cmd := utils.ParseCommand(msg)

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
		if meta.ShowFor == 1 {
			panic("插件必须指定ShowFor")
		}
		if _, ok := ania.pluginSet[meta.Name]; ok {
			panic("插件名称相同，请检查插件是否重复加载")
		}
		ania.pluginSet[meta.Name] = struct{}{}
		ania.plugins = append(ania.plugins, p)
		Logger().Info("已添加插件: ", "name", p.GetMeta().Name)
	}
}

func (ania *AniaBot) Stop() {
	ania.cancel()
}

func (ania *AniaBot) SendGroupMsg(groupId message.QID, chain msgchain.GroupChain) (msgId message.QID, success bool) {
	return ania.adapter.SendGroupMsg(groupId, chain)
}

func (ania *AniaBot) SendFriendMsg(userID message.QID, chain msgchain.FriendChain) (msgId message.QID, success bool) {
	return ania.adapter.SendFriendMsg(userID, chain)
}

func (ania *AniaBot) SendGroupAIVoiceMsg(groupId message.QID, character, msg string) (msgId message.QID, success bool) {
	return ania.adapter.SendGroupAIVoiceMsg(groupId, character, msg)
}

func (ania *AniaBot) SendPokeMsg(userId message.QID, groupId *message.QID) (success bool) {
	return ania.adapter.SendPokeMsg(userId, groupId)
}

func (ania *AniaBot) GetMsgDetail(msgId message.QID) (*message.Message, bool) {
	return ania.adapter.GetMsgDetail(msgId)
}

func (ania *AniaBot) SendGroupForwardMsg(groupId message.QID, chain msgchain.GroupForwardChain) (msgId message.QID, success bool) {
	return ania.adapter.SendGroupForwardMsg(groupId, chain)
}

func (ania *AniaBot) SendFriendForwardMsg(userId message.QID, chain msgchain.FriendForwardChain) (msgId message.QID, success bool) {
	return ania.adapter.SendFriendForwardMsg(userId, chain)
}

func (ania *AniaBot) GetForwardMsg(msgId message.QID) (msgs *[]message.Message, success bool) {
	return ania.adapter.GetForwardMsg(msgId)
}

func (ania *AniaBot) GetGroupUserInfo(groupId, userId message.QID) (*message.GroupUserInfo, bool) {
	return ania.adapter.GetGroupUserInfo(groupId, userId)
}

func (ania *AniaBot) GetNCrkey() ([]message.NCrkey, bool) {
	return ania.adapter.GetNCrkey()
}

func (ania *AniaBot) GetFriendList() (*[]message.Friend, bool) {
	return ania.adapter.GetFriendList()
}

func (ania *AniaBot) GetGroupDetail(groupId message.QID) (*message.GroupInfo, bool) {
	return ania.adapter.GetGroupDetail(groupId)
}

func (ania *AniaBot) SetMsgEmojiLike(msgId message.QID, emojiId int, like bool) (success bool) {
	return ania.adapter.SetMsgEmojiLike(msgId, emojiId, like)
}

func (ania *AniaBot) SendGroupSign(groupId message.QID) (success bool) {
	return ania.adapter.SendGroupSign(groupId)
}

func (ania *AniaBot) GetGroupMsgHistory(groupId message.QID, count int) (*[]message.Message, bool) {
	return ania.adapter.GetGroupMsgHistory(groupId, count)
}

func (ania *AniaBot) GetFriendMsgHistory(userId message.QID, count int) (*[]message.Message, bool) {
	return ania.adapter.GetFriendMsgHistory(userId, count)
}

func (ania *AniaBot) GetAIChatacter() (*[]message.AIChatacter, bool) {
	return ania.adapter.GetAIChatacter()
}

func (ania *AniaBot) GetGroupList() (*[]message.GroupInfo, bool) {
	return ania.adapter.GetGroupList()
}
