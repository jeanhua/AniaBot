package core

import (
	"context"
	_ "embed"
	"encoding/base64"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/jeanhua/AniaBot/bot/adminpanel"
	"github.com/jeanhua/AniaBot/bot/component/consollog"
	"github.com/jeanhua/AniaBot/bot/component/msglog"
	"github.com/jeanhua/AniaBot/bot/component/querylog"
	"github.com/jeanhua/AniaBot/bot/component/tasklog"
	"github.com/jeanhua/AniaBot/bot/core/configstore"
	"github.com/jeanhua/AniaBot/bot/utils"
	"github.com/jeanhua/AniaBot/common/adapter"
	"github.com/jeanhua/AniaBot/common/bot"
	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/jeanhua/AniaBot/common/msgchain"
	"github.com/jeanhua/AniaBot/common/plugin"
	"github.com/jeanhua/AniaBot/common/pluginconfig"
	"github.com/jeanhua/AniaBot/common/plugininfo"
	"github.com/jeanhua/AniaBot/common/storage"
	"github.com/robfig/cron/v3"
	"github.com/spf13/viper"
)

// adapterEntry 一个已启用的平台适配器实例。
// evBot 是该平台能力包装后的 bot 外观：事件分发时传给插件，
// 插件可通过类型断言探测平台专属能力（如 bot.QQ）。
type adapterEntry struct {
	def     adapter.Definition
	adapter adapter.Adapter
	evBot   bot.Bot
}

type AniaBot struct {
	ctx    context.Context
	cancel context.CancelFunc

	// adapters 已启用的平台适配器（可多开：QQ + 飞书……）
	adapters []*adapterEntry
	// adapterFactory 延迟创建单个适配器（旧版单适配器用法，注册表启用后保留兼容）
	adapterFactory func(cfg *viper.Viper) adapter.Adapter
	plugins        []plugin.Plugin
	cfg            *viper.Viper

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
		// 注入的自定义 logger 同样设为 slog 默认，保证依赖 slog.Default()
		// 的组件（tasklog/querylog/平台适配器等）与框架共用同一输出。
		slog.SetDefault(logger)
	}
}

// WithAdapterFactory 延迟创建单个适配器（兼容旧用法）：Run 时配置中心加载完成后
// 调用工厂，按配置选择具体适配器实现。多平台并存请改用适配器注册表
// （各平台包 init() 中 adapter.Register，按 bot.platform.<name>.enable 启用）。
func WithAdapterFactory(factory func(cfg *viper.Viper) adapter.Adapter) Option {
	return func(ania *AniaBot) {
		ania.adapterFactory = factory
	}
}

func NewAniaBot(a adapter.Adapter, option ...Option) *AniaBot {
	ctx, cancel := context.WithCancel(context.Background())
	ania := &AniaBot{
		ctx:       ctx,
		cancel:    cancel,
		pluginSet: map[string]struct{}{},
		plugins:   make([]plugin.Plugin, 0),
	}
	if a != nil {
		ania.addAdapter(adapter.Definition{Name: "legacy", Platform: "qq"}, a)
	}
	for _, op := range option {
		op(ania)
	}
	if ania.logger == nil {
		ania.logger = Logger()
	}
	return ania
}

// addAdapter 注册一个已创建好的适配器：计算平台能力包装的 bot 外观，并注入事件回调。
func (ania *AniaBot) addAdapter(def adapter.Definition, a adapter.Adapter) {
	e := &adapterEntry{def: def, adapter: a}
	e.evBot = adapter.WrapBot(ania, a)
	a.SetTrigger(ania.makeTrigger(e))
	ania.adapters = append(ania.adapters, e)
}

// makeTrigger 构造某个适配器的事件回调包装器：各回调携带来源适配器，
// 以便分发时按平台过滤插件并向插件传入平台能力包装的 bot。
func (ania *AniaBot) makeTrigger(e *adapterEntry) adapter.TriggerWrapper {
	return adapter.TriggerWrapper{
		OnGroupMsg:          func(msg message.Message) { ania.onGroupEvent(e, msg) },
		OnFriendMsg:         func(msg message.Message) { ania.onFriendEvent(e, msg) },
		OnGroupUpload:       func(n message.GroupUploadNotice) { ania.onGroupUploadEvent(e, n) },
		OnGroupAdmin:        func(n message.GroupAdminNotice) { ania.onGroupAdminEvent(e, n) },
		OnGroupDecrease:     func(n message.GroupDecreaseNotice) { ania.onGroupDecreaseEvent(e, n) },
		OnGroupIncrease:     func(n message.GroupIncreaseNotice) { ania.onGroupIncreaseEvent(e, n) },
		OnGroupBan:          func(n message.GroupBanNotice) { ania.onGroupBanEvent(e, n) },
		OnFriendAdd:         func(n message.FriendAddNotice) { ania.onFriendAddEvent(e, n) },
		OnGroupRecall:       func(n message.GroupRecallNotice) { ania.onGroupRecallEvent(e, n) },
		OnFriendRecall:      func(n message.FriendRecallNotice) { ania.onFriendRecallEvent(e, n) },
		OnPoke:              func(n message.PokeNotice) { ania.onPokeEvent(e, n) },
		OnLuckyKing:         func(n message.LuckyKingNotice) { ania.onLuckyKingEvent(e, n) },
		OnHonor:             func(n message.HonorNotice) { ania.onHonorEvent(e, n) },
		OnGroupMsgEmojiLike: func(n message.GroupMsgEmojiLikeNotice) { ania.onGroupMsgEmojiLikeEvent(e, n) },
		OnEssence:           func(n message.EssenceNotice) { ania.onEssenceEvent(e, n) },
		OnGroupCard:         func(n message.GroupCardNotice) { ania.onGroupCardEvent(e, n) },
		OnPlatformEvent:     func(ev message.PlatformEvent) { ania.onPlatformEvent(e, ev) },
	}
}

// route 按统一 ID 前缀路由到对应平台适配器；未命中任何前缀时回退到
// 无前缀的默认适配器（QQ 历史裸数字 ID 兼容）。
func (ania *AniaBot) route(id message.QID) adapter.Adapter {
	if len(ania.adapters) == 0 {
		return nil
	}
	s := string(id)
	for _, e := range ania.adapters {
		if e.def.IDPrefix != "" && strings.HasPrefix(s, e.def.IDPrefix) {
			return e.adapter
		}
	}
	for _, e := range ania.adapters {
		if e.def.IDPrefix == "" {
			return e.adapter
		}
	}
	return ania.adapters[0].adapter
}

// AdapterStatuses 汇总各已启用适配器的连接状态（供 Web 面板展示）。
func (ania *AniaBot) AdapterStatuses() []adminpanel.AdapterStatus {
	out := make([]adminpanel.AdapterStatus, 0, len(ania.adapters))
	for _, e := range ania.adapters {
		st := adminpanel.AdapterStatus{Name: e.def.Name, Platform: e.def.Platform}
		type statuser interface{ AdapterStatus() (string, string) }
		if s, ok := e.adapter.(statuser); ok {
			st.State, st.Detail = s.AdapterStatus()
		} else {
			type connChecker interface{ Connected() bool }
			if c, ok := e.adapter.(connChecker); ok {
				if c.Connected() {
					st.State = "connected"
				} else {
					st.State = "reconnecting"
				}
			} else {
				st.State = "unknown"
			}
		}
		out = append(out, st)
	}
	return out
}

func (ania *AniaBot) Run() {
	ania.startTime = time.Now()
	sort.SliceStable(ania.plugins, func(i, j int) bool {
		return ania.plugins[i].GetMeta().Order < ania.plugins[j].GetMeta().Order
	})

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

	// 收集配置字段元信息（框架 + 各平台适配器 + 各插件的 ConfigRegistrar /
	// ConfigSchemaProvider 声明），面板表单基于该注册表动态渲染。
	pluginconfig.Register(frameworkConfigFields...)
	for _, d := range adapter.Definitions() {
		pluginconfig.Register(d.ConfigFields...)
	}
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

	// 适配器：注册表 + 旧版单适配器工厂二选一。创建早于插件 Start，
	// 保证 setup_pending 期间插件调用发送接口时适配器已就绪。
	if len(ania.adapters) == 0 {
		if ania.adapterFactory != nil {
			a := ania.adapterFactory(ania.cfg)
			ania.addAdapter(adapter.Definition{Name: "legacy", Platform: "qq"}, a)
		}
		for _, d := range adapter.Definitions() {
			if !ania.cfg.GetBool("bot.platform." + d.Name + ".enable") {
				continue
			}
			a, err := d.New(ania.cfg)
			if err != nil {
				Logger().Error("创建适配器失败，已跳过该平台", "platform", d.Name, "error", err)
				continue
			}
			ania.addAdapter(d, a)
			Logger().Info("已启用平台适配器", "platform", d.Name)
		}
	}
	if len(ania.adapters) == 0 {
		Logger().Error("未启用任何平台适配器：请在 Web 面板的「平台适配器」配置中启用至少一个平台")
		os.Exit(1)
	}

	// 自动初始化：把配置填充进各插件声明的配置结构体（Start 之前完成）。
	for _, e := range schemas {
		if err := pluginconfig.Load(ania.cfg, e.schema); err != nil {
			Logger().Error("加载插件配置失败", "plugin", e.name, "error", err)
		}
	}

	// 缓存存储（易失，支持 TTL/列表；默认 memory，可配置 redis）
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
				AdminId: message.FromString(ania.cfg.GetString("bot.admin_id")),
			})

			// start
			startCtx, cancel := context.WithTimeout(ania.ctx, StartEventTimeout)
			err := p.Start(startCtx, ania.cfg)
			logError(err, p, "初始化")
			cancel()
		})
	}

	// 初始化cron
	c := cron.New()
	Logger().Info("开始初始化cron...")
	for _, p := range ania.plugins {
		safeExecute("初始化cron", p, func(p plugin.Plugin) {
			startCtx, cancel := context.WithTimeout(ania.ctx, StartCronEventTimeout)
			err := p.StartCron(startCtx, ania, c)
			logError(err, p, "初始化cron")
			cancel()
		})
	}
	Logger().Info("初始化cron完成")
	c.Start()
	defer c.Stop()

	fmt.Println(LogoASCII)
	Logger().Info("Bot启动完成...")

	// 首次启动（设置向导未完成）时适配器不会连接，跳过 Awake：
	// 插件的 Awake 多依赖连接，此时触发只会发送失败；
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
					err := p.Awake(awakeCtx, ania)
					logError(err, p, "Awake")
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
		Logger().Info("首次启动：暂不连接平台适配器，请在 Web 控制面板完成设置向导（完成后重启 Bot 生效）", "url", "http://"+listen)
		<-ania.ctx.Done()
		return
	}

	// 启动全部平台适配器（各自在 goroutine 中运行连接循环）
	ania.adapterStarted.Store(true)
	for _, e := range ania.adapters {
		go func(a adapter.Adapter) {
			defer func() {
				if err := recover(); err != nil {
					Logger().Error("适配器运行异常", "platform", a.Name(), "error", err)
				}
			}()
			a.Serve(ania.cfg)
		}(e.adapter)
	}
	<-ania.ctx.Done()
}

// startAdminPanel 启动 Web 控制面板（goroutine 内运行 HTTP 服务）。
func (ania *AniaBot) startAdminPanel() {
	// 查找提供定时任务执行日志的插件（如 AI 对话插件的 clock 功能）
	var taskLogFn func(f tasklog.Filter) []tasklog.Entry
	var clockSrc adminpanel.ClockTaskSource
	var msgLogFn func(limit int, beforeID uint64) []msglog.Entry
	var skillSrc adminpanel.SkillSource
	var memorySrc adminpanel.MemorySource
	var kbSrc adminpanel.KnowledgeBaseSource
	var teamSrc adminpanel.TeamSource
	var quotaSrc adminpanel.QuotaSource
	var queryLogFn func(f querylog.Filter) []querylog.Entry
	for _, p := range ania.plugins {
		if src, ok := p.(adminpanel.TaskLogSource); ok {
			taskLogFn = src.TaskLogQuery
		}
		if src, ok := p.(adminpanel.ClockTaskSource); ok {
			clockSrc = src
		}
		if src, ok := p.(adminpanel.MsgLogSource); ok {
			msgLogFn = src.MsgLogPage
		}
		if src, ok := p.(adminpanel.SkillSource); ok {
			skillSrc = src
		}
		if src, ok := p.(adminpanel.MemorySource); ok {
			memorySrc = src
		}
		if src, ok := p.(adminpanel.KnowledgeBaseSource); ok {
			kbSrc = src
		}
		if src, ok := p.(adminpanel.TeamSource); ok {
			teamSrc = src
		}
		if src, ok := p.(adminpanel.QuotaSource); ok {
			quotaSrc = src
		}
		if src, ok := p.(adminpanel.QueryLogSource); ok {
			queryLogFn = src.QueryLogRecent
		}
		if taskLogFn != nil && clockSrc != nil && msgLogFn != nil && skillSrc != nil && memorySrc != nil && kbSrc != nil && teamSrc != nil && quotaSrc != nil && queryLogFn != nil {
			break
		}
	}

	// 面板的 QQ 专属能力来源（群/好友列表）：默认适配器若实现了 QQ 能力则提供
	var qq bot.QQ
	for _, e := range ania.adapters {
		if qb, ok := e.evBot.(bot.QQ); ok {
			qq = qb
			break
		}
	}

	srv := adminpanel.NewServer(adminpanel.Options{
		Listen:     ania.cfg.GetString("bot.admin_panel.listen"),
		Config:     ania.configStore,
		Persistent: ania.persistent,
		Bot:        ania,
		QQ:         qq,
		Adapter: func() string {
			if ania.configStore != nil && ania.configStore.SetupPending() {
				return "setup_pending"
			}
			if !ania.adapterStarted.Load() {
				return "not_started"
			}
			statuses := ania.AdapterStatuses()
			if len(statuses) == 0 {
				return "unknown"
			}
			// 任一已连接即视为 connected
			for _, st := range statuses {
				if st.State == "connected" {
					return "connected"
				}
			}
			return "reconnecting"
		},
		AdapterDetail: func() string {
			statuses := ania.AdapterStatuses()
			if len(statuses) == 0 {
				return ""
			}
			parts := make([]string, 0, len(statuses))
			for _, st := range statuses {
				parts = append(parts, fmt.Sprintf("%s:%s", st.Platform, st.State))
			}
			return strings.Join(parts, " ")
		},
		AdapterStatuses: ania.AdapterStatuses,
		TaskLogs:        taskLogFn,
		Clocks:          clockSrc,
		MsgLogs:         msgLogFn,
		Skills:          skillSrc,
		Memories:        memorySrc,
		Knowledge:       kbSrc,
		Teams:           teamSrc,
		Quota:           quotaSrc,
		QueryLogs:       queryLogFn,
		ConsoleLogs:     consollog.Page,
		Logger:          Logger().WithGroup("AdminPanel"),
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

// supportsPlatform 插件是否声明支持指定平台。
func (ania *AniaBot) supportsPlatform(p plugin.Plugin, platform string) bool {
	return p.GetMeta().SupportsPlatform(platform)
}

func (ania *AniaBot) onGroupEvent(e *adapterEntry, msg message.Message) {
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
		if !ania.supportsPlatform(p, e.def.Platform) {
			continue
		}
		next, panicked := safeExecuteWithReturn("群聊消息事件", p, func(p plugin.Plugin) bool {
			msgCtx, cancel := context.WithTimeout(ania.ctx, MsgEventTimeout)
			next, err := p.OnGroupMsg(msgCtx, e.evBot, cmd, msg)
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

func (ania *AniaBot) onFriendEvent(e *adapterEntry, msg message.Message) {
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
		if !ania.supportsPlatform(p, e.def.Platform) {
			continue
		}
		next, panicked := safeExecuteWithReturn("私聊消息事件", p, func(p plugin.Plugin) bool {
			msgCtx, cancel := context.WithTimeout(ania.ctx, MsgEventTimeout)
			next, err := p.OnFriendMsg(msgCtx, e.evBot, cmd, msg)
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

// SendGroupMsg 发送群聊消息（按群 ID 前缀路由到对应平台适配器）。
func (ania *AniaBot) SendGroupMsg(groupId message.QID, chain msgchain.GroupChain) (msgId message.QID, success bool) {
	a := ania.route(groupId)
	if a == nil {
		return "", false
	}
	return a.SendGroupMsg(groupId, chain)
}

// SendFriendMsg 发送私聊消息（按用户 ID 前缀路由到对应平台适配器）。
func (ania *AniaBot) SendFriendMsg(userID message.QID, chain msgchain.FriendChain) (msgId message.QID, success bool) {
	a := ania.route(userID)
	if a == nil {
		return "", false
	}
	return a.SendFriendMsg(userID, chain)
}

// GetMsgDetail 获取消息详情（按消息 ID 前缀路由到对应平台适配器）。
func (ania *AniaBot) GetMsgDetail(msgId message.QID) (*message.Message, bool) {
	a := ania.route(msgId)
	if a == nil {
		return nil, false
	}
	return a.GetMsgDetail(msgId)
}

// GetGroupDetail 获取群聊详情（按群 ID 前缀路由到对应平台适配器）。
func (ania *AniaBot) GetGroupDetail(groupId message.QID) (*message.GroupInfo, bool) {
	a := ania.route(groupId)
	if a == nil {
		return nil, false
	}
	return a.GetGroupDetail(groupId)
}

// GetGroupMsgHistory 获取群聊消息历史（按群 ID 前缀路由到对应平台适配器）。
func (ania *AniaBot) GetGroupMsgHistory(groupId message.QID, count int, message_seq int) (*[]message.Message, bool) {
	a := ania.route(groupId)
	if a == nil {
		return nil, false
	}
	return a.GetGroupMsgHistory(groupId, count, message_seq)
}

// GetFriendMsgHistory 获取私聊消息历史（按用户 ID 前缀路由到对应平台适配器）。
func (ania *AniaBot) GetFriendMsgHistory(userId message.QID, count int, message_seq int) (*[]message.Message, bool) {
	a := ania.route(userId)
	if a == nil {
		return nil, false
	}
	return a.GetFriendMsgHistory(userId, count, message_seq)
}
