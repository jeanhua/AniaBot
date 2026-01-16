package message

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
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

type msgHandleOpt struct {
	userId           uint
	groupId          uint
	getMsgFunc       func(msgId uint) (bool, *Message)
	getGroupUserInfo func(groupId, userId uint) (success bool, info *GroupUserInfo)
}

type MsgOptFunc func(*msgHandleOpt)

func WithGetMsgFunc(getMsgFunc func(msgId uint) (bool, *Message)) MsgOptFunc {
	return func(o *msgHandleOpt) {
		o.getMsgFunc = getMsgFunc
	}
}

func WithGetGroupUserInfo(groupId, userId uint, getGroupUserInfo func(groupId, userId uint) (success bool, info *GroupUserInfo)) MsgOptFunc {
	return func(o *msgHandleOpt) {
		o.groupId = groupId
		o.userId = userId
		o.getGroupUserInfo = getGroupUserInfo
	}
}

func (s OB11Segment) FriendlyText(optFunc ...MsgOptFunc) (text string) {
	defer func() {
		if err := recover(); err != nil {
			text = "读取消息错误"
		}
	}()

	o := msgHandleOpt{}
	for _, f := range optFunc {
		f(&o)
	}

	switch s.Type {
	case "text":
		return s.Data["text"].(string)
	case "face":
		idStr := s.Data["id"].(string)
		id, err := strconv.Atoi(idStr)
		if err != nil {
			return "[QQ表情]"
		}
		dsc, ok := emojiMap[id]
		if ok {
			return fmt.Sprintf("[QQ表情:%s]", dsc)
		} else {
			return "[QQ表情]"
		}
	case "image":
		return fmt.Sprintf("[图片:%s]", s.Data["url"].(string))
	case "record":
		return fmt.Sprintf("[录音:%s]", s.Data["url"].(string))
	case "video":
		return fmt.Sprintf("[视频:%s]", s.Data["url"].(string))
	case "at":
		qqStr := s.Data["qq"].(string)
		qq, err := strconv.Atoi(qqStr)
		if err != nil {
			return fmt.Sprintf("[at:%s]", s.Data["qq"].(string))
		}
		success, info := o.getGroupUserInfo(o.groupId, uint(qq))
		if success && info != nil {
			nickname := info.Card
			if nickname == "" {
				nickname = info.Nickname
			}
			return fmt.Sprintf("[at:%s id:%s]", nickname, qqStr)
		}
		return fmt.Sprintf("[at:%s]", s.Data["qq"].(string))
	case "music":
		return fmt.Sprintf("[音乐:%s]", s.Data["title"].(string))
	case "reply":
		if o.getMsgFunc == nil {
			return "[回复消息]"
		}
		idStr := s.Data["id"].(string)
		id, err := strconv.Atoi(idStr)
		if err != nil {
			return "[回复消息]"
		}
		ok, msg := o.getMsgFunc(uint(id))
		if !ok {
			return "[回复消息]"
		}
		nickname := msg.Sender.Card
		if nickname == "" {
			nickname = msg.Sender.Nickname
		}
		var back strings.Builder
		back.WriteString("\n---以下为回复的消息内容---\n")
		back.WriteString(fmt.Sprintf("[%s %d]: ", nickname, msg.Sender.UserId))
		for _, m := range msg.Message {
			back.WriteString(m.FriendlyText())
		}
		back.WriteString("\n---以上为回复的消息内容---\n")
		return back.String()
	case "forward":
		return "[转发消息]"
	case "file":
		return fmt.Sprintf("[文件:%s]", s.Data["name"].(string))
	case "json":
		jsonMap := JsonMessage{}
		err := json.Unmarshal([]byte(s.Data["data"].(string)), &jsonMap)
		if err != nil {
			log.Println("error when json unmarshal: json message")
			return "[分享卡片: 无法获取内容]"
		}
		return fmt.Sprintf("[分享卡片,标题: %s,描述: %s,链接: (%s)]", jsonMap.Meta.News.Title, jsonMap.Meta.News.Desc, jsonMap.Meta.News.JumpUrl)
	default:
		return fmt.Sprintf("[%s]", s.Type)
	}
}

func (s OB11Segment) ShortText() string {
	switch s.Type {
	case "text":
		return s.Data["text"].(string)
	case "face":
		idStr := s.Data["id"].(string)
		id, err := strconv.Atoi(idStr)
		if err != nil {
			return "[QQ表情]"
		}
		dsc, ok := emojiMap[id]
		if ok {
			return fmt.Sprintf("[QQ表情:%s]", dsc)
		} else {
			return "[QQ表情]"
		}
	case "image":
		return "[图片]"
	case "record":
		return "[录音]"
	case "video":
		return "[视频]"
	case "at":
		return fmt.Sprintf("[at:%s]", s.Data["qq"].(string))
	case "music":
		return "[音乐]"
	case "reply":
		return "[回复消息]"
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

type Response[T any] struct {
	Status  string `json:"status"`
	RetCode int    `json:"retcode"`
	Data    T      `json:"data"`
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

type GroupUserInfo struct {
	GroupID         uint   `json:"group_id"`
	UserID          uint   `json:"user_id"`
	Nickname        string `json:"nickname"`
	Card            string `json:"card"`
	Sex             string `json:"sex"`
	Age             int    `json:"age"`
	JoinTime        uint   `json:"join_time"`
	LastSentTime    uint   `json:"last_sent_time"`
	Level           string `json:"level"`
	QqLevel         int    `json:"qq_level"`
	Role            string `json:"role"`
	Title           string `json:"title"`
	Area            string `json:"area"`
	Unfriendly      bool   `json:"unfriendly"`
	TitleExpireTime uint   `json:"title_expire_time"`
	CardChangeable  bool   `json:"card_changeable"`
	ShutUpTimestamp uint   `json:"shut_up_timestamp"`
	IsRobot         bool   `json:"is_robot"`
}
