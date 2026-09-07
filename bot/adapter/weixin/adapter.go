package weixin

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jeanhua/AniaBot/common/adapter"
	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/spf13/viper"
)

// errAdapterClosed 适配器客户端尚未就绪。
var errAdapterClosed = errors.New("weixin: 适配器未就绪")

// sleepFor time.Sleep 的可读包装（主循环退避等待）。
func sleepFor(d time.Duration) { time.Sleep(d) }

// idPrefix 微信平台 ID 的框架统一前缀：iLink 用户/机器人 ID 形如 xxx@im.wechat /
// xxx@im.bot，框架内表示为 "wx:" 前缀字符串，core 按前缀路由到本适配器。
const idPrefix = "wx:"

// Platform 平台标识。
const Platform = "weixin"

// weixinConfig 适配器配置（bot.weixin.*）。
type weixinConfig struct {
	// configToken 配置中心手工置入的 token（优先于状态文件；一般留空走扫码）
	configToken string
	// apiBase 登录/轮询 API 地址；登录成功后以服务端下发的 baseurl 优先
	apiBase string
	cdnBase string
	botType string
	// stateDir 凭据与轮询游标的状态目录
	stateDir string
	// pollTimeout getupdates 长轮询客户端超时（服务端约挂起 35s）
	pollTimeout time.Duration
}

// weixinAdapter 微信（iLink bot）平台适配器：HTTP 长轮询收消息、HTTP 发消息，
// 媒体经微信 CDN 中转（AES-128-ECB 端侧加解密）。仅私聊（bot 与用户 1:1 会话）。
//
// 事件为 at-least-once 投递：长轮询游标（get_updates_buf）在响应后落盘，崩溃窗口
// 内已消费未落盘的消息会重推，core 按「平台 + MessageId」兜底去重。
type weixinAdapter struct {
	mu      sync.Mutex
	trigger adapter.TriggerWrapper
	cfg     weixinConfig

	client *client
	// self 机器人自身 ID（xxx@im.bot），登录成功后填充
	self string

	state     *stateStore
	ctxTokens *contextTokenStore

	msgCache *msgCache

	connState string
	lastErr   string

	// loginMu 保护面板扫码登录会话与凭据更新信号
	loginMu      sync.Mutex
	login        *loginSession // 最近一次面板发起的登录会话（可为 nil）
	credsChanged chan struct{} // 凭据落盘信号：关闭即通知进行中的登录会话中止

	logger *slog.Logger
}

// NewAdapter 创建微信适配器。状态目录在 Serve 读到配置后重建；
// 此处以默认路径初始化，保证 Serve 之前的调用（如面板探活）安全。
func NewAdapter(cfg *viper.Viper) *weixinAdapter {
	return &weixinAdapter{
		state:        newStateStore("./data/weixin"),
		ctxTokens:    newContextTokenStore("./data/weixin/context_tokens.json"),
		msgCache:     newMsgCache(msgCachePerUser, msgCacheMaxUsers),
		credsChanged: make(chan struct{}),
		logger:       slog.Default().With("adapter", "weixin"),
	}
}

func (a *weixinAdapter) Name() string     { return "weixin" }
func (a *weixinAdapter) Platform() string { return Platform }

// SelfID 实现 adapter.SelfIDProvider：返回机器人自身 ID（wx:xxx@im.bot），
// 未登录成功时为空。core 用它兜底填充 msg.SelfId，使自消息过滤生效。
func (a *weixinAdapter) SelfID() message.QID {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.self == "" {
		return ""
	}
	return message.QID(idPrefix + a.self)
}

// weixinSegments 微信出站支持的通用段类型；mention/reply/face 等在 sendChain
// 退化（mention 丢弃、face 无 text 键忽略），core 会对不支持段告警。
var weixinSegments = []string{
	message.SegmentText, message.SegmentImage,
	message.SegmentFile, message.SegmentRecord,
	message.SegmentVideo,
}

// SupportedSegments 实现 adapter.SegmentSupport。
func (a *weixinAdapter) SupportedSegments() []string { return weixinSegments }

func (a *weixinAdapter) SetTrigger(trigger adapter.TriggerWrapper) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.trigger = trigger
}

func (a *weixinAdapter) triggerOf() adapter.TriggerWrapper {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.trigger
}

func (a *weixinAdapter) setStatus(state, detail string) {
	a.mu.Lock()
	a.connState = state
	a.lastErr = detail
	a.mu.Unlock()
}

// AdapterStatus 返回连接状态（connecting/connected/reconnecting/未登录）与详情。
func (a *weixinAdapter) AdapterStatus() (string, string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.connState == "" {
		return "connecting", ""
	}
	return a.connState, a.lastErr
}

func (a *weixinAdapter) loadConfig(v *viper.Viper) weixinConfig {
	cfg := weixinConfig{
		configToken: strings.TrimSpace(v.GetString("bot.weixin.token")),
		apiBase:     strings.TrimSpace(v.GetString("bot.weixin.api_base")),
		cdnBase:     strings.TrimSpace(v.GetString("bot.weixin.cdn_base")),
		botType:     strings.TrimSpace(v.GetString("bot.weixin.bot_type")),
		stateDir:    strings.TrimSpace(v.GetString("bot.weixin.state_dir")),
	}
	if cfg.apiBase == "" {
		cfg.apiBase = DefaultAPIBase
	}
	if cfg.cdnBase == "" {
		cfg.cdnBase = DefaultCDNBase
	}
	if cfg.botType == "" {
		cfg.botType = DefaultBotType
	}
	if cfg.stateDir == "" {
		cfg.stateDir = "./data/weixin"
	}
	cfg.pollTimeout = 40 * time.Second // 服务端长轮询约 35s，客户端留余量
	return cfg
}

// resolveAccount 解析登录凭据：配置 token 优先，其次状态文件；均无时返回 nil。
func (a *weixinAdapter) resolveAccount() *accountState {
	if a.cfg.configToken != "" {
		// 配置手工置入 token：其余字段尽力从状态文件补齐
		st := &accountState{Token: a.cfg.configToken, BaseURL: a.cfg.apiBase, CDNBase: a.cfg.cdnBase}
		if saved, err := a.state.load(); err == nil && saved != nil {
			if saved.BaseURL != "" {
				st.BaseURL = saved.BaseURL
			}
			if saved.BotID != "" {
				st.BotID = saved.BotID
				a.setSelf(saved.BotID)
			}
			if saved.UserID != "" {
				st.UserID = saved.UserID
			}
		}
		return st
	}
	st, err := a.state.load()
	if err != nil {
		a.logger.Warn("读取微信账号状态失败，将重新扫码登录", "error", err)
		return nil
	}
	return st
}

func (a *weixinAdapter) setSelf(botID string) {
	a.mu.Lock()
	a.self = botID
	a.mu.Unlock()
}

// currentToken 当前客户端使用的 token（凭证失效比对用）。
func (a *weixinAdapter) currentToken() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.client == nil {
		return ""
	}
	return a.client.token
}

// ensureLogin 确保持有有效凭据：无凭据（或登录后仍无）时进入控制台扫码登录。
// 面板扫码若先行完成，本会话经 superseded 信号中止并直接采用面板保存的凭据。
// 返回非 nil 的账号状态；Bot 永不因未登录而退出（连接失败不可早退）。
func (a *weixinAdapter) ensureLogin() *accountState {
	for {
		st := a.resolveAccount()
		if st != nil && st.Token != "" {
			return st
		}
		a.setStatus("connecting", "等待微信扫码登录（见控制台二维码，或在 Web 面板「配置」页扫码）")
		a.logger.Warn("微信 bot 未登录：请在控制台用手机微信扫描二维码完成授权（8 分钟内有效），或在 Web 面板扫码")
		sess := a.newLoginSession(true)
		st, err := sess.run(context.Background())
		if errors.Is(err, errLoginSuperseded) {
			// 面板登录已保存新凭据：重新解析即可
			continue
		}
		if err != nil {
			a.setStatus("reconnecting", "扫码登录失败: "+err.Error())
			a.logger.Error("微信扫码登录失败，30 秒后重试", "error", err)
			sleepFor(30 * time.Second)
			continue
		}
		return st
	}
}

// ---------- 扫码登录会话 ----------

// saveLoginCredentials 登录成功后的凭据落盘（控制台/面板会话共用），
// 并广播凭据更新信号：另一入口进行中的登录会话感知后自行中止。
func (a *weixinAdapter) saveLoginCredentials(st *accountState) error {
	if err := a.state.save(st); err != nil {
		return err
	}
	a.setSelf(st.BotID)
	a.notifyCredsChanged()
	a.logger.Info("微信账号凭据已保存", "botId", st.BotID)
	return nil
}

// newLoginSession 创建一次扫码登录会话。console=true 时为控制台模式
// （stdin 读配对码、进度打印）。不得在持有 loginMu 时调用。
func (a *weixinAdapter) newLoginSession(console bool) *loginSession {
	return a.newLoginSessionWith(a.credsSignal(), console)
}

// newLoginSessionWith 同 newLoginSession，凭据信号由调用方传入
// （QRLoginStart 持有 loginMu 期间构造会话，需先在锁外取信号避免重入死锁）。
func (a *weixinAdapter) newLoginSessionWith(sig <-chan struct{}, console bool) *loginSession {
	return &loginSession{
		apiBase:     a.cfg.apiBase,
		cdnBase:     a.cfg.cdnBase,
		botType:     a.cfg.botType,
		localTokens: a.localTokens(),
		console:     console,
		superseded:  sig,
		onSave:      a.saveLoginCredentials,
	}
}

// notifyCredsChanged 广播凭据更新（关闭并替换信号 channel）。
func (a *weixinAdapter) notifyCredsChanged() {
	a.loginMu.Lock()
	defer a.loginMu.Unlock()
	if a.credsChanged != nil {
		close(a.credsChanged)
	}
	a.credsChanged = make(chan struct{})
}

// credsSignal 当前凭据更新信号。
func (a *weixinAdapter) credsSignal() <-chan struct{} {
	a.loginMu.Lock()
	defer a.loginMu.Unlock()
	if a.credsChanged == nil {
		a.credsChanged = make(chan struct{})
	}
	return a.credsChanged
}

// QRLoginStart 实现 adminpanel.QRLoginSource：发起（或复用进行中的）面板扫码
// 登录，返回二维码 PNG data URL。
func (a *weixinAdapter) QRLoginStart() (string, error) {
	sig := a.credsSignal() // 锁外取信号（credsSignal 自身加 loginMu，避免重入死锁）
	a.loginMu.Lock()
	if s := a.login; s != nil {
		switch state, _, qr := s.snapshot(); state {
		case LoginStatePending, LoginStateScaned, LoginStateNeedVerify:
			a.loginMu.Unlock()
			if qr != "" {
				return qr, nil
			}
			return "", fmt.Errorf("登录会话正在初始化，请稍候重试")
		}
	}
	sess := a.newLoginSessionWith(sig, false)
	a.login = sess
	a.loginMu.Unlock()
	// 同步取码：保证返回时二维码已就绪；轮询循环在后台运行至终态
	if err := sess.prepare(context.Background()); err != nil {
		return "", err
	}
	go func() {
		_, _ = sess.loop(context.Background())
	}()
	_, _, qr := sess.snapshot()
	return qr, nil
}

// QRLoginStatus 实现 adminpanel.QRLoginSource：返回登录流程状态与提示。
func (a *weixinAdapter) QRLoginStatus() (string, string, string) {
	a.loginMu.Lock()
	s := a.login
	a.loginMu.Unlock()
	if s == nil {
		return LoginStateIdle, "尚未发起扫码登录", ""
	}
	state, detail, qr := s.snapshot()
	if state == LoginStateConnected && a.currentConnState() == "connected" {
		detail += "；当前 bot 已在线，若登录了其他账号，重启 Bot 后生效"
	}
	return state, detail, qr
}

// QRLoginSubmitVerify 实现 adminpanel.QRLoginSource：提交手机配对数字。
func (a *weixinAdapter) QRLoginSubmitVerify(code string) error {
	a.loginMu.Lock()
	s := a.login
	a.loginMu.Unlock()
	if s == nil {
		return fmt.Errorf("没有进行中的扫码登录")
	}
	state, _, _ := s.snapshot()
	if state != LoginStateNeedVerify && state != LoginStateScaned && state != LoginStatePending {
		return fmt.Errorf("当前登录状态（%s）无需配对码", state)
	}
	s.submitVerify(code)
	return nil
}

// currentConnState 当前连接状态。
func (a *weixinAdapter) currentConnState() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.connState
}

// localTokens 已保存的本地 token 列表（登录请求携带，服务端用于识别已登录 bot）。
func (a *weixinAdapter) localTokens() []string {
	var tokens []string
	if st, err := a.state.load(); err == nil && st != nil && st.Token != "" {
		tokens = append(tokens, st.Token)
	}
	if a.cfg.configToken != "" {
		tokens = append(tokens, a.cfg.configToken)
	}
	return tokens
}

// Serve 启动微信适配器（阻塞）：确保登录 → notifyStart → 长轮询循环。
func (a *weixinAdapter) Serve(v *viper.Viper) {
	a.cfg = a.loadConfig(v)
	a.state = newStateStore(a.cfg.stateDir)
	a.ctxTokens = newContextTokenStore(a.cfg.stateDir + "/context_tokens.json")

	st := a.ensureLogin()
	apiBase := st.BaseURL
	if apiBase == "" {
		apiBase = a.cfg.apiBase
	}
	cdnBase := st.CDNBase
	if cdnBase == "" {
		cdnBase = a.cfg.cdnBase
	}
	a.mu.Lock()
	a.client = newClient(st.Token, apiBase, cdnBase)
	a.mu.Unlock()

	a.setStatus("connecting", "连接微信服务")
	ctx := context.Background()
	if err := a.client.notifyStart(ctx); err != nil {
		a.logger.Warn("notifyStart 失败（忽略）", "error", err)
	}
	a.logger.Info("微信长轮询启动", "apiBase", apiBase, "botId", st.BotID)
	a.setStatus("connected", "")
	a.pollLoop(ctx, apiBase, cdnBase)
}

// pollLoop 长轮询主循环：getupdates(buf) → 推进游标 → 异步分发。
// 凭证失效（errcode -14）时自动重新进入扫码登录（控制台展示二维码）。
func (a *weixinAdapter) pollLoop(ctx context.Context, apiBase, cdnBase string) {
	getUpdatesBuf := a.state.loadUpdatesBuf()
	if getUpdatesBuf != "" {
		a.logger.Info("恢复上次长轮询游标", "size", len(getUpdatesBuf))
	}
	backoff := 2 * time.Second
	for {
		if ctx.Err() != nil {
			return
		}
		resp, err := a.getUpdates(ctx, getUpdatesBuf)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			if StaleToken(err) {
				a.logger.Error("微信登录凭证已失效（errcode -14）：请重新扫码（控制台将展示二维码）")
				a.setStatus("reconnecting", "登录凭证失效，等待重新扫码")
				oldToken := a.currentToken()
				newSt := a.ensureLogin()
				// 凭据未变化（如配置手工置入的 token 已失效）时退避等待，
				// 避免对服务端形成紧循环
				if newSt.Token == oldToken {
					a.logger.Warn("凭据未更新，10 分钟后重试；请更换 bot.weixin.token 或清除状态目录后重新扫码")
					sleepFor(10 * time.Minute)
					continue
				}
				if newSt.BaseURL != "" {
					apiBase = newSt.BaseURL
				}
				a.mu.Lock()
				a.client = newClient(newSt.Token, apiBase, cdnBase)
				a.mu.Unlock()
				a.setStatus("connected", "")
				backoff = 2 * time.Second
				continue
			}
			a.setStatus("reconnecting", err.Error())
			a.logger.Warn("微信 getUpdates 失败", "error", err)
			sleepFor(backoff)
			if backoff < 30*time.Second {
				backoff *= 2
			}
			continue
		}
		backoff = 2 * time.Second
		a.setStatus("connected", "")
		if resp.LongpollingTimeoutMs > 0 {
			a.mu.Lock()
			if a.client != nil {
				a.cfg.pollTimeout = time.Duration(resp.LongpollingTimeoutMs)*time.Millisecond + 5*time.Second
			}
			a.mu.Unlock()
		}
		if resp.GetUpdatesBuf != "" {
			getUpdatesBuf = resp.GetUpdatesBuf
			a.state.saveUpdatesBuf(getUpdatesBuf)
		}
		a.ctxTokens.flush()
		for _, m := range resp.Msgs {
			if m == nil {
				continue
			}
			go a.handleMessage(m)
		}
	}
}

// getUpdates 长轮询拉取（post 内按 timeout 以 context 控制超时，客户端实例不变）。
func (a *weixinAdapter) getUpdates(ctx context.Context, buf string) (*GetUpdatesResp, error) {
	a.mu.Lock()
	c := a.client
	timeout := a.cfg.pollTimeout
	a.mu.Unlock()
	if c == nil {
		return nil, errAdapterClosed
	}
	return c.getUpdates(ctx, buf, timeout)
}

// handleMessage 分发一条入站消息（独立 goroutine）：缓存 context_token、
// 翻译为通用消息后触发插件链。微信 iLink bot 仅 bot↔用户 1:1 会话（私聊）。
func (a *weixinAdapter) handleMessage(m *WeixinMessage) {
	// 自消息过滤（message_type=2 为 bot 发出的消息回执）
	if m.MessageType == MsgTypeBot {
		return
	}
	from := strings.TrimSpace(m.FromUserID)
	if from == "" {
		return
	}
	a.ctxTokens.set(from, m.ContextToken)

	msg := a.translateMessage(m)
	if msg == nil {
		return
	}
	a.msgCache.Push(from, *msg)
	if trig := a.triggerOf(); trig.OnFriendMsg != nil {
		trig.OnFriendMsg(*msg)
	}
}

// ---------- ID 编解码 ----------

// frameUserID 框架内用户 ID："wx:<原始 ID>"（如 wx:abc@im.wechat）。
func frameUserID(raw string) message.QID {
	return message.QID(idPrefix + raw)
}

// frameMsgID 框架内消息 ID："wx:<用户原始 ID>:<message_id>"（出站为 ":c<client_id>"）。
func frameMsgID(rawUserID, mid string) message.QID {
	return message.QID(idPrefix + rawUserID + ":" + mid)
}

// parseFrameMsgID 解析 "wx:<raw>:<mid>"，返回 (用户原始 ID, mid)。
func parseFrameMsgID(s string) (string, string, bool) {
	if !strings.HasPrefix(s, idPrefix) {
		return "", "", false
	}
	rest := strings.TrimPrefix(s, idPrefix)
	sep := strings.LastIndex(rest, ":")
	if sep <= 0 || sep == len(rest)-1 {
		return "", "", false
	}
	return rest[:sep], rest[sep+1:], true
}

// msgSeqText 入站消息的稳定去重/缓存键：优先 message_id，缺省用 seq。
func msgSeqText(m *WeixinMessage) string {
	if m.MessageID != 0 {
		return strconv.FormatInt(m.MessageID, 10)
	}
	return strconv.FormatInt(m.Seq, 10)
}
