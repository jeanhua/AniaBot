package message

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
)

type msgHandleOpt struct {
	groupId              uint
	getMsgFunc           func(msgId uint) (*Message, bool)
	getGroupUserInfoFunc func(groupId, userId uint) (info *GroupUserInfo, success bool)
	getImageOCRFunc      func(url string) string
}

type MsgOptFunc func(*msgHandleOpt)

func WithGetMsgFunc(getMsgFunc func(msgId uint) (*Message, bool)) MsgOptFunc {
	return func(o *msgHandleOpt) {
		o.getMsgFunc = getMsgFunc
	}
}

func WithGetGroupUserInfo(groupId uint, getGroupUserInfo func(groupId, userId uint) (info *GroupUserInfo, success bool)) MsgOptFunc {
	return func(o *msgHandleOpt) {
		o.groupId = groupId
		o.getGroupUserInfoFunc = getGroupUserInfo
	}
}

func WithGetImageOCRFunc(f func(url string) string) MsgOptFunc {
	return func(o *msgHandleOpt) {
		o.getImageOCRFunc = f
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
		url := s.Data["url"].(string)
		if o.getImageOCRFunc != nil {
			var str strings.Builder
			str.WriteString("\n<图片消息>\n")
			str.WriteString(o.getImageOCRFunc(url))
			str.WriteString("\n</图片消息>\n")
			return str.String()
		}
		return fmt.Sprintf("[图片:%s]", url)
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
		info, success := o.getGroupUserInfoFunc(o.groupId, uint(qq))
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
		msg, ok := o.getMsgFunc(uint(id))
		if !ok {
			return "[回复消息]"
		}
		nickname := msg.Sender.Card
		if nickname == "" {
			nickname = msg.Sender.Nickname
		}
		var back strings.Builder
		back.WriteString("\n<reply>\n")
		back.WriteString(fmt.Sprintf("[%s %d]: ", nickname, msg.Sender.UserId))
		for _, m := range msg.Message {
			back.WriteString(m.FriendlyText(
				WithGetGroupUserInfo(o.groupId, o.getGroupUserInfoFunc),
				WithGetImageOCRFunc(o.getImageOCRFunc),
			))
		}
		back.WriteString("\n</reply>\n")
		return back.String()
	case "forward":
		return "[转发消息]"
	case "file":
		return fmt.Sprintf("[文件:%s]", s.Data["file"].(string))
	case "json":
		jsonMap := JsonMessage{}
		err := json.Unmarshal([]byte(s.Data["data"].(string)), &jsonMap)
		if err != nil {
			log.Println("error when json unmarshal: json message", err)
			return "[分享卡片: 无法获取内容]"
		}
		switch jsonMap.View {
		case "news":
			news := JsonNews{}
			if err := json.Unmarshal(jsonMap.Meta, &news); err != nil {
				return "[分享卡片: 无法获取内容]"
			}
			return fmt.Sprintf("[分享卡片,标题: %s,描述: %s,链接: (%s)]", news.News.Title, news.News.Desc, news.News.JumpUrl)
		default:
			detail := JsonDetailMeta{}
			if err := json.Unmarshal(jsonMap.Meta, &detail); err != nil {
				return "[分享卡片: 无法获取内容]"
			}
			return fmt.Sprintf("[分享卡片,标题: %s,描述: %s]", detail.Detail.Title, detail.Detail.Desc)
		}

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
