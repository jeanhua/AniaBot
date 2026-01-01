package msgchain

type GroupChainBuilder interface {
	commonMsgBuilder
	Mention(userId uint)
}

type FriendChainBuilder interface {
	commonMsgBuilder
}

type commonMsgBuilder interface {
	Text(text string)
	ImageUrl(url string)
	ImageBase64(bs64code string)
	ImageLocal(path string)
	Reply(msgId uint)
	Build() Chain
}
