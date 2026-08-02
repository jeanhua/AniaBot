package bot

import (
	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/jeanhua/AniaBot/common/msgchain"
	"github.com/jeanhua/AniaBot/common/plugininfo"
	"github.com/jeanhua/AniaBot/common/tracer"
)

// Bot 平台适配器公共能力外观。任何平台（QQ、飞书、Telegram……）都应能实现
// 这些方法；平台专属能力（戳一戳、合并转发、rkey 等）不在此接口，
// 而是通过可选的 bot.QQ 等扩展接口暴露——插件在事件回调里类型断言探测：
//
//	if qb, ok := b.(bot.QQ); ok { qb.SendPokeMsg(...) }
//
// 断言仅当事件来源适配器实现了对应能力接口时成功（框架在适配器边界包装）。
type Bot interface {
	botSendMsgItf
	botGetMsgItf
	pluginItf
	Stop()

	tracer.Tracer
}

type botSendMsgItf interface {
	// SendGroupMsg 发送群聊消息
	SendGroupMsg(groupId message.QID, chain msgchain.GroupChain) (msgId message.QID, success bool)
	// SendFriendMsg 发送私聊消息
	SendFriendMsg(userId message.QID, chain msgchain.FriendChain) (msgId message.QID, success bool)
}

type botGetMsgItf interface {
	// GetMsgDetail 获取消息详情
	GetMsgDetail(msgId message.QID) (msg *message.Message, success bool)
	// GetGroupDetail 获取群聊详情
	GetGroupDetail(groupId message.QID) (info *message.GroupInfo, success bool)
	// GetGroupMsgHistory 获取群聊消息历史记录
	GetGroupMsgHistory(groupId message.QID, count int, message_seq int) (*[]message.Message, bool)
	// GetFriendMsgHistory 获取私聊消息历史记录
	GetFriendMsgHistory(userId message.QID, count int, message_seq int) (*[]message.Message, bool)
}

type pluginItf interface {
	// GetPluginList 获取插件列表
	GetPluginList() []plugininfo.PluginInfo
}

// StreamHandle 流式消息句柄：Patch 更新已发送消息的内容；End 结束（此后不可再 Patch）。
type StreamHandle interface {
	// Patch 将已发送消息的内容替换为 text（实现方负责节流与内容上限）
	Patch(text string) error
	// End 结束流式消息（强制发送最终内容，幂等）
	End()
}

// StreamSender 流式回复能力，可选接口（与 bot.QQ 同模式）。
// 平台支持「先发后改」（如飞书卡片 Patch）时，事件回调收到的 bot.Bot 可断言为
// bot.StreamSender；QQ/OneBot v11 无消息编辑 API，断言失败退化为一次性回复。
type StreamSender interface {
	// SendGroupStream 以 chain 的初始内容创建一条流式群聊消息，返回可更新的句柄。
	// 平台不支持时返回 (nil, false)。
	SendGroupStream(groupId message.QID, chain msgchain.GroupChain) (StreamHandle, bool)
	// SendFriendStream 以 chain 的初始内容创建一条流式私聊消息，返回可更新的句柄。
	// 平台不支持时返回 (nil, false)。
	SendFriendStream(userId message.QID, chain msgchain.FriendChain) (StreamHandle, bool)
}

// QQ QQ（NapCat/OneBot v11）平台专属能力，可选接口。
// 事件来源为 QQ 适配器时，事件回调收到的 bot.Bot 可断言为 bot.QQ。
type QQ interface {
	qqSendMsgItf
	qqGetMsgItf
	qqSysItf
}

type qqSendMsgItf interface {
	// SendGroupAIVoiceMsg 发送群聊AI语音消息
	SendGroupAIVoiceMsg(groupId message.QID, character, msg string) (msgId message.QID, success bool)
	// SendPokeMsg 发送戳一戳消息
	SendPokeMsg(userId message.QID, groupId *message.QID) (success bool)
	// SendGroupForwardMsg 发送群聊合并转发消息
	SendGroupForwardMsg(groupId message.QID, chain msgchain.GroupForwardChain) (msgId message.QID, success bool)
	// SendFriendForwardMsg 发送私聊合并转发消息
	SendFriendForwardMsg(userId message.QID, chain msgchain.FriendForwardChain) (msgId message.QID, success bool)
	// SetMsgEmojiLike 设置消息表情点赞
	SetMsgEmojiLike(msgId message.QID, emojiId int, like bool) (success bool)
	// SendGroupSign 群打卡
	SendGroupSign(groupId message.QID) (success bool)
}

type qqGetMsgItf interface {
	// GetForwardMsg 获取合并转发消息详情
	GetForwardMsg(msgId message.QID) (msgs *[]message.Message, success bool)
	// GetGroupUserInfo 获取群聊中某成员信息
	GetGroupUserInfo(groupId, userId message.QID) (info *message.GroupUserInfo, success bool)
	// GetFriendList 获取好友列表
	GetFriendList() (*[]message.Friend, bool)
	// GetGroupList 获取群聊列表
	GetGroupList() (*[]message.GroupInfo, bool)
	// GetAIChatacter 获取AI角色列表
	GetAIChatacter() (*[]message.AIChatacter, bool)
	// GetPrivateFileURL 获取私聊文件URL
	GetPrivateFileURL(userId message.QID, fileId string) (string, bool)
}

type qqSysItf interface {
	// GetNCrkey 获取rkey
	GetNCrkey() ([]message.NCrkey, bool)
}
