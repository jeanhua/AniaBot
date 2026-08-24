package napcat

import (
	"encoding/base64"
	"maps"
	"path/filepath"
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
// 转为 image 段（见 fileSegmentToImage）。
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
			if imgData, ok := fileSegmentToImage(data); ok {
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

// fileSegmentToImage 判断 file 段指向的是否为图片文件：是则返回对应的 image 段数据
// （file 源保持不变，url/summary 与 msgchain 的 ImageBase64/ImageUrl 一致），使
// NapCat 走图片发送通道（群内/私聊内联展示），而不是文件发送通道（附件）。
// 仅当存在 file 源（路径/URL/base64）时才能转换，只有 file_id 的段无法作为图片发送。
func fileSegmentToImage(data map[string]any) (map[string]any, bool) {
	file, _ := data["file"].(string)
	if file == "" {
		return nil, false
	}
	name, _ := data["name"].(string)
	if !isImageSource(file, name) {
		return nil, false
	}
	img := map[string]any{
		"file":    file,
		"summary": "[图片]",
	}
	if url, ok := data["url"].(string); ok && url != "" {
		img["url"] = url
	} else {
		img["url"] = file
	}
	return img, true
}

// 常见图片扩展名（QQ 支持内联展示的位图格式）。
var imageExts = map[string]bool{
	".png":  true,
	".jpg":  true,
	".jpeg": true,
	".gif":  true,
	".webp": true,
	".bmp":  true,
}

// isImageSource 判断文件源是否指向图片：
// base64:// 与 data:image/ 直接嗅探内容字节，最准确；URL/本地路径无法在不下载/读盘
// 的前提下确认内容，退化为扩展名判断（优先用 name，缺省回退 file 源）。
func isImageSource(file, name string) bool {
	switch {
	case strings.HasPrefix(file, "data:image/"):
		return true
	case strings.HasPrefix(file, "base64://"):
		return sniffImageBase64(strings.TrimPrefix(file, "base64://"))
	case strings.HasPrefix(file, "data:"):
		// 其他 data: 类型（非图片 MIME），不做转换
		return false
	}
	ext := strings.ToLower(filepath.Ext(name))
	if ext == "" {
		ext = sourceExt(file)
	}
	return imageExts[ext]
}

// sourceExt 从文件源（URL/本地路径等）提取扩展名，忽略查询参数与锚点、协议前缀。
func sourceExt(src string) string {
	src = strings.SplitN(src, "?", 2)[0]
	src = strings.SplitN(src, "#", 2)[0]
	if i := strings.Index(src, "://"); i >= 0 {
		src = src[i+3:]
	}
	return strings.ToLower(filepath.Ext(src))
}

// sniffImageBase64 解码 base64 头字节（图片格式魔数都在前 16 字节内），
// 按魔数判断是否为常见图片格式；解码失败时返回 false（不转换，保持文件发送）。
func sniffImageBase64(b64 string) bool {
	// 16 字节内容需要 ceil(16/3)*4=24 个 base64 字符；不足则全部解码
	if len(b64) > 24 {
		b64 = b64[:24]
	}
	data, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return false
	}
	return sniffImageMagic(data)
}

// sniffImageMagic 按文件头魔数识别常见图片格式（PNG/JPEG/GIF/WebP/BMP）。
func sniffImageMagic(b []byte) bool {
	if len(b) >= 8 && string(b[:8]) == "\x89PNG\r\n\x1a\n" {
		return true
	}
	if len(b) >= 3 && b[0] == 0xff && b[1] == 0xd8 && b[2] == 0xff {
		return true
	}
	if len(b) >= 6 && (string(b[:6]) == "GIF87a" || string(b[:6]) == "GIF89a") {
		return true
	}
	if len(b) >= 12 && string(b[:4]) == "RIFF" && string(b[8:12]) == "WEBP" {
		return true
	}
	if len(b) >= 2 && b[0] == 'B' && b[1] == 'M' {
		return true
	}
	return false
}
