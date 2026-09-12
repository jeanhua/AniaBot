package message

import (
	"encoding/base64"
	"path/filepath"
	"strings"
)

// FileSegmentAsImage 判断 file 段指向的是否为图片文件：是则返回等价 image 段的数据
// （file 源保持不变，url/summary 与 msgchain 的 ImageBase64/ImageUrl 一致），使拥有
// 独立图片发送接口的平台适配器在出站时走图片通道（聊天内联展示），而不是文件通道
// （附件）。仅当存在 file 源（路径/URL/base64）时才能转换，只有 file_id 的段无法
// 作为图片发送。
func FileSegmentAsImage(seg OB11Segment) (map[string]any, bool) {
	if seg.Type != SegmentFile || seg.Data == nil {
		return nil, false
	}
	file, _ := seg.Data["file"].(string)
	if file == "" {
		return nil, false
	}
	name, _ := seg.Data["name"].(string)
	if !isImageSource(file, name) {
		return nil, false
	}
	img := map[string]any{
		"file":    file,
		"summary": "[图片]",
	}
	if url, ok := seg.Data["url"].(string); ok && url != "" {
		img["url"] = url
	} else {
		img["url"] = file
	}
	return img, true
}

// 常见图片扩展名（各平台聊天内联展示的位图格式）。
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
