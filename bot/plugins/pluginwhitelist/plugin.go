package pluginwhitelist

import (
	"context"
	"fmt"

	"github.com/jeanhua/AniaBot/bot/plugins/plugininterceptor"
	"github.com/jeanhua/AniaBot/common/bot"
	"github.com/jeanhua/AniaBot/common/model/command"
	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/jeanhua/AniaBot/common/msgchain"
	"github.com/jeanhua/AniaBot/common/plugin"
	"github.com/jeanhua/AniaBot/common/plugininfo"
	"github.com/spf13/viper"
)

// WhitelistPlugin 白名单管理插件：用 /wl 命令管理「请求拦截插件」的群聊与
// 私聊名单，改动写回配置并即时刷新，无需 /reboot。
//
// 与 plugininterceptor 的分工：名单的存储与拦截判定仍属 interceptor
// （两者共用同一个 ListStore），本插件负责命令入口、持久化与热生效，
// 并在 block_all 开启时把非白名单会话的消息拦在所有功能插件之前。
type WhitelistPlugin struct {
	plugin.Meta
	cfg   whitelistConfig
	store *plugininterceptor.ListStore
}

func NewPlugin() *WhitelistPlugin {
	p := &WhitelistPlugin{store: plugininterceptor.Store()}
	p.Name = "白名单管理插件"
	p.HelpWords = "管理员用 /wl 管理群聊与私聊白名单：/wl help 查看用法，改动立即生效"
	p.AdminOnly = true
	p.Author = "disillusion"
	p.Version = "1.0.0"
	// 排在日志之后、其余全部插件之前：block_all 开启时非白名单会话的消息
	// 到不了任何功能插件。比 pluginsys(-1100) 晚，保证管理员的 /help、
	// /reboot 等系统命令不受名单影响（否则配错名单会把自己关在门外）。
	p.Order = plugin.LevelLog + 1
	p.ShowFor = plugininfo.ShowForNone
	return p
}

func (p *WhitelistPlugin) Start(ctx context.Context, cfg *viper.Viper) error {
	if !p.cfg.Enable {
		p.Logger.Info("白名单管理插件已加载（未启用）")
		return nil
	}
	if p.ConfigEditor == nil {
		p.Logger.Warn("配置中心未注入，/wl 的增删将无法持久化（重启后丢失）")
	}
	return nil
}

// Awake 在全部插件 Start 完成后汇报名单状态。
// 不能在 Start 里汇报：本插件的 Order 比 interceptor 更靠前，Start 按 Order
// 顺序执行，那时 interceptor 还没把名单读进共享 store，日志会全是 0。
func (p *WhitelistPlugin) Awake(ctx context.Context, b bot.Bot) error {
	if !p.cfg.Enable {
		return nil
	}
	groups, friends, groupUsers := p.store.Counts()
	p.Logger.Info("白名单管理插件已就绪",
		"enable", p.store.Enabled(), "mode", p.store.Mode(),
		"block_all", p.cfg.BlockAll,
		"groups", groups, "friends", friends, "groupUsers", groupUsers)
	return nil
}

func (p *WhitelistPlugin) OnGroupMsg(ctx context.Context, b bot.Bot, cmd command.Command, msg message.Message) (bool, error) {
	if !p.cfg.Enable {
		return true, nil
	}
	// /wl 命令：仅管理员、且需 @ 机器人（与框架内其他群聊命令一致）
	if cmd.Name == "wl" && cmd.Mention {
		if msg.Sender.UserId != p.SystemConfig.AdminId {
			p.reply(b, msg.GroupId, true, msg.Sender.UserId, "只有管理员能管理白名单")
			return false, nil
		}
		p.reply(b, msg.GroupId, true, msg.Sender.UserId, p.handleCommand(cmd, msg.GroupId, true))
		return false, nil
	}
	return p.gate(b, msg.GroupId, true, msg)
}

func (p *WhitelistPlugin) OnFriendMsg(ctx context.Context, b bot.Bot, cmd command.Command, msg message.Message) (bool, error) {
	if !p.cfg.Enable {
		return true, nil
	}
	if cmd.Name == "wl" {
		if msg.Sender.UserId != p.SystemConfig.AdminId {
			p.reply(b, msg.Sender.UserId, false, msg.Sender.UserId, "只有管理员能管理白名单")
			return false, nil
		}
		p.reply(b, msg.Sender.UserId, false, msg.Sender.UserId,
			p.handleCommand(cmd, msg.Sender.UserId, false))
		return false, nil
	}
	return p.gate(b, msg.Sender.UserId, false, msg)
}

// gate 在 block_all 开启时把非白名单会话的消息拦在所有功能插件之前。
// 管理员本人恒放行——否则一旦名单配错，管理员连 /wl 都发不进来（虽然
// pluginsys 排在更前面，但 /wl 由本插件处理，必须自己留门）。
func (p *WhitelistPlugin) gate(b bot.Bot, id message.QID, isGroup bool, msg message.Message) (bool, error) {
	if !p.cfg.BlockAll || !p.store.Enabled() {
		return true, nil
	}
	if msg.Sender.UserId == p.SystemConfig.AdminId {
		return true, nil
	}

	allowed := true
	switch {
	case isGroup:
		allowed = p.store.AllowGroup(id) && !p.store.BlockedInGroup(id, msg.Sender.UserId)
		// 黑名单模式下群内发送者也要过用户名单（与 interceptor 判定一致）
		if allowed && !p.store.IsWhitelist() {
			allowed = p.store.AllowFriend(msg.Sender.UserId)
		}
	default:
		allowed = p.store.AllowFriend(msg.Sender.UserId)
	}
	if allowed {
		return true, nil
	}

	p.Logger.Info("拦截未授权会话的全部插件", "id", id, "is_group", isGroup,
		"user", msg.Sender.UserId, "mode", p.store.Mode())
	if p.cfg.NotifyDenied {
		p.reply(b, id, isGroup, msg.Sender.UserId, "这个会话未在白名单内，机器人不会响应")
	}
	return false, nil
}

// reply 回复一段文本（群聊 @ 发起者）
func (p *WhitelistPlugin) reply(b bot.Bot, id message.QID, isGroup bool, at message.QID, text string) {
	if text == "" {
		return
	}
	if isGroup {
		c := msgchain.Builder().Group()
		c.Mention(at)
		c.Text(" " + text)
		if _, ok := b.SendGroupMsg(id, c.Build()); !ok {
			p.Logger.Error("发送白名单管理回复失败", "group", id)
		}
		return
	}
	c := msgchain.Builder().Friend()
	c.Text(text)
	if _, ok := b.SendFriendMsg(id, c.Build()); !ok {
		p.Logger.Error("发送白名单管理回复失败", "user", id)
	}
}

// statusText 当前名单状态摘要
func (p *WhitelistPlugin) statusText() string {
	groups, friends, groupUsers := p.store.Counts()
	mode := "黑名单（名单内被拦）"
	if p.store.IsWhitelist() {
		mode = "白名单（仅名单内放行）"
	}
	state := "已启用"
	if !p.store.Enabled() {
		state = "未启用（当前放行全部会话）"
	}
	scope := "仅拦 AI 对话"
	if p.cfg.BlockAll {
		scope = "拦住全部功能插件"
	}
	return fmt.Sprintf("名单状态：%s\n模式：%s\n拦截范围：%s\n群名单 %d 条，用户名单 %d 条，群内屏蔽规则 %d 条",
		state, mode, scope, groups, friends, groupUsers)
}
