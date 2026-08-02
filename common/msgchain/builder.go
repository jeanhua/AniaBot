package msgchain

import (
	"github.com/jeanhua/AniaBot/common/model/message"
)

// ---------消息构造入口开始---------

type chainBuilder struct {
	message []message.OB11Segment
}

type friendChainBuilder struct {
	chainBuilder
}

type groupChainBuilder struct {
	chainBuilder
}

type forwardChainBuilder struct {
	message message.ForwardMessageSegment
}

type groupForwardChainBuilder struct {
	forwardChainBuilder
}

type friendForwardChainBuilder struct {
	forwardChainBuilder
}

// Friend 私聊消息构造器
func (c chainBuilder) Friend() FriendChainBuilder {
	return &friendChainBuilder{
		chainBuilder: chainBuilder{
			message: make([]message.OB11Segment, 0),
		},
	}
}

// Group 群聊消息构造器
func (c chainBuilder) Group() GroupChainBuilder {
	return &groupChainBuilder{
		chainBuilder: chainBuilder{
			message: make([]message.OB11Segment, 0),
		},
	}
}

// FriendForward 好友合并转发消息构造器
func (c chainBuilder) FriendForward() FriendForwardChainBuilder {
	return &friendForwardChainBuilder{
		forwardChainBuilder: forwardChainBuilder{
			message: message.ForwardMessageSegment{
				Prompt:  "[聊天记录]",
				Summary: "[聊天记录]",
				Source:  "[聊天记录]",
			},
		},
	}
}

// GroupForward 群聊合并转发消息构造器
func (c chainBuilder) GroupForward() GroupForwardChainBuilder {
	return &groupForwardChainBuilder{
		forwardChainBuilder: forwardChainBuilder{
			message: message.ForwardMessageSegment{
				Prompt:  "[聊天记录]",
				Summary: "[聊天记录]",
				Source:  "[聊天记录]",
			},
		},
	}
}

func Builder() chainBuilder {
	return chainBuilder{}
}

// ---------消息构造入口结束---------

func (c *friendChainBuilder) Build() FriendChain {
	return c
}

func (c *groupChainBuilder) Build() GroupChain {
	return c
}

func (fc *friendForwardChainBuilder) Build() FriendForwardChain {
	return fc
}

func (fc *groupForwardChainBuilder) Build() GroupForwardChain {
	return fc
}

func (fc *friendForwardChainBuilder) GetForwardMsg() message.ForwardMessageSegment {
	return fc.message
}

func (fc *groupForwardChainBuilder) GetForwardMsg() message.ForwardMessageSegment {
	return fc.message
}

func (c *groupChainBuilder) GetGroupMsg() []message.OB11Segment {
	return c.message
}

func (c *friendChainBuilder) GetFriendMsg() []message.OB11Segment {
	return c.message
}

func (c *chainBuilder) Mention(userId message.QID) {
	c.message = append(c.message, message.OB11Segment{
		Type: "at",
		Data: message.MentionMessage{QQ: userId}.Marshal(),
	})
}

func (c *chainBuilder) Text(text string) {
	c.message = append(c.message, message.OB11Segment{
		Type: "text",
		Data: message.TextMessage{Text: text}.Marshal(),
	})
}

func (c *chainBuilder) Face(faceId int) {
	c.message = append(c.message, message.OB11Segment{
		Type: "face",
		Data: message.FaceMessage{Id: faceId}.Marshal(),
	})
}

func (c *chainBuilder) ImageUrl(url string) {
	// 同时写 file 与 url：ParseImage 依赖 url（FriendlyText 渲染 / 历史回放）
	c.message = append(c.message, message.OB11Segment{
		Type: "image",
		Data: message.ImageMessage{File: url, Url: url, Summary: "[图片]"}.Marshal(),
	})
}

func (c *chainBuilder) ImageBase64(bs64code string) {
	c.message = append(c.message, message.OB11Segment{
		Type: "image",
		Data: message.ImageMessage{File: "base64://" + bs64code, Url: "base64://" + bs64code, Summary: "[图片]"}.Marshal(),
	})
}

func (c *chainBuilder) ImageLocal(path string) {
	c.message = append(c.message, message.OB11Segment{
		Type: "image",
		Data: message.ImageMessage{File: "file://" + path, Url: "file://" + path, Summary: "[图片]"}.Marshal(),
	})
}

func (c *chainBuilder) VideoUrl(url string) {
	// 同时写 file 与 url：ParseVideo 依赖 url
	c.message = append(c.message, message.OB11Segment{
		Type: "video",
		Data: message.VideoMessage{URL: url}.Marshal(),
	})
}

func (c *chainBuilder) VideoLocal(path string) {
	c.message = append(c.message, message.OB11Segment{
		Type: "video",
		Data: message.VideoMessage{URL: "file://" + path}.Marshal(),
	})
}

func (c *chainBuilder) VideoBase64(bs64code string) {
	c.message = append(c.message, message.OB11Segment{
		Type: "video",
		Data: message.VideoMessage{URL: "base64://" + bs64code}.Marshal(),
	})
}

func (c *chainBuilder) FileUrl(name, url string) {
	c.message = append(c.message, message.OB11Segment{
		Type: "file",
		Data: message.FileMessage{File: url, Name: name}.Marshal(),
	})
}

func (c *chainBuilder) FileLocal(name, path string) {
	c.message = append(c.message, message.OB11Segment{
		Type: "file",
		Data: message.FileMessage{File: "file://" + path, Name: name}.Marshal(),
	})
}

func (c *chainBuilder) FileBase64(name, bs64code string) {
	c.message = append(c.message, message.OB11Segment{
		Type: "file",
		Data: message.FileMessage{File: "base64://" + bs64code, Name: name}.Marshal(),
	})
}

func (c *chainBuilder) Reply(msgId message.QID) {
	c.message = append(c.message, message.OB11Segment{
		Type: "reply",
		Data: message.ReplyMessage{Id: msgId}.Marshal(),
	})
}

func (c *chainBuilder) RecordUrl(url string) {
	c.message = append(c.message, message.OB11Segment{
		Type: "record",
		Data: message.RecordMessage{URL: url}.Marshal(),
	})
}

func (c *chainBuilder) RecordLocal(path string) {
	c.message = append(c.message, message.OB11Segment{
		Type: "record",
		Data: message.RecordMessage{URL: "file://" + path}.Marshal(),
	})
}

func (c *chainBuilder) RecordBase64(bs64code string) {
	c.message = append(c.message, message.OB11Segment{
		Type: "record",
		Data: message.RecordMessage{URL: "base64://" + bs64code}.Marshal(),
	})
}

func (c *chainBuilder) Raw(rawMsg ...message.OB11Segment) {
	c.message = append(c.message, rawMsg...)
}

// FriendChainBuilder 链式方法

func (c *friendChainBuilder) Text(text string) FriendChainBuilder {
	c.chainBuilder.Text(text)
	return c
}

func (c *friendChainBuilder) Face(faceId int) FriendChainBuilder {
	c.chainBuilder.Face(faceId)
	return c
}

func (c *friendChainBuilder) ImageUrl(url string) FriendChainBuilder {
	c.chainBuilder.ImageUrl(url)
	return c
}

func (c *friendChainBuilder) ImageBase64(bs64code string) FriendChainBuilder {
	c.chainBuilder.ImageBase64(bs64code)
	return c
}

func (c *friendChainBuilder) ImageLocal(path string) FriendChainBuilder {
	c.chainBuilder.ImageLocal(path)
	return c
}

func (c *friendChainBuilder) VideoUrl(url string) FriendChainBuilder {
	c.chainBuilder.VideoUrl(url)
	return c
}

func (c *friendChainBuilder) VideoLocal(path string) FriendChainBuilder {
	c.chainBuilder.VideoLocal(path)
	return c
}

func (c *friendChainBuilder) VideoBase64(bs64code string) FriendChainBuilder {
	c.chainBuilder.VideoBase64(bs64code)
	return c
}

func (c *friendChainBuilder) FileUrl(name, url string) FriendChainBuilder {
	c.chainBuilder.FileUrl(name, url)
	return c
}

func (c *friendChainBuilder) FileLocal(name, path string) FriendChainBuilder {
	c.chainBuilder.FileLocal(name, path)
	return c
}

func (c *friendChainBuilder) FileBase64(name, bs64code string) FriendChainBuilder {
	c.chainBuilder.FileBase64(name, bs64code)
	return c
}

func (c *friendChainBuilder) Reply(msgId message.QID) FriendChainBuilder {
	c.chainBuilder.Reply(msgId)
	return c
}

func (c *friendChainBuilder) RecordUrl(url string) FriendChainBuilder {
	c.chainBuilder.RecordUrl(url)
	return c
}

func (c *friendChainBuilder) RecordLocal(path string) FriendChainBuilder {
	c.chainBuilder.RecordLocal(path)
	return c
}

func (c *friendChainBuilder) RecordBase64(bs64code string) FriendChainBuilder {
	c.chainBuilder.RecordBase64(bs64code)
	return c
}

func (c *friendChainBuilder) Raw(rawMsg ...message.OB11Segment) FriendChainBuilder {
	c.chainBuilder.Raw(rawMsg...)
	return c
}

// GroupChainBuilder 链式方法
func (c *groupChainBuilder) Text(text string) GroupChainBuilder {
	c.chainBuilder.Text(text)
	return c
}

func (c *groupChainBuilder) Face(faceId int) GroupChainBuilder {
	c.chainBuilder.Face(faceId)
	return c
}

func (c *groupChainBuilder) ImageUrl(url string) GroupChainBuilder {
	c.chainBuilder.ImageUrl(url)
	return c
}

func (c *groupChainBuilder) ImageBase64(bs64code string) GroupChainBuilder {
	c.chainBuilder.ImageBase64(bs64code)
	return c
}

func (c *groupChainBuilder) ImageLocal(path string) GroupChainBuilder {
	c.chainBuilder.ImageLocal(path)
	return c
}

func (c *groupChainBuilder) VideoUrl(url string) GroupChainBuilder {
	c.chainBuilder.VideoUrl(url)
	return c
}

func (c *groupChainBuilder) VideoLocal(path string) GroupChainBuilder {
	c.chainBuilder.VideoLocal(path)
	return c
}

func (c *groupChainBuilder) VideoBase64(bs64code string) GroupChainBuilder {
	c.chainBuilder.VideoBase64(bs64code)
	return c
}

func (c *groupChainBuilder) FileUrl(name, url string) GroupChainBuilder {
	c.chainBuilder.FileUrl(name, url)
	return c
}

func (c *groupChainBuilder) FileLocal(name, path string) GroupChainBuilder {
	c.chainBuilder.FileLocal(name, path)
	return c
}

func (c *groupChainBuilder) FileBase64(name, bs64code string) GroupChainBuilder {
	c.chainBuilder.FileBase64(name, bs64code)
	return c
}

func (c *groupChainBuilder) Reply(msgId message.QID) GroupChainBuilder {
	c.chainBuilder.Reply(msgId)
	return c
}

func (c *groupChainBuilder) RecordUrl(url string) GroupChainBuilder {
	c.chainBuilder.RecordUrl(url)
	return c
}

func (c *groupChainBuilder) RecordLocal(path string) GroupChainBuilder {
	c.chainBuilder.RecordLocal(path)
	return c
}

func (c *groupChainBuilder) RecordBase64(bs64code string) GroupChainBuilder {
	c.chainBuilder.RecordBase64(bs64code)
	return c
}

func (c *groupChainBuilder) Raw(rawMsg ...message.OB11Segment) GroupChainBuilder {
	c.chainBuilder.Raw(rawMsg...)
	return c
}

func (c *groupChainBuilder) Mention(userId message.QID) GroupChainBuilder {
	c.chainBuilder.Mention(userId)
	return c
}

func (fc *friendForwardChainBuilder) Message(userId message.QID, nickname string, c FriendChain) {
	fc.message.Messages = append(fc.message.Messages,
		message.NodeMsg{
			Type: "node",
			Data: struct {
				UserId   message.QID           `json:"user_id"`
				Nickname string                `json:"nickname"`
				Content  []message.OB11Segment `json:"content"`
			}{
				UserId:   userId,
				Nickname: nickname,
				Content:  c.GetFriendMsg(),
			},
		},
	)
}

func (fc *groupForwardChainBuilder) Message(userId message.QID, nickname string, c GroupChain) {
	fc.message.Messages = append(fc.message.Messages,
		message.NodeMsg{
			Type: "node",
			Data: struct {
				UserId   message.QID           `json:"user_id"`
				Nickname string                `json:"nickname"`
				Content  []message.OB11Segment `json:"content"`
			}{
				UserId:   userId,
				Nickname: nickname,
				Content:  c.GetGroupMsg(),
			},
		},
	)
}
