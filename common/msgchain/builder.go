package msgchain

import (
	"fmt"

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

func (c *chainBuilder) Mention(userId uint) {
	c.message = append(c.message, message.OB11Segment{
		Type: "at",
		Data: map[string]interface{}{
			"qq": fmt.Sprintf("%d", userId),
		},
	})
}

func (c *chainBuilder) Text(text string) {
	c.message = append(c.message, message.OB11Segment{
		Type: "text",
		Data: map[string]interface{}{
			"text": text,
		},
	})
}

func (c *chainBuilder) Face(faceId uint) {
	c.message = append(c.message, message.OB11Segment{
		Type: "face",
		Data: map[string]interface{}{
			"id": faceId,
		},
	})
}

func (c *chainBuilder) ImageUrl(url string) {
	c.message = append(c.message, message.OB11Segment{
		Type: "image",
		Data: map[string]interface{}{
			"file":    url,
			"summary": "[图片]",
		},
	})
}

func (c *chainBuilder) ImageBase64(bs64code string) {
	c.message = append(c.message, message.OB11Segment{
		Type: "image",
		Data: map[string]interface{}{
			"file":    "base64://" + bs64code,
			"summary": "[图片]",
		},
	})
}

func (c *chainBuilder) ImageLocal(path string) {
	c.message = append(c.message, message.OB11Segment{
		Type: "image",
		Data: map[string]interface{}{
			"file":    "file://" + path,
			"summary": "[图片]",
		},
	})
}

func (c *chainBuilder) VideoUrl(url string) {
	c.message = append(c.message, message.OB11Segment{
		Type: "video",
		Data: map[string]interface{}{
			"file": url,
		},
	})
}

func (c *chainBuilder) VideoLocal(path string) {
	c.message = append(c.message, message.OB11Segment{
		Type: "video",
		Data: map[string]interface{}{
			"file": "file://" + path,
		},
	})
}

func (c *chainBuilder) VideoBase64(bs64code string) {
	c.message = append(c.message, message.OB11Segment{
		Type: "video",
		Data: map[string]interface{}{
			"file": "base64://" + bs64code,
		},
	})
}

func (c *chainBuilder) FileUrl(name, url string) {
	c.message = append(c.message, message.OB11Segment{
		Type: "file",
		Data: map[string]interface{}{
			"file": url,
			"name": name,
		},
	})
}

func (c *chainBuilder) FileLocal(name, path string) {
	c.message = append(c.message, message.OB11Segment{
		Type: "file",
		Data: map[string]interface{}{
			"file": "file://" + path,
			"name": name,
		},
	})
}

func (c *chainBuilder) FileBase64(name, bs64code string) {
	c.message = append(c.message, message.OB11Segment{
		Type: "file",
		Data: map[string]interface{}{
			"file": "base64://" + bs64code,
			"name": name,
		},
	})
}

func (c *chainBuilder) Reply(msgId uint) {
	c.message = append(c.message, message.OB11Segment{
		Type: "reply",
		Data: map[string]interface{}{
			"id": msgId,
		},
	})
}

func (c *chainBuilder) RecordUrl(url string) {
	c.message = append(c.message, message.OB11Segment{
		Type: "record",
		Data: map[string]interface{}{
			"file": url,
		},
	})
}

func (c *chainBuilder) RecordLocal(path string) {
	c.message = append(c.message, message.OB11Segment{
		Type: "record",
		Data: map[string]interface{}{
			"file": "file://" + path,
		},
	})
}

func (c *chainBuilder) RecordBase64(bs64code string) {
	c.message = append(c.message, message.OB11Segment{
		Type: "record",
		Data: map[string]interface{}{
			"file": "base64://" + bs64code,
		},
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

func (c *friendChainBuilder) Face(faceId uint) FriendChainBuilder {
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

func (c *friendChainBuilder) Reply(msgId uint) FriendChainBuilder {
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

func (c *groupChainBuilder) Face(faceId uint) GroupChainBuilder {
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

func (c *groupChainBuilder) Reply(msgId uint) GroupChainBuilder {
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

func (c *groupChainBuilder) Mention(userId uint) GroupChainBuilder {
	c.chainBuilder.Mention(userId)
	return c
}

func (fc *friendForwardChainBuilder) Message(userId uint, nickname string, c FriendChain) {
	fc.message.Messages = append(fc.message.Messages,
		message.NodeMsg{
			Type: "node",
			Data: struct {
				UserId   uint                  `json:"user_id"`
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

func (fc *groupForwardChainBuilder) Message(userId uint, nickname string, c GroupChain) {
	fc.message.Messages = append(fc.message.Messages,
		message.NodeMsg{
			Type: "node",
			Data: struct {
				UserId   uint                  `json:"user_id"`
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
