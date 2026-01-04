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
	Text(text string)
	Face(faceId uint) // 参考 https://bot.q.qq.com/wiki/develop/api-v2/openapi/emoji/model.html#EmojiType
	ImageUrl(url string)
	ImageBase64(bs64code string)
	ImageLocal(path string)
	Reply(msgId uint)
	RecordUrl(url string)
	RecordLocal(path string)
	Build() Chain
	Raw(rawMsg []message.OB11Segment)
}
