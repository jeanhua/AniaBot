package message

import (
	"fmt"
)

type Message struct {
	Time        uint          `json:"time"`
	PostType    string        `json:"post_type"`
	MessageType string        `json:"message_type"`
	SubType     string        `json:"sub_type"`
	MessageId   uint          `json:"message_id"`
	UserId      uint          `json:"user_id"`
	GroupId     uint          `json:"group_id"`
	Message     []OB11Segment `json:"message"`
	RawMessage  string        `json:"raw_message"`
	Sender      MessageSender `json:"sender"`
	SelfId      uint          `json:"self_id"`
}

type ForwardMessage struct {
	Messages []NodeMsg                `json:"messages"`
	News     []map[string]interface{} `json:"news"`
	Prompt   string                   `json:"prompt"`
	Summary  string                   `json:"summary"`
	Source   string                   `json:"source"`
}

type GroupForwardMessage struct {
	GroupId uint `json:"group_id"`
	ForwardMessage
}

type FriendForwardMessage struct {
	UserId uint `json:"user_id"`
	ForwardMessage
}

type NodeMsg struct {
	Type string `json:"type"` // node
	Data struct {
		UserId   uint          `json:"user_id"`
		Nickname string        `json:"nickname"`
		Content  []OB11Segment `json:"content"`
	} `json:"data"`
}

type OB11Segment struct {
	Type string                 `json:"type"`
	Data map[string]interface{} `json:"data"`
}

func (s OB11Segment) FriendlyText() string {
	switch s.Type {
	case "text":
		return s.Data["text"].(string)
	case "face":
		return fmt.Sprintf("[QQ表情:%s]", s.Data["id"].(string))
	case "image":
		return fmt.Sprintf("[图片:%s]", s.Data["url"].(string))
	case "record":
		return fmt.Sprintf("[录音:%s]", s.Data["url"].(string))
	case "video":
		return fmt.Sprintf("[视频:%s]", s.Data["url"].(string))
	case "at":
		return fmt.Sprintf("[at:%s]", s.Data["qq"].(string))
	case "music":
		return fmt.Sprintf("[音乐:%s]", s.Data["title"].(string))
	case "reply":
		return fmt.Sprintf("[回复:%s]", s.Data["id"].(string))
	case "forward":
		return "[转发消息]"
	case "file":
		return fmt.Sprintf("[文件:%s]", s.Data["name"].(string))
	default:
		return fmt.Sprintf("[%s]", s.Type)
	}
}

type MessageSender struct {
	UserId   uint   `json:"user_id"`
	Nickname string `json:"nickname"`
	Sex      string `json:"sex"`
	Card     string `json:"card"`
	Role     string `json:"role"`
}

type Response struct {
	Status  string  `json:"status"`
	RetCode int     `json:"retcode"`
	Data    Message `json:"data"`
}

type JsonMessage struct {
	App    string `json:"app"`
	Bizsrc string `json:"bizsrc"`
	Config struct {
		Ctime   int64  `json:"ctime"`
		Forward int    `json:"forward"`
		Token   string `json:"token"`
		Type    string `json:"type"`
	} `json:"config"`
	Extra struct {
		AppType int   `json:"app_type"`
		Appid   int64 `json:"appid"`
		MsgSeq  int64 `json:"msg_seq"`
		Uin     int64 `json:"uin"`
	} `json:"extra"`
	Meta struct {
		News struct {
			AppType int    `json:"app_type"`
			Appid   int64  `json:"appid"`
			Ctime   int64  `json:"ctime"`
			Desc    string `json:"desc"`
			JumpUrl string `json:"jumpUrl"`
			Preview string `json:"preview"`
			Tag     string `json:"tag"`
			TagIcon string `json:"tagIcon"`
			Title   string `json:"title"`
			Uin     int64  `json:"uin"`
		} `json:"news"`
	} `json:"meta"`
	Prompt string `json:"prompt"`
	Ver    string `json:"ver"`
	View   string `json:"view"`
}

type AiVoiceMsg struct {
	GroupId   uint   `json:"group_id"`
	Character string `json:"character"`
	Text      string `json:"text"`
}
