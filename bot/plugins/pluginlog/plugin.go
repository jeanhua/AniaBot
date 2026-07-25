package pluginlog

import (
	"context"
	"fmt"

	"github.com/jeanhua/AniaBot/bot/component/msglog"
	"github.com/jeanhua/AniaBot/bot/utils"
	"github.com/jeanhua/AniaBot/common/bot"
	"github.com/jeanhua/AniaBot/common/model/command"
	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/jeanhua/AniaBot/common/plugin"
	"github.com/jeanhua/AniaBot/common/plugininfo"
	"github.com/spf13/viper"
)

type LogPlugin struct {
	plugin.Meta
	recorder *msglog.Recorder
}

func NewPlugin() *LogPlugin {
	p := &LogPlugin{
		recorder: msglog.New(0),
	}
	p.Name = "日志打印插件"
	p.HelpWords = "用于在控制台打印日志信息"
	p.Author = "jeanhua"
	p.Version = "1.0.0"
	p.AdminOnly = true
	p.Order = plugin.LevelLog
	p.ShowFor = plugininfo.ShowForNone
	return p
}

func (p *LogPlugin) Start(ctx context.Context, cfg *viper.Viper) error {
	lastStartTime, ok := p.Storage.GetString(ctx, "last_start_time")
	if !ok {
		lastStartTime = "未保存"
	}
	p.Storage.SetString(ctx, "last_start_time", utils.GetFormattedTime())
	p.Logger.Info("日志打印插件初始化完成", "lastStartTime", lastStartTime)
	return nil
}

// MsgLogRecent 供 Web 控制面板读取最近的消息日志（实现 adminpanel.MsgLogSource）。
func (p *LogPlugin) MsgLogRecent(limit int) []msglog.Entry {
	return p.recorder.Recent(limit)
}

// senderName 群名片优先，其次昵称。
func senderName(msg message.Message) string {
	if msg.Sender.Card != "" {
		return msg.Sender.Card
	}
	return msg.Sender.Nickname
}

func (p *LogPlugin) OnGroupMsg(ctx context.Context, bot bot.Bot, cmd command.Command, msg message.Message) (bool, error) {
	text := msg.FriendlyText(false,
		message.WithGetMsgFunc(bot.GetMsgDetail),
		message.WithGetForwardMsgFunc(bot.GetForwardMsg),
		message.WithNoSenderPrefix(),
	)
	p.Logger.Info("[收<-群]", "groupId", msg.GroupId, "userId", msg.Sender.UserId, "message", text)
	p.recorder.Add(msglog.Entry{
		Type:     msglog.TypeGroup,
		GroupId:  msg.GroupId.String(),
		UserId:   msg.Sender.UserId.String(),
		Nickname: senderName(msg),
		Text:     text,
	})
	return true, nil
}

func (p *LogPlugin) OnFriendMsg(ctx context.Context, bot bot.Bot, cmd command.Command, msg message.Message) (bool, error) {
	text := msg.FriendlyText(false,
		message.WithGetMsgFunc(bot.GetMsgDetail),
		message.WithGetForwardMsgFunc(bot.GetForwardMsg),
		message.WithNoSenderPrefix(),
	)
	p.Logger.Info("[收<-好友]", "userId", msg.Sender.UserId, "message", text)
	p.recorder.Add(msglog.Entry{
		Type:     msglog.TypeFriend,
		UserId:   msg.Sender.UserId.String(),
		Nickname: msg.Sender.Nickname,
		Text:     text,
	})
	return true, nil
}

// notice 记录一条通知事件日志。
func (p *LogPlugin) notice(groupId, userId message.QID, title, text string) {
	p.recorder.Add(msglog.Entry{
		Type:    msglog.TypeNotice,
		GroupId: groupId.String(),
		UserId:  userId.String(),
		Title:   title,
		Text:    text,
	})
}

func (p *LogPlugin) OnGroupUpload(ctx context.Context, bot bot.Bot, n message.GroupUploadNotice) error {
	p.notice(n.GroupId, n.UserId, "群文件上传",
		fmt.Sprintf("%d 上传了文件 %s（%d 字节）", n.UserId.Uint64(), n.File.Name, n.File.Size))
	return nil
}

func (p *LogPlugin) OnGroupAdmin(ctx context.Context, bot bot.Bot, n message.GroupAdminNotice) error {
	action := "被设置为管理员"
	if n.SubType == "unset" {
		action = "被取消管理员"
	}
	p.notice(n.GroupId, n.UserId, "群管理员变动",
		fmt.Sprintf("%d %s", n.UserId.Uint64(), action))
	return nil
}

func (p *LogPlugin) OnGroupDecrease(ctx context.Context, bot bot.Bot, n message.GroupDecreaseNotice) error {
	action := map[string]string{
		"leave":   "退出了群聊",
		"kick":    "被踢出群聊",
		"kick_me": "Bot 被踢出群聊",
	}[n.SubType]
	if action == "" {
		action = "离开了群聊"
	}
	p.notice(n.GroupId, n.UserId, "群成员减少",
		fmt.Sprintf("%d %s（操作者 %d）", n.UserId.Uint64(), action, n.OperatorId.Uint64()))
	return nil
}

func (p *LogPlugin) OnGroupIncrease(ctx context.Context, bot bot.Bot, n message.GroupIncreaseNotice) error {
	action := "加入了群聊"
	if n.SubType == "invite" {
		action = "被邀请加入群聊"
	}
	p.notice(n.GroupId, n.UserId, "群成员增加",
		fmt.Sprintf("%d %s（操作者 %d）", n.UserId.Uint64(), action, n.OperatorId.Uint64()))
	return nil
}

func (p *LogPlugin) OnGroupBan(ctx context.Context, bot bot.Bot, n message.GroupBanNotice) error {
	if n.SubType == "lift_ban" {
		p.notice(n.GroupId, n.UserId, "群禁言",
			fmt.Sprintf("%d 被解除禁言（操作者 %d）", n.UserId.Uint64(), n.OperatorId.Uint64()))
	} else {
		p.notice(n.GroupId, n.UserId, "群禁言",
			fmt.Sprintf("%d 被禁言 %d 秒（操作者 %d）", n.UserId.Uint64(), n.Duration, n.OperatorId.Uint64()))
	}
	return nil
}

func (p *LogPlugin) OnFriendAdd(ctx context.Context, bot bot.Bot, n message.FriendAddNotice) error {
	p.notice("", n.UserId, "新好友",
		fmt.Sprintf("%d 添加了 Bot 为好友", n.UserId.Uint64()))
	return nil
}

func (p *LogPlugin) OnGroupRecall(ctx context.Context, bot bot.Bot, n message.GroupRecallNotice) error {
	p.notice(n.GroupId, n.UserId, "群消息撤回",
		fmt.Sprintf("%d 的消息（%d）被 %d 撤回", n.UserId.Uint64(), n.MessageId, n.OperatorId.Uint64()))
	return nil
}

func (p *LogPlugin) OnFriendRecall(ctx context.Context, bot bot.Bot, n message.FriendRecallNotice) error {
	p.notice("", n.UserId, "好友消息撤回",
		fmt.Sprintf("%d 撤回了消息（%d）", n.UserId.Uint64(), n.MessageId))
	return nil
}

func (p *LogPlugin) OnPoke(ctx context.Context, bot bot.Bot, n message.PokeNotice) error {
	var groupId message.QID
	if n.GroupId != nil {
		groupId = *n.GroupId
	}
	p.notice(groupId, n.UserId, "戳一戳",
		fmt.Sprintf("%d 戳了戳 %d", n.UserId.Uint64(), n.TargetId.Uint64()))
	return nil
}

func (p *LogPlugin) OnLuckyKing(ctx context.Context, bot bot.Bot, n message.LuckyKingNotice) error {
	p.notice(n.GroupId, n.TargetId, "运气王",
		fmt.Sprintf("%d 发出的红包，%d 成为运气王", n.UserId.Uint64(), n.TargetId.Uint64()))
	return nil
}

func (p *LogPlugin) OnHonor(ctx context.Context, bot bot.Bot, n message.HonorNotice) error {
	honor := map[string]string{
		"talkative":     "龙王",
		"performer":     "群聊之火",
		"legend":        "群聊炽焰",
		"strong_newbie": "冒尖小春笋",
		"emotion":       "快乐源泉",
	}[n.HonorType]
	if honor == "" {
		honor = n.HonorType
	}
	p.notice(n.GroupId, n.UserId, "群荣誉变更",
		fmt.Sprintf("%d 获得群荣誉「%s」", n.UserId.Uint64(), honor))
	return nil
}

func (p *LogPlugin) OnGroupMsgEmojiLike(ctx context.Context, bot bot.Bot, n message.GroupMsgEmojiLikeNotice) error {
	p.notice(n.GroupId, n.OperatorId, "表情回应",
		fmt.Sprintf("%d 对消息（%d）添加了 %d 个表情回应", n.OperatorId.Uint64(), n.MessageId.Uint64(), len(n.Likes)))
	return nil
}

func (p *LogPlugin) OnEssence(ctx context.Context, bot bot.Bot, n message.EssenceNotice) error {
	action := "被设为精华"
	if n.SubType == "delete" {
		action = "被移出精华"
	}
	p.notice(n.GroupId, n.SenderId, "群精华消息",
		fmt.Sprintf("%d 的消息（%d）%s（操作者 %d）", n.SenderId.Uint64(), n.MessageId.Uint64(), action, n.OperatorId.Uint64()))
	return nil
}

func (p *LogPlugin) OnGroupCard(ctx context.Context, bot bot.Bot, n message.GroupCardNotice) error {
	p.notice(n.GroupId, n.UserId, "群名片变更",
		fmt.Sprintf("%d 的群名片由「%s」改为「%s」", n.UserId.Uint64(), n.CardOld, n.CardNew))
	return nil
}
