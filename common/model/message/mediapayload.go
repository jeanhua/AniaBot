package message

import (
	"maps"
	"strings"
)

// isInlinePayload 判断字符串是否为内联二进制负载（base64:// 或 data:）。
// 这类负载会随消息整段进入内存，体积可能达到数 MB，不适合长期驻留缓存。
func isInlinePayload(v string) bool {
	return strings.HasPrefix(v, "base64://") || strings.HasPrefix(v, "data:")
}

// StripInlinePayloadSegments 返回消息段的浅拷贝副本，并把 image/file/video/record
// 段中的内联 base64/data 负载键（file/url）删除，仅保留 http(s) URL、file_id、
// file:// 路径与文本等轻量信息。
//
// 用途：适配器把出站消息写入内存历史缓存（GetMsgDetail/历史兜底）前瘦身，
// 避免发送过的 base64 图片整段常驻内存导致 Bot 堆占用只升不降。
// 原段不被修改：只有确实删除了负载键的段才复制 Data，其余段与原切片共享。
func StripInlinePayloadSegments(segs []OB11Segment) []OB11Segment {
	if len(segs) == 0 {
		return nil
	}
	out := make([]OB11Segment, len(segs))
	for i, seg := range segs {
		out[i] = seg
		if seg.Data == nil {
			continue
		}
		switch seg.Type {
		case SegmentImage, SegmentFile, SegmentVideo, SegmentRecord:
		default:
			continue
		}
		needCopy := false
		for _, key := range []string{"file", "url"} {
			if v, ok := seg.Data[key].(string); ok && isInlinePayload(v) {
				needCopy = true
				break
			}
		}
		if !needCopy {
			continue
		}
		data := make(map[string]any, len(seg.Data))
		maps.Copy(data, seg.Data)
		for _, key := range []string{"file", "url"} {
			if v, ok := data[key].(string); ok && isInlinePayload(v) {
				delete(data, key)
			}
		}
		out[i].Data = data
	}
	return out
}
