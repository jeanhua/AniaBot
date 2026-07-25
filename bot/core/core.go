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
	"github.com/jeanhua/AniaBot/bot/adminpanel"
	"github.com/jeanhua/AniaBot/bot/component/msglog"
	"github.com/jeanhua/AniaBot/bot/component/tasklog"
	"github.com/jeanhua/AniaBot/bot/core/configstore"
	"github.com/jeanhua/AniaBot/bot/utils"
	"github.com/jeanhua/AniaBot/common/adapter"
	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/jeanhua/AniaBot/common/msgchain"
	"github.com/jeanhua/AniaBot/common/plugin"
	"github.com/jeanhua/AniaBot/common/pluginconfig"
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

	configStore *configstore.Store

	storage     storage.Storage
	persistent  storage.PersistentStorage
	restyClient *resty.Client

	logger *slog.Logger

	// 插件名称集合
	pluginSet map[string]struct{}

	// goroutine数目
	goroutineNum atomic.Int32

	// 启动时间
	startTime time.Time

	// 适配器是否已开始连接（首次启动等待向导时不会启动）
	adapterStarted atomic.Bool
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

func WithPersistentStorage(storage storage.PersistentStorage) Option {
	return func(ania *AniaBot) {
		ania.persistent = storage
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
	ania.startTime = time.Now()
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

	// 持久化存储（重启不丢失；配置中心的载体，必须最先初始化。
	// 驱动与位置由环境变量引导：ANIABOT_STORE_DRIVER / ANIABOT_SQLITE_PATH / ANIABOT_MYSQL_DSN）
	if ania.persistent == nil {
		store, err := newPersistentStorage(context.Background(), Logger().WithGroup("Persistent"))
		if err != nil {
			Logger().Error("初始化持久化存储失败", "error", err)
			os.Exit(1)
		}
		ania.persistent = store
	}

	// 收集配置字段元信息（框架 + 各插件的 ConfigRegistrar / ConfigSchemaProvider
	// 声明），面板表单基于该注册表动态渲染，新增插件无需改动面板代码。
	// 实现 ConfigSchemaProvider 的插件：反射注册其结构体标签字段，并缓存
	// 结构体指针，待配置中心构建完成后统一填充（自动初始化）。
	pluginconfig.Register(frameworkConfigFields...)
	var schemas []struct {
		name   string
		schema any
	}
	for _, p := range ania.plugins {
		if r, ok := p.(plugin.ConfigRegistrar); ok {
			pluginconfig.Register(r.ConfigFields()...)
		}
		if sp, ok := p.(plugin.ConfigSchemaProvider); ok {
			if schema := sp.ConfigSchema(); schema != nil {
				if err := pluginconfig.RegisterStruct(schema); err != nil {
					Logger().Error("注册插件配置结构体失败", "plugin", p.GetMeta().Name, "error", err)
				} else {
					schemas = append(schemas, struct {
						name   string
						schema any
					}{p.GetMeta().Name, schema})
				}
			}
		}
	}

	// 配置中心：全部配置存于数据库（首启写入默认值并进入设置向导）。
	// ToViper 构建内存 viper，插件与适配器的读取方式保持不变。
	if ania.cfg == nil {
		cs := configstore.New(ania.persistent, Logger().WithGroup("Config"))
		if err := cs.Init(); err != nil {
			Logger().Error("初始化配置中心失败", "error", err)
			os.Exit(1)
		}
		ania.configStore = cs

		// 补齐缺失的默认值后再构建 viper，保证插件 Start 读到的配置已含默认值。
		cs.EnsureDefaults(pluginconfig.Defaults())
		ania.cfg = cs.ToViper()
	}

	// 自动初始化：把配置填充进各插件声明的配置结构体（Start 之前完成）。
	// 失败按插件隔离记日志，该插件退化为结构体零值 + Start 内的既有校验兜底。
	for _, e := range schemas {
		if err := pluginconfig.Load(ania.cfg, e.schema); err != nil {
			Logger().Error("加载插件配置失败", "plugin", e.name, "error", err)
		}
	}

	// 缓存存储（易失，支持 TTL/列表；默认 redis，可配置 memory）
	if ania.storage == nil {
		store, err := newCacheStorage(context.Background(), ania.cfg, Logger().WithGroup("Cache"))
		if err != nil {
			Logger().Error("初始化缓存存储失败", "error", err)
			os.Exit(1)
		}
		ania.storage = store
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
			p.SetPersistentStorage(ania.persistent.Clone(encodeName))
			p.SetRestyClient(ania.restyClient)
			p.SetLogger(Logger().WithGroup(p.GetMeta().Name))
			p.SetConfig(plugin.SystemConfig{
				AdminId: message.FromUint64(uint64(ania.cfg.GetInt64("bot.admin_id"))),
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

	// 首次启动（设置向导未完成）时适配器不会连接，跳过 Awake：
	// 插件的 Awake 多依赖 QQ 连接（启动通知、定时任务触发），此时触发只会发送失败；
	// 向导完成重启后会正常执行。
	setupPending := ania.configStore != nil && ania.configStore.SetupPending()
	if !setupPending {
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
	}

	// Web 控制面板（配置修改重启后生效）
	if ania.configStore != nil && ania.cfg.GetBool("bot.admin_panel.enable") {
		ania.startAdminPanel()
	}

	// 首次启动（设置向导未完成）时不连接适配器：保持面板可访问，
	// 等用户在向导中填写连接配置并重启后再连接，避免控制台刷重连日志。
	if setupPending {
		listen := ania.cfg.GetString("bot.admin_panel.listen")
		if listen == "" {
			listen = "127.0.0.1:7700"
		}
		Logger().Info("首次启动：暂不连接 NapCat，请在 Web 控制面板完成设置向导（完成后重启 Bot 生效）", "url", "http://"+listen)
		<-ania.ctx.Done()
		return
	}

	ania.adapterStarted.Store(true)
	ania.adapter.Serve(ania.cfg)
}

// startAdminPanel 启动 Web 控制面板（goroutine 内运行 HTTP 服务）。
func (ania *AniaBot) startAdminPanel() {
	// 查找提供定时任务执行日志的插件（如 AI 对话插件的 clock 功能）
	var taskLogFn func(limit int) []tasklog.Entry
	var clockSrc adminpanel.ClockTaskSource
	var msgLogFn func(limit int) []msglog.Entry
	for _, p := range ania.plugins {
		if src, ok := p.(adminpanel.TaskLogSource); ok {
			taskLogFn = src.TaskLogRecent
		}
		if src, ok := p.(adminpanel.ClockTaskSource); ok {
			clockSrc = src
		}
		if src, ok := p.(adminpanel.MsgLogSource); ok {
			msgLogFn = src.MsgLogRecent
		}
		if taskLogFn != nil && clockSrc != nil && msgLogFn != nil {
			break
		}
	}

	srv := adminpanel.NewServer(adminpanel.Options{
		Listen:     ania.cfg.GetString("bot.admin_panel.listen"),
		Config:     ania.configStore,
		Persistent: ania.persistent,
		Bot:        ania,
		Adapter: func() string {
			if ania.configStore != nil && ania.configStore.SetupPending() {
				return "setup_pending"
			}
			if !ania.adapterStarted.Load() {
				return "not_started"
			}
			type statuser interface{ AdapterStatus() (string, string) }
			if s, ok := ania.adapter.(statuser); ok {
				state, _ := s.AdapterStatus()
				return state
			}
			type connChecker interface{ Connected() bool }
			if c, ok := ania.adapter.(connChecker); ok {
				if c.Connected() {
					return "connected"
				}
				return "reconnecting"
			}
			return "unknown"
		},
		AdapterDetail: func() string {
			type statuser interface{ AdapterStatus() (string, string) }
			if s, ok := ania.adapter.(statuser); ok {
				_, detail := s.AdapterStatus()
				return detail
			}
			return ""
		},
		TaskLogs: taskLogFn,
		Clocks:   clockSrc,
		MsgLogs:  msgLogFn,
		Logger:   Logger().WithGroup("AdminPanel"),
	})
	go srv.Run()
}

// GoroutineNum 返回当前由框架追踪的 goroutine 数目。
func (ania *AniaBot) GoroutineNum() int32 {
	return ania.goroutineNum.Load()
}

// StartTime 返回 Bot 启动时间。
func (ania *AniaBot) StartTime() time.Time {
	return ania.startTime
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
		next, panicked := safeExecuteWithReturn("群聊消息事件", p, func(p plugin.Plugin) bool {
			msgCtx, cancel := context.WithTimeout(ania.ctx, MsgEventTimeout)
			next, err := p.OnGroupMsg(msgCtx, ania, cmd, msg)
			logError(err, p, "群聊消息事件")
			cancel()
			return next
		})
		if panicked {
			next = true // 插件 panic 不应阻断后续插件，继续传播（与通知事件一致）
		}
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
		next, panicked := safeExecuteWithReturn("私聊消息事件", p, func(p plugin.Plugin) bool {
			msgCtx, cancel := context.WithTimeout(ania.ctx, MsgEventTimeout)
			next, err := p.OnFriendMsg(msgCtx, ania, cmd, msg)
			logError(err, p, "私聊消息事件")
			cancel()
			return next
		})
		if panicked {
			next = true // 插件 panic 不应阻断后续插件，继续传播（与通知事件一致）
		}
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

func (ania *AniaBot) GetGroupMsgHistory(groupId message.QID, count int, message_seq int) (*[]message.Message, bool) {
	return ania.adapter.GetGroupMsgHistory(groupId, count, message_seq)
}

func (ania *AniaBot) GetFriendMsgHistory(userId message.QID, count int, message_seq int) (*[]message.Message, bool) {
	return ania.adapter.GetFriendMsgHistory(userId, count, message_seq)
}

func (ania *AniaBot) GetAIChatacter() (*[]message.AIChatacter, bool) {
	return ania.adapter.GetAIChatacter()
}

func (ania *AniaBot) GetPrivateFileURL(userId message.QID, fileId string) (string, bool) {
	return ania.adapter.GetPrivateFileURL(userId, fileId)
}

func (ania *AniaBot) GetGroupList() (*[]message.GroupInfo, bool) {
	return ania.adapter.GetGroupList()
}
