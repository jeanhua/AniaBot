package msgchain

import "github.com/jeanhua/AniaBot/common/model/message"

type GroupChainBuilder interface {
	commonMsgBuilder
	Mention(userId uint)
}

type FriendChainBuilder interface {
	commonMsgBuilder
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
	// Reply 回复消息
	Reply(msgId uint)
	// RecordUrl 添加语音消息
	RecordUrl(url string)
	// RecordLocal 添加语音消息，`path`为对adapter(如napcat)的相对路径
	RecordLocal(path string)
	// Build 构造消息
	Build() Chain
	// Raw 添加OB11Segment裸消息
	Raw(rawMsg []message.OB11Segment)
}
