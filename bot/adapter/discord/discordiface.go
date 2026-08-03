package discord

import "github.com/bwmarrin/discordgo"

// discordAPI 适配器使用的 discordgo REST 能力窄接口：*discordgo.Session 直接满足，
// 测试注入 fake（与 Telegram 手写客户端的可注入性对齐）。
type discordAPI interface {
	ChannelMessageSend(channelID, content string, options ...discordgo.RequestOption) (*discordgo.Message, error)
	ChannelMessageSendComplex(channelID string, data *discordgo.MessageSend, options ...discordgo.RequestOption) (*discordgo.Message, error)
	ChannelMessageEdit(channelID, messageID, content string, options ...discordgo.RequestOption) (*discordgo.Message, error)
	UserChannelCreate(recipientID string, options ...discordgo.RequestOption) (*discordgo.Channel, error)
	ChannelMessage(channelID, messageID string, options ...discordgo.RequestOption) (*discordgo.Message, error)
	ChannelMessages(channelID string, limit int, beforeID, afterID, aroundID string, options ...discordgo.RequestOption) ([]*discordgo.Message, error)
	Channel(channelID string, options ...discordgo.RequestOption) (*discordgo.Channel, error)
	GuildWithCounts(guildID string, options ...discordgo.RequestOption) (*discordgo.Guild, error)
}
