package pluginaichat

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	"image/jpeg"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"golang.org/x/image/bmp"
)

func TestFetchImageDataURIPassthrough(t *testing.T) {
	got, err := fetchImageDataURI(context.Background(), testPNGDataURI)
	if err != nil {
		t.Fatalf("fetchImageDataURI err = %v", err)
	}
	if got != testPNGDataURI {
		t.Fatalf("data URI 应原样透传, got %q", got)
	}
}

func TestFetchImageDataURIDownloadPNG(t *testing.T) {
	raw := mustPNGBytes(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(raw)
	}))
	defer srv.Close()

	got, err := fetchImageDataURI(context.Background(), srv.URL+"/a.png")
	if err != nil {
		t.Fatalf("fetchImageDataURI err = %v", err)
	}
	if !strings.HasPrefix(got, "data:image/png;base64,") {
		t.Fatalf("应返回 png data URI, got prefix %q", got[:min(40, len(got))])
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(got, "data:image/png;base64,"))
	if err != nil {
		t.Fatalf("base64 解码失败: %v", err)
	}
	if !bytes.Equal(decoded, raw) {
		t.Fatal("下载的 PNG 应原样内联")
	}
}

// TestFetchImageDataURIDownloadBMPConvertsToPNG QQ 截图/表情常见 BMP 格式，
// 多模态模型不接受，应在本机转码为 PNG 后再内联。
func TestFetchImageDataURIDownloadBMPConvertsToPNG(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 2, 1))
	var buf bytes.Buffer
	if err := bmp.Encode(&buf, img); err != nil {
		t.Fatalf("bmp.Encode err = %v", err)
	}
	raw := buf.Bytes()
	if !bytes.Equal(raw[:2], []byte("BM")) {
		t.Fatalf("BMP 魔数异常: %x", raw[:2])
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/bmp")
		_, _ = w.Write(raw)
	}))
	defer srv.Close()

	got, err := fetchImageDataURI(context.Background(), srv.URL+"/a.bmp")
	if err != nil {
		t.Fatalf("fetchImageDataURI err = %v", err)
	}
	if !strings.HasPrefix(got, "data:image/png;base64,") {
		t.Fatalf("BMP 应转码为 png data URI, got prefix %q", got[:min(40, len(got))])
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(got, "data:image/png;base64,"))
	if err != nil {
		t.Fatalf("base64 解码失败: %v", err)
	}
	out, err := png.Decode(bytes.NewReader(decoded))
	if err != nil {
		t.Fatalf("转码结果无法解码为 PNG: %v", err)
	}
	if out.Bounds().Dx() != 2 || out.Bounds().Dy() != 1 {
		t.Fatalf("转码尺寸异常: %v", out.Bounds())
	}
}

func TestFetchImageDataURIDownloadFailure(t *testing.T) {
	// 200 但内容不是图片（如 QQ 临时链接过期返回的错误页）
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html>expired</html>"))
	}))
	defer srv.Close()

	_, err := fetchImageDataURI(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("非图片内容应报错")
	}
	if !strings.Contains(err.Error(), "无法识别的图片格式") {
		t.Fatalf("错误信息应说明格式无法识别, got %v", err)
	}

	// 404
	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv2.Close()
	if _, err := fetchImageDataURI(context.Background(), srv2.URL+"/missing.png"); err == nil {
		t.Fatal("404 应报错")
	}
}

// testProgressiveJPEG 8x8 渐进式 JPEG（SOF2 编码）。渐进式 JPEG 即使 MIME 标签
// 正确，DeepSeek 等部分模型服务解码也会报 "unsupported image" 400。
const testProgressiveJPEG = "/9j/4AAQSkZJRgABAQAAAQABAAD/2wBDAAUDBAQEAwUEBAQFBQUGBwwIBwcHBw8LCwkMEQ8SEhEPERETFhwXExQaFRERGCEYGh0dHx8fExciJCIeJBweHx7/2wBDAQUFBQcGBw4ICA4eFBEUHh4eHh4eHh4eHh4eHh4eHh4eHh4eHh4eHh4eHh4eHh4eHh4eHh4eHh4eHh4eHh4eHh7/wgARCAAIAAgDASIAAhEBAxEB/8QAFQABAQAAAAAAAAAAAAAAAAAAAAX/xAAVAQEBAAAAAAAAAAAAAAAAAAADBP/aAAwDAQACEAMQAAABnCVP/8QAFhAAAwAAAAAAAAAAAAAAAAAAAAQF/9oACAEBAAEFAkpZ/8QAGREAAQUAAAAAAAAAAAAAAAAABQABBBQh/9oACAEDAQE/AQ5KRWbV/8QAGBEAAgMAAAAAAAAAAAAAAAAAAAQDEjH/2gAIAQIBAT8BYemvp//EABYQAAMAAAAAAAAAAAAAAAAAAAABIv/aAAgBAQAGPwJSf//EABUQAQEAAAAAAAAAAAAAAAAAAADx/9oACAEBAAE/IZD/2gAMAwEAAgADAAAAEPf/xAAUEQEAAAAAAAAAAAAAAAAAAAAA/9oACAEDAQE/EG//xAAVEQEBAAAAAAAAAAAAAAAAAAAAEf/aAAgBAgEBPxCsf//EABQQAQAAAAAAAAAAAAAAAAAAAAD/2gAIAQEAAT8QA//Z"

// firstSOFFind 返回 JPEG 数据中第一个 SOF 标记字节（0xC0-0xCF，跳过 C4/C8/CC），
// 找不到返回 0。
func firstSOFFind(data []byte) byte {
	for i := 0; i+3 < len(data); {
		if data[i] != 0xFF || data[i+1] == 0xFF || data[i+1] == 0x00 {
			i++
			continue
		}
		marker := data[i+1]
		if marker == 0xC4 || marker == 0xC8 || marker == 0xCC || marker == 0xD8 || (marker >= 0xD0 && marker <= 0xD7) {
			// 非 SOF 段：D8 是 SOI，D0-D7 是 RST，均无长度字段
			i += 2
			continue
		}
		if marker >= 0xC0 && marker <= 0xCF {
			return marker
		}
		if i+4 > len(data) {
			return 0
		}
		segLen := int(data[i+2])<<8 | int(data[i+3])
		if segLen <= 0 {
			return 0
		}
		i += 2 + segLen
	}
	return 0
}

// TestImageDataURIProgressiveJPEGReencoded 渐进式 JPEG 应重编码为 baseline 后内联。
func TestImageDataURIProgressiveJPEGReencoded(t *testing.T) {
	raw, err := base64.StdEncoding.DecodeString(testProgressiveJPEG)
	if err != nil {
		t.Fatalf("测试样本 base64 解码失败: %v", err)
	}
	if got := firstSOFFind(raw); got != 0xC2 {
		t.Fatalf("测试样本应为渐进式 JPEG（SOF2）, got SOF marker %x", got)
	}

	uri, err := imageDataURI(raw)
	if err != nil {
		t.Fatalf("imageDataURI err = %v", err)
	}
	const prefix = "data:image/jpeg;base64,"
	if !strings.HasPrefix(uri, prefix) {
		t.Fatalf("应返回 jpeg data URI, got prefix %q", uri[:min(40, len(uri))])
	}
	out, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(uri, prefix))
	if err != nil {
		t.Fatalf("base64 解码失败: %v", err)
	}
	if got := firstSOFFind(out); got != 0xC0 {
		t.Fatalf("重编码后应为 baseline JPEG（SOF0）, got SOF marker %x", got)
	}
	if _, err := jpeg.Decode(bytes.NewReader(out)); err != nil {
		t.Fatalf("重编码结果无法解码: %v", err)
	}
}

// TestImageDataURIBrokenJPEGFails 带 JPEG 魔数但内容截断/损坏时，本机都无法解码，
// 原样转发会让模型服务整轮 400，应按单图加载失败返回错误。
func TestImageDataURIBrokenJPEGFails(t *testing.T) {
	broken := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00}
	_, err := imageDataURI(broken)
	if err == nil {
		t.Fatal("无法解码的 JPEG 应报错而不是原样内联")
	}
	if !strings.Contains(err.Error(), "JPEG 图片解码失败") {
		t.Fatalf("错误信息应说明 JPEG 解码失败, got %v", err)
	}
}

func mustPNGBytes(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("png.Encode err = %v", err)
	}
	return buf.Bytes()
}
