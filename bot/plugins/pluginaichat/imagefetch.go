package pluginaichat

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	_ "image/gif"  // 注册 GIF 解码器（转码备用）
	_ "image/jpeg" // 注册 JPEG 解码器（转码备用）
	"image/png"    // PNG 解码注册 + 转码输出
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	_ "golang.org/x/image/bmp" // 注册 BMP 解码器：QQ 截图/表情常见 BMP，需转码后才被多模态模型接受
)

const (
	// imageFetchTimeout 单张图片下载超时。
	imageFetchTimeout = 30 * time.Second
	// imageFetchMaxBytes 图片大小上限：超过则拒绝加载，避免超大 base64 撑爆上下文。
	imageFetchMaxBytes = 25 << 20
	// imageFetchUserAgent 下载 QQ 等平台图片链接时使用的 UA，避免 CDN 因默认 UA 拒绝。
	imageFetchUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0 Safari/537.36"
)

// fetchImageDataURI 把图片引用统一为 data URI：
//   - data: URI 原样透传（本地图片等场景）；
//   - http(s) URL 在本机下载，按魔数识别真实格式，webp/png/jpeg/gif 直接内联；
//     其余格式（如 QQ 常见的 BMP）解码后转码为 PNG。
//
// 这样上游模型服务不再需要自己拉取 QQ 临时链接（rkey 过期、机房拉不到时
// 会报“不支持的图片”400 错误），且格式始终在模型服务支持范围内。
func fetchImageDataURI(ctx context.Context, ref string) (string, error) {
	if strings.HasPrefix(ref, "data:") {
		return ref, nil
	}
	client := resty.New().
		SetTimeout(imageFetchTimeout).
		SetHeader("User-Agent", imageFetchUserAgent)
	resp, err := client.R().SetContext(ctx).Get(ref)
	if err != nil {
		return "", fmt.Errorf("下载图片失败: %w", err)
	}
	if !resp.IsSuccess() {
		return "", fmt.Errorf("下载图片失败: HTTP %d", resp.StatusCode())
	}
	data := resp.Body()
	if len(data) == 0 {
		return "", fmt.Errorf("图片内容为空")
	}
	if len(data) > imageFetchMaxBytes {
		return "", fmt.Errorf("图片过大（%.1fMB，上限 %.1fMB）", float64(len(data))/(1<<20), float64(imageFetchMaxBytes)/(1<<20))
	}
	if mime, ok := sniffImageMIME(data); ok {
		// 模型服务支持的格式直接内联，保持原始字节
		return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data), nil
	}
	// 不支持直接内联的格式（如 BMP）：解码后转码为 PNG
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("无法识别的图片格式: %v", err)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", fmt.Errorf("图片转码失败: %w", err)
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

// sniffImageMIME 按文件魔数识别模型服务可直接接受的图片格式，返回 MIME 与是否识别成功。
func sniffImageMIME(data []byte) (string, bool) {
	switch {
	case len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF:
		return "image/jpeg", true
	case len(data) >= 8 && bytes.Equal(data[:8], []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}):
		return "image/png", true
	case len(data) >= 6 && (bytes.Equal(data[:6], []byte("GIF87a")) || bytes.Equal(data[:6], []byte("GIF89a"))):
		return "image/gif", true
	case len(data) >= 12 && bytes.Equal(data[:4], []byte("RIFF")) && bytes.Equal(data[8:12], []byte("WEBP")):
		return "image/webp", true
	default:
		return "", false
	}
}
