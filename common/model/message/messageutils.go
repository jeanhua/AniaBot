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
	getForwardMsgFunc    func(msgId string) (*[]Message, bool)
}

type MsgOptFunc func(*msgHandleOpt)

func WithGetMsgFunc(getMsgFunc func(msgId uint) (*Message, bool)) MsgOptFunc {
	return func(o *msgHandleOpt) {
		o.getMsgFunc = getMsgFunc
	}
}

func WithGetForwardMsgFunc(getForwardMsgFunc func(msgId string) (*[]Message, bool)) MsgOptFunc {
	return func(o *msgHandleOpt) {
		o.getForwardMsgFunc = getForwardMsgFunc
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

	msgFuncs := msgHandleOpt{}
	for _, f := range optFunc {
		f(&msgFuncs)
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
		if msgFuncs.getImageOCRFunc != nil {
			var str strings.Builder
			str.WriteString("\n<图片消息>\n")
			str.WriteString(msgFuncs.getImageOCRFunc(url))
			str.WriteString("\n</图片消息>\n")
			return str.String()
		} else {
			log.Printf("Processing image, OCR func is nil")
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
		info, success := msgFuncs.getGroupUserInfoFunc(msgFuncs.groupId, uint(qq))
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
		if msgFuncs.getMsgFunc == nil {
			return "[回复消息]"
		}
		idStr := s.Data["id"].(string)
		id, err := strconv.Atoi(idStr)
		if err != nil {
			return "[回复消息]"
		}
		msg, ok := msgFuncs.getMsgFunc(uint(id))
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
				WithGetGroupUserInfo(msgFuncs.groupId, msgFuncs.getGroupUserInfoFunc),
				WithGetImageOCRFunc(msgFuncs.getImageOCRFunc),
			))
		}
		back.WriteString("\n</reply>\n")
		return back.String()
	case "forward":
		if msgFuncs.getForwardMsgFunc != nil {
			id := s.Data["id"].(string)
			detail, ok := msgFuncs.getForwardMsgFunc(id)
			if ok {
				builder := strings.Builder{}
				builder.WriteString("\n<合并转发消息>")
				for _, msg := range *detail {
					nickname := msg.Sender.Card
					if nickname == "" {
						nickname = msg.Sender.Nickname
					}
					var str strings.Builder
					for _, m := range msg.Message {
						str.WriteString(m.FriendlyText(
							WithGetImageOCRFunc(msgFuncs.getImageOCRFunc),
							// napcat有问题，不能解析嵌套的合并转发消息，https://github.com/NapNeko/NapCatQQ/issues/1278
							// WithGetForwardMsgFunc(msgFuncs.getForwardMsgFunc),
						))
					}
					builder.WriteString(fmt.Sprintf("\n[nickname: %s id: %d]: %s\n", nickname, msg.Sender.UserId, str.String()))
				}
				builder.WriteString("</合并转发消息>\n")
				return builder.String()
			} else {
				return "[转发消息, 无法获取详情]"
			}
		} else {
			return "[转发消息]"
		}
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

// func (s OB11Segment) ShortText() string {
// 	switch s.Type {
// 	case "text":
// 		return s.Data["text"].(string)
// 	case "face":
// 		idStr := s.Data["id"].(string)
// 		id, err := strconv.Atoi(idStr)
// 		if err != nil {
// 			return "[QQ表情]"
// 		}
// 		dsc, ok := emojiMap[id]
// 		if ok {
// 			return fmt.Sprintf("[QQ表情:%s]", dsc)
// 		} else {
// 			return "[QQ表情]"
// 		}
// 	case "image":
// 		return "[图片]"
// 	case "record":
// 		return "[录音]"
// 	case "video":
// 		return "[视频]"
// 	case "at":
// 		return fmt.Sprintf("[at:%s]", s.Data["qq"].(string))
// 	case "music":
// 		return "[音乐]"
// 	case "reply":
// 		return "[回复消息]"
// 	case "forward":
// 		return "[转发消息]"
// 	case "file":
// 		return fmt.Sprintf("[文件:%s]", s.Data["name"].(string))
// 	default:
// 		return fmt.Sprintf("[%s]", s.Type)
// 	}
// }
