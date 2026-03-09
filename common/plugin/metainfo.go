package plugin

import (
	"context"
	"log/slog"

	"github.com/go-resty/resty/v2"
	"github.com/jeanhua/AniaBot/common/bot"
	"github.com/jeanhua/AniaBot/common/model/command"
	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/jeanhua/AniaBot/common/storage"
	"github.com/spf13/viper"
)

type Meta struct {
	Name      string // 插件名字
	HelpWords string // 插件帮助字段，发送 /help 指令显示
	AdminOnly bool   // 插件是否为管理员触发(对其他人隐藏)
	Order     int    // 插件执行顺序，从小到大

	Storage     storage.Storage
	RestyClient *resty.Client
	Logger      *slog.Logger
}

func (p *Meta) GetMeta() *Meta {
	return p
}

func (p *Meta) SetStorage(s storage.Storage) {
	p.Storage = s
}

func (p *Meta) SetRestyClient(c *resty.Client) {
	p.RestyClient = c
}

func (p *Meta) SetLogger(logger *slog.Logger) {
	p.Logger = logger
}

// OnGroupMsg 收到群聊消息触发事件
func (p *Meta) OnGroupMsg(ctx context.Context, bot bot.Bot, cmd command.Command, msg message.Message) (bool, error) {
	return true, nil
}

// OnFriendMsg 收到私聊消息触发事件
func (p *Meta) OnFriendMsg(ctx context.Context, bot bot.Bot, cmd command.Command, msg message.Message) (bool, error) {
	return true, nil
}

// Start 插件初始化事件
func (p *Meta) Start(ctx context.Context, cfg *viper.Viper) error {
	return nil
}

// StartCron 初始化cron事件
func (p *Meta) StartCron(ctx context.Context, bot bot.Bot, c CronManager) error {
	return nil
}

// Awake Bot启动完成事件
func (p *Meta) Awake(ctx context.Context, bot bot.Bot) error {
	return nil
}

// OnGroupUpload 处理群文件上传
func (p *Meta) OnGroupUpload(ctx context.Context, bot bot.Bot, notice message.GroupUploadNotice) error {
	return nil
}

// OnGroupAdmin 处理群管理员变动
func (p *Meta) OnGroupAdmin(ctx context.Context, bot bot.Bot, notice message.GroupAdminNotice) error {
	return nil
}

// OnGroupDecrease 处理群成员减少
func (p *Meta) OnGroupDecrease(ctx context.Context, bot bot.Bot, notice message.GroupDecreaseNotice) error {
	return nil
}

// OnGroupIncrease 处理群成员增加
func (p *Meta) OnGroupIncrease(ctx context.Context, bot bot.Bot, notice message.GroupIncreaseNotice) error {
	return nil
}

// OnGroupBan 处理群禁言
func (p *Meta) OnGroupBan(ctx context.Context, bot bot.Bot, notice message.GroupBanNotice) error {
	return nil
}

// OnFriendAdd 处理好友添加
func (p *Meta) OnFriendAdd(ctx context.Context, bot bot.Bot, notice message.FriendAddNotice) error {
	return nil
}

// OnGroupRecall 处理群消息撤回
func (p *Meta) OnGroupRecall(ctx context.Context, bot bot.Bot, notice message.GroupRecallNotice) error {
	return nil
}

// OnFriendRecall 处理好友消息撤回
func (p *Meta) OnFriendRecall(ctx context.Context, bot bot.Bot, notice message.FriendRecallNotice) error {
	return nil
}

// OnPoke 处理戳一戳
func (p *Meta) OnPoke(ctx context.Context, bot bot.Bot, notice message.PokeNotice) error {
	return nil
}

// OnLuckyKing 处理运气王
func (p *Meta) OnLuckyKing(ctx context.Context, bot bot.Bot, notice message.LuckyKingNotice) error {
	return nil
}

// OnHonor 处理群荣誉变更
func (p *Meta) OnHonor(ctx context.Context, bot bot.Bot, notice message.HonorNotice) error {
	return nil
}

// OnGroupMsgEmojiLike 处理群消息表情回应
func (p *Meta) OnGroupMsgEmojiLike(ctx context.Context, bot bot.Bot, notice message.GroupMsgEmojiLikeNotice) error {
	return nil
}

// OnEssence 处理群精华消息变更
func (p *Meta) OnEssence(ctx context.Context, bot bot.Bot, notice message.EssenceNotice) error {
	return nil
}

// OnGroupCard 处理群名片变更
func (p *Meta) OnGroupCard(ctx context.Context, bot bot.Bot, notice message.GroupCardNotice) error {
	return nil
}

// OnPanic 处理插件运行时panic
func (p *Meta) OnPanic(ctx context.Context, bot bot.Bot, name string, err any) {}
