package message

import (
	"encoding/json"
	"fmt"
	"strings"
)

type msgHandleOpt struct {
	getMsgFunc        func(msgId QID) (*Message, bool)
	getImageOCRFunc   func(url string) string
	getForwardMsgFunc func(msgId QID) (*[]Message, bool)
	noSenderPrefix    bool
}

type MsgOptFunc func(*msgHandleOpt)

func WithGetMsgFunc(getMsgFunc func(msgId QID) (*Message, bool)) MsgOptFunc {
	return func(o *msgHandleOpt) {
		o.getMsgFunc = getMsgFunc
	}
}

func WithGetForwardMsgFunc(getForwardMsgFunc func(msgId QID) (*[]Message, bool)) MsgOptFunc {
	return func(o *msgHandleOpt) {
		o.getForwardMsgFunc = getForwardMsgFunc
	}
}

func WithGetImageOCRFunc(f func(url string) string) MsgOptFunc {
	return func(o *msgHandleOpt) {
		o.getImageOCRFunc = f
	}
}

// WithNoSenderPrefix 不输出开头的 [nickname:… id:…] 发送者前缀。
// 用于日志展示等已单独展示发送者信息的场景。
func WithNoSenderPrefix() MsgOptFunc {
	return func(o *msgHandleOpt) {
		o.noSenderPrefix = true
	}
}

func (raw Message) FriendlyText(showUrl bool, opts ...MsgOptFunc) string {
	msgFuncs := msgHandleOpt{}
	for _, f := range opts {
		f(&msgFuncs)
	}
	var result strings.Builder
	if !msgFuncs.noSenderPrefix {
		nickname := raw.Sender.Card
		if nickname == "" {
			nickname = raw.Sender.Nickname
		}
		if nickname == "" {
			nickname = "用户" // 昵称不可得（如通讯录查询失败/权限缺失）时的兜底
		}
		result.WriteString(fmt.Sprintf("[nickname:%s id:%s]: ", nickname, raw.Sender.UserId.String()))
	}
	for _, s := range raw.Message {
		switch s.Type {
		case SegmentText:
			var msg TextMessage
			if ok := ParseText(s, &msg); ok {
				result.WriteString(msg.Text)
			}
		case SegmentFace:
			var msg FaceMessage
			if ok := ParseFace(s, &msg); ok {
				if dsc, ok2 := emojiMap[msg.Id]; ok2 {
					result.WriteString(fmt.Sprintf("[QQ表情:%s]", dsc))
				} else {
					result.WriteString(fmt.Sprintf("[QQ表情: id %d]", msg.Id))
				}
			}
		case SegmentImage:
			var msg ImageMessage
			if ok := ParseImage(s, &msg); ok {
				if msgFuncs.getImageOCRFunc != nil {
					result.WriteString(fmt.Sprintf("\n<图片消息 %s>\n", msg.Hash()))
					result.WriteString(msgFuncs.getImageOCRFunc(msg.Url))
					result.WriteString(fmt.Sprintf("\n</图片消息 %s>\n", msg.Hash()))
				} else {
					if showUrl {
						// 同时输出短哈希与 URL：哈希用于 load_images 按需加载，
						// URL 供 AI 下载图片到本地（如 bash/file 工具）
						if msg.Url != "" {
							result.WriteString(fmt.Sprintf("[图片 %s url:%s]", msg.Hash(), msg.Url))
						} else {
							result.WriteString(fmt.Sprintf("[图片 %s]", msg.Hash()))
						}
					} else {
						result.WriteString("[图片]")
					}
				}
			}
		case SegmentRecord:
			var msg RecordMessage
			if ok := ParseRecord(s, &msg); ok {
				if showUrl {
					result.WriteString(fmt.Sprintf("[录音:%s]", msg.URL))
				} else {
					result.WriteString("[录音]")
				}
			}
		case SegmentVideo:
			var msg VideoMessage
			if ok := ParseVideo(s, &msg); ok {
				if showUrl {
					result.WriteString(fmt.Sprintf("[视频:%s]", msg.URL))
				} else {
					result.WriteString("[视频]")
				}
			}
		case SegmentMention:
			var msg MentionMessage
			if ok := ParseMention(s, &msg); ok {
				if msg.QQ == raw.Sender.UserId {
					continue
				}
				if msg.IsAll {
					result.WriteString("[at:全体成员]")
				} else if msg.QQ == raw.SelfId {
					result.WriteString("[at我]")
				} else {
					// 无法解析被@用户在本群的真实昵称，仅输出其 id，避免误用发送者昵称造成张冠李戴
					result.WriteString(fmt.Sprintf("[at:id:%s]", msg.QQ))
				}
			}
		case SegmentMusic:
			var msg MusicMessage
			if ok := ParseMusic(s, &msg); ok {
				result.WriteString(fmt.Sprintf("[音乐:%s]", msg.Title))
			}
		case SegmentReply:
			var msg ReplyMessage
			if ok := ParseReply(s, &msg); ok {
				if msgFuncs.getMsgFunc != nil {
					if dtMsg, ok2 := msgFuncs.getMsgFunc(msg.Id); ok2 {
						nickname := dtMsg.Sender.Card
						if nickname == "" {
							nickname = dtMsg.Sender.Nickname
						}
						_ = nickname
						result.WriteString("<reply>\n")
						result.WriteString(dtMsg.FriendlyText(showUrl,
							WithGetImageOCRFunc(msgFuncs.getImageOCRFunc),
							WithGetForwardMsgFunc(msgFuncs.getForwardMsgFunc)))
						result.WriteString("\n</reply>\n")
					}
				}
			}
		case SegmentForward:
			if msgFuncs.getForwardMsgFunc != nil {
				var msg ForwardMessage
				if ok := ParseForward(s, &msg); ok {
					if detail, ok := msgFuncs.getForwardMsgFunc(msg.Id); ok {
						result.WriteString("\n<合并转发消息>")
						for _, msg := range *detail {
							// 与回复段展开保持一致，透传 OCR/转发拉取回调：
							// 合并转发里再嵌套合并转发时，内层转发也能继续递归拉取展开
							result.WriteString(msg.FriendlyText(showUrl,
								WithGetImageOCRFunc(msgFuncs.getImageOCRFunc),
								WithGetForwardMsgFunc(msgFuncs.getForwardMsgFunc)))
						}
						result.WriteString("</合并转发消息>\n")
					} else {
						result.WriteString("[转发消息, 无法获取详情]")
					}
				}
			} else {
				result.WriteString("[转发消息]")
			}
		case SegmentFile:
			var msg FileMessage
			if ok := ParseFile(s, &msg); ok {
				result.WriteString(fmt.Sprintf("[文件:%s fileId:%s url:%s]", msg.File, msg.FileId, msg.URL))
			} else {
				result.WriteString("[文件消息]")
			}
		case SegmentJson:
			var jsonMap JsonMessage
			if ok := ParseJson(s, &jsonMap); ok {
				switch jsonMap.View {
				case "news":
					news := JsonNews{}
					if err := json.Unmarshal(jsonMap.Meta, &news); err != nil {
						result.WriteString("[分享卡片: 无法获取内容]")
					} else {
						result.WriteString(fmt.Sprintf("[分享卡片,标题: %s,描述: %s,链接: (%s)]", news.News.Title, news.News.Desc, news.News.JumpUrl))
					}
				default:
					detail := JsonDetailMeta{}
					if err := json.Unmarshal(jsonMap.Meta, &detail); err != nil {
						result.WriteString("[分享卡片: 无法获取内容]")
					} else {
						result.WriteString(fmt.Sprintf("[分享卡片,标题: %s,描述: %s]", detail.Detail.Title, detail.Detail.Desc))
					}
				}
			} else {
				result.WriteString("[分享卡片: 无法获取内容]")
			}
		default:
			result.WriteString(fmt.Sprintf("[%s]", s.Type))
		}
	}
	return result.String()
}
