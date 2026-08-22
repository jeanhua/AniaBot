package pluginaichat

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
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

func mustPNGBytes(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("png.Encode err = %v", err)
	}
	return buf.Bytes()
}
