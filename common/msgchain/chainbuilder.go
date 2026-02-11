package msgchain

import "github.com/jeanhua/AniaBot/common/model/message"

type GroupChainBuilder interface {
	commonMsgBuilder
	// Mention 在群里AT某人
	Mention(userId uint)
	Build() GroupChain
}

type FriendChainBuilder interface {
	commonMsgBuilder
	Build() FriendChain
}

type GroupForwardChainBuilder interface {
	Message(userId uint, nickname string, c GroupChain)
	Build() GroupForwardChain
}
type FriendForwardChainBuilder interface {
	Message(userId uint, nickname string, c FriendChain)
	Build() FriendForwardChain
}
type FriendChain interface {
	GetFriendMsg() []message.OB11Segment
}

type GroupChain interface {
	GetGroupMsg() []message.OB11Segment
}

type GroupForwardChain interface {
	GetForwardMsg() message.ForwardMessage
}

type FriendForwardChain interface {
	GetForwardMsg() message.ForwardMessage
}
type commonMsgBuilder interface {
	// Text 添加文本消息
	Text(text string)
	// Face 添加QQ表情，参考 https://bot.q.qq.com/wiki/develop/api-v2/openapi/emoji/model.html#EmojiType
	Face(faceId uint)
	// ImageUrl 添加图片消息
	ImageUrl(url string)
	// ImageBase64 添加图片消息
	ImageBase64(bs64code string)
	// ImageLocal 添加图片消息，`path`为对adapter(如napcat)的相对路径
	ImageLocal(path string)
	// VideoUrl 添加视频消息
	VideoUrl(url string)
	// VideoLocal 添加视频消息，`path`为对adapter(如napcat)的相对路径
	VideoLocal(path string)
	// VideoBase64 添加视频消息
	VideoBase64(bs64code string)
	// FileUrl 添加文件消息
	FileUrl(name, url string)
	// FileLocal 添加文件消息，`path`为对adapter(如napcat)的相对路径
	FileLocal(name, path string)
	// FileBase64 添加文件消息
	FileBase64(name, bs64code string)
	// Reply 回复消息
	Reply(msgId uint)
	// RecordUrl 添加语音消息
	RecordUrl(url string)
	// RecordLocal 添加语音消息，`path`为对adapter(如napcat)的相对路径
	RecordLocal(path string)
	// RecordeBase64 添加语音消息
	RecordeBase64(bs64code string)
	// Raw 添加OB11Segment裸消息
	Raw(rawMsg ...message.OB11Segment)
}
