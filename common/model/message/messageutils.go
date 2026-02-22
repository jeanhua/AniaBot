package message

import (
	"encoding/json"
	"fmt"
	"strings"
)

type msgHandleOpt struct {
	groupId              uint
	ignoreMentionId      uint
	getMsgFunc           func(msgId uint) (*Message, bool)
	getGroupUserInfoFunc func(groupId, userId uint) (info *GroupUserInfo, success bool)
	getImageOCRFunc      func(url string) string
	getForwardMsgFunc    func(msgId string) (*[]Message, bool)
}

type MsgOptFunc func(*msgHandleOpt)

func WithIgnoreMentionId(userId uint) MsgOptFunc {
	return func(o *msgHandleOpt) {
		o.ignoreMentionId = userId
	}
}

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

func (s OB11Segment) FriendlyText(optFunc ...MsgOptFunc) string {
	msgFuncs := msgHandleOpt{}
	for _, f := range optFunc {
		f(&msgFuncs)
	}

	switch s.Type {
	case SegmentText:
		var msg TextMessage
		if ok := ParseText(s, &msg); ok {
			return msg.Text
		}
		return "[无法解析的文本消息]"
	case SegmentFace:
		var msg FaceMessage
		if ok := ParseFace(s, &msg); ok {
			if dsc, ok2 := emojiMap[msg.Id]; ok2 {
				return fmt.Sprintf("[QQ表情:%s]", dsc)
			}
		}
		return "[QQ表情]"
	case SegmentImage:
		var msg ImageMessage
		if ok := ParseImage(s, &msg); ok {
			if msgFuncs.getImageOCRFunc != nil {
				var str strings.Builder
				str.WriteString("\n<图片消息>\n")
				str.WriteString(msgFuncs.getImageOCRFunc(msg.Url))
				str.WriteString("\n</图片消息>\n")
				return str.String()
			}
		}
		return "[图片消息]"
	case SegmentRecord:
		var msg RecordMessage
		if ok := ParseRecord(s, &msg); ok {
			return fmt.Sprintf("[录音:%s]", msg.URL)
		}
		return "[录音消息]"
	case SegmentVideo:
		var msg VideoMessage
		if ok := ParseVideo(s, &msg); ok {
			return fmt.Sprintf("[视频:%s]", msg.URL)
		}
		return "[视频消息]"
	case SegmentMention:
		var msg MentionMessage
		if ok := ParseMention(s, &msg); ok {
			if msg.IsAll {
				return "[at:全体成员]"
			}
			if msgFuncs.ignoreMentionId == msg.QQ {
				return ""
			}
			if msgFuncs.getGroupUserInfoFunc != nil {
				if info, success := msgFuncs.getGroupUserInfoFunc(msgFuncs.groupId, msg.QQ); success {
					nickname := info.Card
					if nickname == "" {
						nickname = info.Nickname
					}
					return fmt.Sprintf("[at:%s id:%d]", nickname, msg.QQ)
				}
			}
		}
		return "[at]"
	case SegmentMusic:
		var msg MusicMessage
		if ok := ParseMusic(s, &msg); ok {
			return fmt.Sprintf("[音乐:%s]", msg.Title)
		}
		return "[音乐消息]"
	case SegmentReply:
		var msg ReplyMessage
		if ok := ParseReply(s, &msg); ok {
			if msgFuncs.getMsgFunc != nil {
				if dtMsg, ok2 := msgFuncs.getMsgFunc(msg.Id); ok2 {
					nickname := dtMsg.Sender.Card
					if nickname == "" {
						nickname = dtMsg.Sender.Nickname
					}
					var back strings.Builder
					back.WriteString("\n<reply>\n")
					back.WriteString(fmt.Sprintf("[%s %d]: ", nickname, dtMsg.Sender.UserId))
					for _, m := range dtMsg.Message {
						back.WriteString(m.FriendlyText(
							WithGetGroupUserInfo(msgFuncs.groupId, msgFuncs.getGroupUserInfoFunc),
							WithGetImageOCRFunc(msgFuncs.getImageOCRFunc),
							WithGetForwardMsgFunc(msgFuncs.getForwardMsgFunc),
						))
					}
					back.WriteString("\n</reply>\n")
					return back.String()
				}
			}
		}
		return "[回复消息]"
	case SegmentForward:
		if msgFuncs.getForwardMsgFunc != nil {
			var msg ForwardMessage
			if ok := ParseForward(s, &msg); ok {
				if detail, ok := msgFuncs.getForwardMsgFunc(msg.Id); ok {
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
							// 图片太多容易超时，不解析了
							// WithGetImageOCRFunc(msgFuncs.getImageOCRFunc),

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
			}
		}
		return "[转发消息]"
	case SegmentFile:
		var msg FileMessage
		if ok := ParseFile(s, &msg); ok {
			return fmt.Sprintf("[文件:%s]", msg.File)
		}
		return "[文件消息]"
	case SegmentJson:
		var jsonMap JsonMessage
		if ok := ParseJson(s, &jsonMap); ok {
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
		} else {
			return "[分享卡片: 无法获取内容]"
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
