package msgchain

import (
	"fmt"

	"github.com/jeanhua/AniaBot/common/model/message"
)

// ---------消息构造入口开始---------
type chainBuilder struct {
	message []message.OB11Segment
}

type forardChainBuilder struct {
	message message.ForwardMessage
}

// Friend 私聊消息构造器
func (c *chainBuilder) Friend() FriendChainBuilder {
	return &chainBuilder{
		message: make([]message.OB11Segment, 0),
	}
}

// Group 群聊消息构造器
func (c *chainBuilder) Group() GroupChainBuilder {
	return &chainBuilder{
		message: make([]message.OB11Segment, 0),
	}
}

// Forward 合并转发消息构造器
func (c *chainBuilder) Forward() ForwardChainBuilder {
	return &forardChainBuilder{
		message: message.ForwardMessage{
			Prompt:  "聊天记录",
			Summary: "聊天记录",
			Source:  "聊天记录",
		},
	}
}

var Builder = &chainBuilder{}

// ---------消息构造入口结束---------

func (c *chainBuilder) Build() Chain {
	return c
}

func (fc *forardChainBuilder) Build() ForwardChain {
	return fc
}

func (c *chainBuilder) GetMsg() []message.OB11Segment {
	return c.message
}

func (fc *forardChainBuilder) GetMsg() message.ForwardMessage {
	return fc.message
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

func (c *chainBuilder) Raw(rawMsg ...message.OB11Segment) {
	c.message = append(c.message, rawMsg...)
}

func (fc *forardChainBuilder) Message(userId uint, nickname string, c Chain) {
	fc.message.Messages = append(fc.message.Messages,
		message.NodeMsg{
			Type: "node",
			Data: struct {
				UserId   uint                  "json:\"user_id\""
				Nickname string                "json:\"nickname\""
				Content  []message.OB11Segment "json:\"content\""
			}{
				UserId:   userId,
				Nickname: nickname,
				Content:  c.GetMsg(),
			},
		},
	)
}
