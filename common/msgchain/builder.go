package msgchain

import (
	"encoding/base64"
	"fmt"
	"log"
	"os"

	"github.com/jeanhua/AniaBot/common/model/message"
)

// ---------消息构造入口开始---------
type chainBuilder struct{}

func (c chainBuilder) Friend() FriendChainBuilder {
	return &chain{}
}

func (c chainBuilder) Group() GroupChainBuilder {
	return &chain{}
}

var Buider = chainBuilder{}

// ---------消息构造入口结束---------

type chain struct {
	message []message.OB11Segment
}

type Chain interface {
	GetMsg() []message.OB11Segment
}

func (c *chain) Build() Chain {
	return c
}

func (c *chain) GetMsg() []message.OB11Segment {
	return c.message
}

func (c *chain) Mention(userId uint) {
	c.message = append(c.message, message.OB11Segment{
		Type: "at",
		Data: map[string]interface{}{
			"qq": fmt.Sprintf("%d", userId),
		},
	})
}

func (c *chain) Text(text string) {
	c.message = append(c.message, message.OB11Segment{
		Type: "text",
		Data: map[string]interface{}{
			"text": text,
		},
	})
}

func (c *chain) ImageUrl(url string) {
	c.message = append(c.message, message.OB11Segment{
		Type: "image",
		Data: map[string]interface{}{
			"file":    url,
			"summary": "[图片]",
		},
	})
}

func (c *chain) ImageBase64(bs64code string) {
	c.message = append(c.message, message.OB11Segment{
		Type: "image",
		Data: map[string]interface{}{
			"file":    "base64://" + bs64code,
			"summary": "[图片]",
		},
	})
}

func (c *chain) ImageLocal(path string) {
	file, err := os.ReadFile(path)
	if err != nil {
		log.Println("构造消息出错，无法打开本地图片", err)
		return
	}
	bs64code := base64.StdEncoding.EncodeToString(file)
	c.message = append(c.message, message.OB11Segment{
		Type: "image",
		Data: map[string]interface{}{
			"file":    "base64://" + bs64code,
			"summary": "[图片]",
		},
	})
}

func (c *chain) Reply(msgId uint) {
	c.message = append(c.message, message.OB11Segment{
		Type: "reply",
		Data: map[string]interface{}{
			"id": msgId,
		},
	})
}
