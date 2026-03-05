package msgchain

import "github.com/jeanhua/AniaBot/common/model/message"

type GroupChainBuilder interface {
	Text(text string) GroupChainBuilder
	Face(faceId int) GroupChainBuilder
	ImageUrl(url string) GroupChainBuilder
	ImageBase64(bs64code string) GroupChainBuilder
	ImageLocal(path string) GroupChainBuilder
	VideoUrl(url string) GroupChainBuilder
	VideoLocal(path string) GroupChainBuilder
	VideoBase64(bs64code string) GroupChainBuilder
	FileUrl(name, url string) GroupChainBuilder
	FileLocal(name, path string) GroupChainBuilder
	FileBase64(name, bs64code string) GroupChainBuilder
	Reply(msgId message.QID) GroupChainBuilder
	RecordUrl(url string) GroupChainBuilder
	RecordLocal(path string) GroupChainBuilder
	RecordBase64(bs64code string) GroupChainBuilder
	Raw(rawMsg ...message.OB11Segment) GroupChainBuilder

	Mention(userId message.QID) GroupChainBuilder
	Build() GroupChain
}

type FriendChainBuilder interface {
	Text(text string) FriendChainBuilder
	Face(faceId int) FriendChainBuilder
	ImageUrl(url string) FriendChainBuilder
	ImageBase64(bs64code string) FriendChainBuilder
	ImageLocal(path string) FriendChainBuilder
	VideoUrl(url string) FriendChainBuilder
	VideoLocal(path string) FriendChainBuilder
	VideoBase64(bs64code string) FriendChainBuilder
	FileUrl(name, url string) FriendChainBuilder
	FileLocal(name, path string) FriendChainBuilder
	FileBase64(name, bs64code string) FriendChainBuilder
	Reply(msgId message.QID) FriendChainBuilder
	RecordUrl(url string) FriendChainBuilder
	RecordLocal(path string) FriendChainBuilder
	RecordBase64(bs64code string) FriendChainBuilder
	Raw(rawMsg ...message.OB11Segment) FriendChainBuilder

	Build() FriendChain
}

type GroupForwardChainBuilder interface {
	Message(userId message.QID, nickname string, c GroupChain)
	Build() GroupForwardChain
}
type FriendForwardChainBuilder interface {
	Message(userId message.QID, nickname string, c FriendChain)
	Build() FriendForwardChain
}
type FriendChain interface {
	GetFriendMsg() []message.OB11Segment
}

type GroupChain interface {
	GetGroupMsg() []message.OB11Segment
}

type GroupForwardChain interface {
	GetForwardMsg() message.ForwardMessageSegment
}

type FriendForwardChain interface {
	GetForwardMsg() message.ForwardMessageSegment
}
