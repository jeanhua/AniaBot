package plugin

import (
	"context"

	"github.com/go-resty/resty/v2"
	"github.com/jeanhua/AniaBot/common/bot"
	"github.com/jeanhua/AniaBot/common/model/command"
	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/jeanhua/AniaBot/common/storage"
	"github.com/robfig/cron/v3"
	"github.com/spf13/viper"
)

type Plugin interface {
	GetMeta() *Meta
	DI
	BasicEvent
	StartupEvent
	NoticeEvent
}

type DI interface {
	SetStorage(s storage.Storage)
	SetRestyClient(*resty.Client)
}

type BasicEvent interface {
	// OnGroupMsg 收到群聊消息触发事件
	OnGroupMsg(context.Context, bot.Bot, command.Command, message.Message) (bool, error)
	// OnFriendMsg 收到私聊消息触发事件
	OnFriendMsg(context.Context, bot.Bot, command.Command, message.Message) (bool, error)
}

type CronManager interface {
	// AddFunc 添加定时任务
	AddFunc(spec string, cmd func()) (cron.EntryID, error)
}

type StartupEvent interface {
	// Start 插件初始化事件
	Start(ctx context.Context, cfg *viper.Viper) error
	// StartCron 初始化cron事件
	StartCron(ctx context.Context, bot bot.Bot, c CronManager) error
	// Awake Bot启动完成事件
	Awake(ctx context.Context, bot bot.Bot) error
}

type NoticeEvent interface {
	// OnGroupUpload 处理群文件上传
	OnGroupUpload(context.Context, bot.Bot, message.GroupUploadNotice) error
	// OnGroupAdmin 处理群管理员变动
	OnGroupAdmin(context.Context, bot.Bot, message.GroupAdminNotice) error
	// OnGroupDecrease 处理群成员减少
	OnGroupDecrease(context.Context, bot.Bot, message.GroupDecreaseNotice) error
	// OnGroupIncrease 处理群成员增加
	OnGroupIncrease(context.Context, bot.Bot, message.GroupIncreaseNotice) error
	// OnGroupBan 处理群禁言
	OnGroupBan(context.Context, bot.Bot, message.GroupBanNotice) error
	// OnFriendAdd 处理好友添加
	OnFriendAdd(context.Context, bot.Bot, message.FriendAddNotice) error
	// OnGroupRecall 处理群消息撤回
	OnGroupRecall(context.Context, bot.Bot, message.GroupRecallNotice) error
	// OnFriendRecall 处理好友消息撤回
	OnFriendRecall(context.Context, bot.Bot, message.FriendRecallNotice) error
	// OnPoke 处理戳一戳
	OnPoke(context.Context, bot.Bot, message.PokeNotice) error
	// OnLuckyKing 处理运气王
	OnLuckyKing(context.Context, bot.Bot, message.LuckyKingNotice) error
	// OnHonor 处理群荣誉变更
	OnHonor(context.Context, bot.Bot, message.HonorNotice) error
	// OnGroupMsgEmojiLike 处理群消息表情回应
	OnGroupMsgEmojiLike(context.Context, bot.Bot, message.GroupMsgEmojiLikeNotice) error
	// OnEssence 处理群精华消息变更
	OnEssence(context.Context, bot.Bot, message.EssenceNotice) error
	// OnGroupCard 处理群名片变更
	OnGroupCard(context.Context, bot.Bot, message.GroupCardNotice) error
}
