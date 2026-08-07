package functool

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jeanhua/AniaBot/bot/component/llmtool"
)

// startMockMemeServer 返回 suol 风格响应（{"data":[{"img_url":...}]}）的模拟接口，
// listPath/imgField 可据此验证 gjson 提取；返回空数组时用于测试空结果分支
func startMockMemeServer(t *testing.T, giphyStyle bool, empty bool) *httptest.Server {
	t.Helper()
	var serverURL string
	mux := http.NewServeMux()

	mux.HandleFunc("/meme", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if empty {
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
			return
		}
		num := 3
		if q := r.URL.Query().Get("num"); q != "" {
			if n, err := fmt.Sscanf(q, "%d", &num); err != nil || n != 1 {
				num = 3
			}
		}
		data := make([]map[string]any, 0, num)
		for i := 0; i < num; i++ {
			imgURL := fmt.Sprintf("%s/images/%d.gif", serverURL, i)
			if giphyStyle {
				data = append(data, map[string]any{
					"images": map[string]any{
						"fixed_width": map[string]any{"url": imgURL},
					},
				})
			} else {
				data = append(data, map[string]any{"img_url": imgURL})
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
	})

	mux.HandleFunc("/images/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/gif")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("GIFDATA"))
	})

	server := httptest.NewServer(mux)
	serverURL = server.URL
	t.Cleanup(server.Close)
	return server
}

// captureSendImage 返回拦截 SendImage 的回调集合与取回 base64 内容的函数
func captureSendImage(sent *string) llmtool.CallBackFuncs {
	return llmtool.CallBackFuncs{
		SendImage: func(bs64 string) (string, error) {
			*sent = bs64
			return "ok", nil
		},
	}
}

func TestMemeExecuteSuolStyle(t *testing.T) {
	server := startMockMemeServer(t, false, false)
	tool := NewMemeTool(MemeConfig{
		URL:      server.URL + "/meme?msg=${msg}&num=${num}",
		ListPath: "data",
		ImgField: "img_url",
		Num:      5,
	})

	var sent string
	msg, err := tool.Execute(context.Background(), &MemeParams{Text: "开心"}, captureSendImage(&sent))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if !strings.Contains(msg, "开心") {
		t.Errorf("unexpected result message: %s", msg)
	}
	if got, _ := base64.StdEncoding.DecodeString(sent); string(got) != "GIFDATA" {
		t.Errorf("SendImage got unexpected content: %q", string(got))
	}
}

func TestMemeExecuteGiphyStyle(t *testing.T) {
	server := startMockMemeServer(t, true, false)
	tool := NewMemeTool(MemeConfig{
		URL:      server.URL + "/meme?api_key=${key}&q=${msg}&limit=${num}",
		Key:      "test-key",
		ListPath: "data",
		ImgField: "images.fixed_width.url",
		Num:      3,
	})

	var sent string
	if _, err := tool.Execute(context.Background(), &MemeParams{Text: "happy"}, captureSendImage(&sent)); err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if sent == "" {
		t.Error("expected SendImage to be called")
	}
}

func TestMemeBuildURL(t *testing.T) {
	cfg := MemeConfig{
		URL: "https://example.com/api?key=${key}&msg=${msg}&num=${num}",
		Key: "k&1",
		Num: 7,
	}
	cfg.fillDefaults()
	got := cfg.buildURL("开 心")
	want := "https://example.com/api?key=k%261&msg=%E5%BC%80+%E5%BF%83&num=7"
	if got != want {
		t.Errorf("buildURL = %q, want %q", got, want)
	}
}

func TestMemeFillDefaults(t *testing.T) {
	cfg := MemeConfig{}
	cfg.fillDefaults()
	if cfg.URL != DefaultMemeURL || cfg.ListPath != DefaultMemeListPath ||
		cfg.ImgField != DefaultMemeImgField || cfg.Num != DefaultMemeNum {
		t.Errorf("fillDefaults got %+v", cfg)
	}
}

func TestMemeMissingKey(t *testing.T) {
	tool := NewMemeTool(MemeConfig{}) // 默认模板含 ${key} 但未配置 Key
	var sent string
	_, err := tool.Execute(context.Background(), &MemeParams{Text: "开心"}, captureSendImage(&sent))
	if err == nil || !strings.Contains(err.Error(), "API Key") {
		t.Errorf("expected missing-key error, got %v", err)
	}
}

func TestMemeEmptyResult(t *testing.T) {
	server := startMockMemeServer(t, false, true)
	tool := NewMemeTool(MemeConfig{
		URL:      server.URL + "/meme?msg=${msg}&num=${num}",
		ListPath: "data",
		ImgField: "img_url",
		Num:      5,
	})
	var sent string
	_, err := tool.Execute(context.Background(), &MemeParams{Text: "不存在的东西"}, captureSendImage(&sent))
	if err == nil || !strings.Contains(err.Error(), "未找到相关表情包") {
		t.Errorf("expected empty-result error, got %v", err)
	}
}
