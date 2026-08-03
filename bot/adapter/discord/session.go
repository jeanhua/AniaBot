package discord

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/gorilla/websocket"
	"golang.org/x/net/proxy"
)

// newSession 构造 discordgo 会话：Token 鉴权、intents 订阅、代理接线（REST 与
// WebSocket 网关双通道）、网关事件 handler 注册。
func (a *discordAdapter) newSession() (*discordgo.Session, error) {
	tok := a.cfg.token
	if !strings.HasPrefix(tok, "Bot ") {
		tok = "Bot " + tok
	}
	s, err := discordgo.New(tok)
	if err != nil {
		return nil, err
	}

	// MessageContent 为特权意图：Developer Portal 未开启时 identify 被拒
	// （close 4014 disallowed intent），错误呈现在连接状态与日志中
	s.Identify.Intents = discordgo.IntentsGuilds |
		discordgo.IntentsGuildMessages |
		discordgo.IntentsDirectMessages |
		discordgo.IntentsMessageContent |
		discordgo.IntentsGuildMessageReactions |
		discordgo.IntentsDirectMessageReactions
	if a.cfg.memberEvents {
		// Server Members 同为特权意图，默认关闭（仅需成员进出事件时开启）
		s.Identify.Intents |= discordgo.IntentsGuildMembers
	}

	if err := a.setupProxy(s); err != nil {
		return nil, err
	}

	s.AddHandler(a.onReady)
	s.AddHandler(a.onResumed)
	s.AddHandler(a.onDisconnect)
	s.AddHandler(a.onMessageCreate)
	s.AddHandler(a.onMessageDelete)
	s.AddHandler(a.onReactionAdd)
	s.AddHandler(a.onGuildCreate)
	s.AddHandler(a.onGuildDelete)
	s.AddHandler(a.onGuildMemberAdd)
	s.AddHandler(a.onGuildMemberRemove)
	return s, nil
}

// setupProxy 代理配置同时作用于 REST 客户端（含附件下载）与 WebSocket 网关拨号
// （同 Telegram 适配器语义：http(s):// 走 HTTP CONNECT，socks5(h):// 走 x/net/proxy）。
func (a *discordAdapter) setupProxy(s *discordgo.Session) error {
	if a.cfg.proxy == "" {
		return nil
	}
	u, err := url.Parse(a.cfg.proxy)
	if err != nil {
		return err
	}
	if strings.HasPrefix(a.cfg.proxy, "socks5://") || strings.HasPrefix(a.cfg.proxy, "socks5h://") {
		d, err := proxy.FromURL(u, proxy.Direct)
		if err != nil {
			return err
		}
		cd, ok := d.(proxy.ContextDialer)
		if !ok {
			return nil
		}
		s.Client = &http.Client{Transport: &http.Transport{DialContext: cd.DialContext}}
		s.Dialer = &websocket.Dialer{NetDialContext: cd.DialContext, HandshakeTimeout: 20 * time.Second}
		return nil
	}
	s.Client = &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(u)}}
	s.Dialer = &websocket.Dialer{Proxy: http.ProxyURL(u), HandshakeTimeout: 20 * time.Second}
	return nil
}

// ---------- 网关事件分发 ----------

// onReady READY 事件：填充自身信息（SelfID/自提及识别依赖），置连接状态。
func (a *discordAdapter) onReady(s *discordgo.Session, r *discordgo.Ready) {
	if r.User != nil {
		a.mu.Lock()
		a.selfID = r.User.ID
		a.selfUsername = r.User.Username
		a.mu.Unlock()
	}
	a.setStatus("connected", "")
	a.logger.Info("Discord 网关就绪", "bot", a.selfInfo(), "guilds", len(r.Guilds))
}

func (a *discordAdapter) onResumed(s *discordgo.Session, r *discordgo.Resumed) {
	a.setStatus("connected", "")
	a.logger.Info("Discord 网关会话已恢复")
}

func (a *discordAdapter) onDisconnect(s *discordgo.Session, d *discordgo.Disconnect) {
	a.setStatus("reconnecting", "网关连接断开，等待 discordgo 自动重连")
	a.logger.Warn("Discord 网关连接断开")
}

// onMessageCreate 消息事件：异步翻译分发（附件下载等 I/O 不经由网关读循环，
// discordgo handler 本身已在独立 goroutine，此处再包裹一层防止长耗时下载
// 堆积 handler 队列——与 Telegram 的 go handleUpdate 同构）。
func (a *discordAdapter) onMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	go a.handleMessage(m.Message)
}

// handleMessage 分发一条消息：过滤机器人/webhook 消息，翻译为通用消息后触发。
func (a *discordAdapter) handleMessage(m *discordgo.Message) {
	if m == nil || m.Author == nil {
		return
	}
	// 过滤机器人消息（含自身），防止 bot 循环（同飞书/Telegram 过滤 bot 发送者）
	if m.Author.Bot {
		return
	}
	// 过滤 webhook 消息（无法归属真实用户，也无 bot 标记兜底）
	if m.WebhookID != "" {
		a.logger.Debug("忽略 Discord webhook 消息", "channelId", m.ChannelID, "messageId", m.ID)
		return
	}
	msg := a.translateMessage(m)
	if msg == nil {
		a.logger.Debug("Discord 消息翻译为空，丢弃", "channelId", m.ChannelID, "messageId", m.ID)
		return
	}
	a.msgCache.Push(m.ChannelID, *msg)
	trig := a.triggerOf()
	switch msg.MessageType {
	case "private":
		if trig.OnFriendMsg != nil {
			trig.OnFriendMsg(*msg)
		}
	default:
		if trig.OnGroupMsg != nil {
			trig.OnGroupMsg(*msg)
		}
	}
}

// onGuildCreate 机器人加入服务器 → 平台事件（Discord 无"群"会话概念，
// 不进公共通知，同 telegram.bot_added 语义）。
// 注意：READY 后的 GuildCreate 是存量服务器推送，仅运行期间新增才算 bot_added
// —— 用 READY 标记区分（READY 前会话尚未就绪，READY 时的存量推送与运行期加入
// 无法从事件本身区分，这里以"连接建立后收到的非Unavailable推送"近似，与
// telegram my_chat_member 语义对齐，误报窗口仅在 READY 瞬间，可接受）。
func (a *discordAdapter) onGuildCreate(s *discordgo.Session, g *discordgo.GuildCreate) {
	if g.Guild == nil || g.Unavailable {
		return // 服务器宕机不可用推送，非加入事件
	}
	a.mu.Lock()
	ready := a.selfID != ""
	a.mu.Unlock()
	if !ready {
		return // READY 前的存量推送
	}
	a.emitPlatformEvent("discord.bot_added", g.Guild)
}

// onGuildDelete 机器人被移出服务器 → 平台事件。
// GuildDelete.Unavailable=true 是服务器宕机而非被移出，必须排除。
func (a *discordAdapter) onGuildDelete(s *discordgo.Session, g *discordgo.GuildDelete) {
	if g.Guild == nil || g.Unavailable {
		return
	}
	a.emitPlatformEvent("discord.bot_removed", g.Guild)
}

// onGuildMemberAdd/onGuildMemberRemove 成员进出 → 平台事件。
// 不映射公共 group_increase/decrease 通知：事件携带的是服务器 ID 而非频道 ID，
// 而 GroupIncreaseNotice.GroupId 必须保持频道可寻址（插件会向它回消息），
// 映射会污染 GroupId 不变量并击穿 core 的 route()。
func (a *discordAdapter) onGuildMemberAdd(s *discordgo.Session, m *discordgo.GuildMemberAdd) {
	if m.Member != nil {
		a.emitPlatformEvent("discord.guild_member_add", m.Member)
	}
}

func (a *discordAdapter) onGuildMemberRemove(s *discordgo.Session, m *discordgo.GuildMemberRemove) {
	if m.Member != nil {
		a.emitPlatformEvent("discord.guild_member_remove", m.Member)
	}
}
