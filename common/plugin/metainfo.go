package plugin

import (
	"github.com/jeanhua/AniaBot/common/bot"
	"github.com/jeanhua/AniaBot/common/model/command"
	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/spf13/viper"
)

type Meta struct {
	Name      string // 插件名字
	HelpWords string // 插件帮助字段，发送 /help 指令显示
	AdminOnly bool   // 插件是否为管理员触发(对其他人隐藏)
	Order     int    // 插件执行顺序，从小到大
}

func (p *Meta) GetMeta() *Meta {
	return p
}

func (p *Meta) OnGroupMsg(bot.Bot, *command.Command, message.Message) bool {
	// 收到群聊消息触发事件
	return true
}

func (p *Meta) OnFriendMsg(bot.Bot, *command.Command, message.Message) bool {
	// 收到私聊消息触发事件
	return true
}

func (p *Meta) Start(cfg *viper.Viper) {
	// 插件初始化事件
}

func (p *Meta) OnGroupUpload(b bot.Bot, notice message.GroupUploadNotice) {
	// 处理群文件上传
}

func (p *Meta) OnGroupAdmin(b bot.Bot, notice message.GroupAdminNotice) {
	// 处理群管理员变动
}

func (p *Meta) OnGroupDecrease(b bot.Bot, notice message.GroupDecreaseNotice) {
	// 处理群成员减少
}

func (p *Meta) OnGroupIncrease(b bot.Bot, notice message.GroupIncreaseNotice) {
	// 处理群成员增加
}

func (p *Meta) OnGroupBan(b bot.Bot, notice message.GroupBanNotice) {
	// 处理群禁言
}

func (p *Meta) OnFriendAdd(b bot.Bot, notice message.FriendAddNotice) {
	// 处理好友添加
}

func (p *Meta) OnGroupRecall(b bot.Bot, notice message.GroupRecallNotice) {
	// 处理群消息撤回
}

func (p *Meta) OnFriendRecall(b bot.Bot, notice message.FriendRecallNotice) {
	// 处理好友消息撤回
}

func (p *Meta) OnPoke(b bot.Bot, notice message.PokeNotice) {
	// 处理戳一戳
}

func (p *Meta) OnLuckyKing(b bot.Bot, notice message.LuckyKingNotice) {
	// 处理运气王
}

func (p *Meta) OnHonor(b bot.Bot, notice message.HonorNotice) {
	// 处理群荣誉变更
}

func (p *Meta) OnGroupMsgEmojiLike(b bot.Bot, notice message.GroupMsgEmojiLikeNotice) {
	// 处理群消息表情回应
}

func (p *Meta) OnEssence(b bot.Bot, notice message.EssenceNotice) {
	// 处理群精华消息变更
}

func (p *Meta) OnGroupCard(b bot.Bot, notice message.GroupCardNotice) {
	// 处理群名片变更
}
