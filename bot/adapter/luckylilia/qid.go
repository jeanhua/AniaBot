package luckylilia

import (
	"maps"
	"strings"

	"github.com/jeanhua/AniaBot/common/model/message"
)

// idPrefix luckylilia 平台的框架统一 ID 前缀。
// LLBot 与 NapCat 同为 OneBot v11 QQ 协议端，但注册表要求各适配器 ID 前缀唯一，
// 两者可同时启用（两个 QQ 账号），入站 ID 统一改写为 lil:<数字> 以便出站路由回本适配器。
const idPrefix = "lil:"

// rawLil 去掉 lil: 前缀，返回 LLBot/OneBot 需要的 QQ 原始数字 ID。
func rawLil(q message.QID) string {
	return strings.TrimPrefix(string(q), idPrefix)
}

// rawLilString 对段 Data 中的字符串 ID 执行同样的去前缀处理。
func rawLilString(s string) string {
	return strings.TrimPrefix(s, idPrefix)
}

// toLil 把已按 QQ 规范化（qq: 前缀，QID 反序列化对纯数字自动添加）的 ID
// 改写为 lil: 前缀，其余值（"all"、其他平台 ID）原样保留。
func toLil(q message.QID) message.QID {
	s := string(q)
	if strings.HasPrefix(s, message.QQIDPrefix) {
		return message.QID(idPrefix + s[len(message.QQIDPrefix):])
	}
	return q
}

// relilMessage 入站消息/API 响应的 ID 改写：先经 message.NormalizeQQMessage
// 统一补 qq: 前缀，再整体替换为 lil:，插件与核心路由即可按 lil: 识别本平台消息。
func relilMessage(msg *message.Message) {
	if msg == nil {
		return
	}
	msg.MessageId = toLil(msg.MessageId)
	msg.UserId = toLil(msg.UserId)
	msg.GroupId = toLil(msg.GroupId)
	msg.SelfId = toLil(msg.SelfId)
	msg.Sender.UserId = toLil(msg.Sender.UserId)
	for _, seg := range msg.Message {
		if seg.Data == nil {
			continue
		}
		switch seg.Type {
		case message.SegmentMention:
			if qq, ok := seg.Data["qq"].(string); ok {
				seg.Data["qq"] = string(toLil(message.QID(qq)))
			}
		case message.SegmentReply, message.SegmentForward:
			if id, ok := seg.Data["id"].(string); ok {
				seg.Data["id"] = string(toLil(message.QID(id)))
			}
		}
	}
}

// relilMessages 对消息列表逐条执行 relilMessage。
func relilMessages(msgs []message.Message) {
	for i := range msgs {
		relilMessage(&msgs[i])
	}
}

// stripLilSegments 出站前规范化消息段：移除段内的 lil: 前缀（适配器边界只接收统一
// QID，调用 OneBot API 时必须还原平台原始数字 ID），并把指向图片文件的 file 段
// 转为 image 段（见 message.FileSegmentAsImage）。
func stripLilSegments(segs []message.OB11Segment) []message.OB11Segment {
	out := make([]message.OB11Segment, len(segs))
	for i, seg := range segs {
		out[i] = seg
		if seg.Data == nil {
			continue
		}
		data := make(map[string]any, len(seg.Data))
		maps.Copy(data, seg.Data)
		switch seg.Type {
		case message.SegmentMention:
			if qq, ok := data["qq"].(string); ok && qq != "all" {
				data["qq"] = rawLilString(qq)
			}
		case message.SegmentReply, message.SegmentForward:
			if id, ok := data["id"].(string); ok {
				data["id"] = rawLilString(id)
			}
		case message.SegmentFile:
			if imgData, ok := message.FileSegmentAsImage(seg); ok {
				out[i].Type = message.SegmentImage
				data = imgData
			}
		}
		out[i].Data = data
	}
	return out
}

// stripLilForward 出站前移除合并转发节点里的 lil: 前缀，包含节点内嵌消息段。
func stripLilForward(f message.ForwardMessageSegment) message.ForwardMessageSegment {
	f.Messages = append([]message.NodeMsg(nil), f.Messages...)
	for i := range f.Messages {
		f.Messages[i].Data.UserId = message.QID(rawLil(f.Messages[i].Data.UserId))
		f.Messages[i].Data.Content = stripLilSegments(f.Messages[i].Data.Content)
	}
	return f
}
