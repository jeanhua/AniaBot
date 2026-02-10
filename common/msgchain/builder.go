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
	message message.ForwardMessage
}

type groupForwardChainBuilder struct {
	forwardChainBuilder
}

type friendForwardChainBuilder struct {
	forwardChainBuilder
}

// Friend 私聊消息构造器
func (c *chainBuilder) Friend() FriendChainBuilder {
	return &friendChainBuilder{
		chainBuilder: chainBuilder{
			message: make([]message.OB11Segment, 0),
		},
	}
}

// Group 群聊消息构造器
func (c *chainBuilder) Group() GroupChainBuilder {
	return &groupChainBuilder{
		chainBuilder: chainBuilder{
			message: make([]message.OB11Segment, 0),
		},
	}
}

// FriendForward 好友合并转发消息构造器
func (c *chainBuilder) FriendForward() FriendForwardChainBuilder {
	return &friendForwardChainBuilder{
		forwardChainBuilder: forwardChainBuilder{
			message: message.ForwardMessage{
				Prompt:  "[聊天记录]",
				Summary: "[聊天记录]",
				Source:  "[聊天记录]",
			},
		},
	}
}

// GroupForward 群聊合并转发消息构造器
func (c *chainBuilder) GroupForward() GroupForwardChainBuilder {
	return &groupForwardChainBuilder{
		forwardChainBuilder: forwardChainBuilder{
			message: message.ForwardMessage{
				Prompt:  "[聊天记录]",
				Summary: "[聊天记录]",
				Source:  "[聊天记录]",
			},
		},
	}
}

var Builder = &chainBuilder{}

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

func (fc *friendForwardChainBuilder) GetFriendForwardMsg() message.ForwardMessage {
	return fc.message
}

func (fc *groupForwardChainBuilder) GetGroupForwardMsg() message.ForwardMessage {
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

func (c *chainBuilder) RecordeBase64(bs64code string) {
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
