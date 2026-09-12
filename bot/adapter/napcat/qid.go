package napcat

import (
	"maps"
	"strings"

	"github.com/jeanhua/AniaBot/common/model/message"
)

// rawQQ 去掉 qq: 前缀，返回 NapCat/OneBot 需要的 QQ 原始数字 ID。
func rawQQ(q message.QID) string {
	return q.TrimQQPrefix()
}

// rawQQString 对段 Data 中的字符串 ID 执行同样的去前缀处理。
func rawQQString(s string) string {
	return strings.TrimPrefix(s, message.QQIDPrefix)
}

// stripQQSegments 出站前规范化消息段：移除段内的 qq: 前缀（适配器边界只接收统一
// QID，调用 OneBot API 时必须还原平台原始数字 ID），并把指向图片文件的 file 段
// 转为 image 段（见 message.FileSegmentAsImage）。
func stripQQSegments(segs []message.OB11Segment) []message.OB11Segment {
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
				data["qq"] = rawQQString(qq)
			}
		case message.SegmentReply, message.SegmentForward:
			if id, ok := data["id"].(string); ok {
				data["id"] = rawQQString(id)
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

// stripQQForward 出站前移除合并转发节点里的 qq: 前缀，包含节点内嵌消息段。
func stripQQForward(f message.ForwardMessageSegment) message.ForwardMessageSegment {
	f.Messages = append([]message.NodeMsg(nil), f.Messages...)
	for i := range f.Messages {
		f.Messages[i].Data.UserId = message.QID(f.Messages[i].Data.UserId.TrimQQPrefix())
		f.Messages[i].Data.Content = stripQQSegments(f.Messages[i].Data.Content)
	}
	return f
}
