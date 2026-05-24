package functool

import (
	"log"
	"net/http"
	"testing"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/jeanhua/AniaBot/bot/utils"
)

func TestMemeAPI(t *testing.T) {
	tests := []struct {
		name string
		text string
	}{
		{"happy", "开心"},
		{"angry", "生气"},
		{"sad", "难过"},
		{"why", "为什么"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			modifier, err := utils.NewURLModifier("https://api.suol.cc/v1/meme.php")
			if err != nil {
				t.Fatalf("NewURLModifier error: %v", err)
			}
			modifier.SetQuery("msg", tt.text)
			modifier.SetQuery("num", "10")

			type responseTy struct {
				Data []struct {
					ImageUrl string `json:"img_url"`
				} `json:"data"`
			}
			result := responseTy{}
			client := resty.New()
			resp, err := client.R().SetResult(&result).Get(modifier.String())
			if err != nil {
				t.Fatalf("request error: %v", err)
			}

			log.Printf("text=%s status=%d data_count=%d", tt.text, resp.StatusCode(), len(result.Data))

			if resp.StatusCode() != 200 {
				t.Fatalf("expected status 200, got %d", resp.StatusCode())
			}

			if len(result.Data) == 0 {
				t.Fatal("expected non-empty data array")
			}

			for i, item := range result.Data {
				if item.ImageUrl == "" {
					t.Errorf("data[%d].ImageUrl is empty", i)
				}
				log.Printf("  [%d] url=%s", i, item.ImageUrl)
			}
		})
	}
}

func TestMemeImageURLAccessibility(t *testing.T) {
	modifier, err := utils.NewURLModifier("https://api.suol.cc/v1/meme.php")
	if err != nil {
		t.Fatalf("NewURLModifier error: %v", err)
	}
	modifier.SetQuery("msg", "开心")
	modifier.SetQuery("num", "10")

	type responseTy struct {
		Data []struct {
			ImageUrl string `json:"img_url"`
		} `json:"data"`
	}
	result := responseTy{}
	client := resty.New()
	_, err = client.R().SetResult(&result).Get(modifier.String())
	if err != nil {
		t.Fatalf("request error: %v", err)
	}

	if len(result.Data) == 0 {
		t.Fatal("no data returned")
	}

	httpClient := &http.Client{Timeout: 10 * time.Second}
	for i, item := range result.Data {
		t.Run(item.ImageUrl, func(t *testing.T) {
			// 先尝试HEAD请求
			resp, err := httpClient.Head(item.ImageUrl)
			if err != nil {
				t.Logf("  [%d] HEAD failed: %v, trying GET", i, err)
				// HEAD失败则尝试GET
				resp, err = httpClient.Get(item.ImageUrl)
				if err != nil {
					t.Errorf("GET request also failed for url[%d]: %v", i, err)
					return
				}
			}
			defer resp.Body.Close()
			log.Printf("  [%d] status=%d content-type=%s url=%s", i, resp.StatusCode, resp.Header.Get("Content-Type"), item.ImageUrl)
			if resp.StatusCode != http.StatusOK {
				t.Errorf("expected status 200, got %d for url[%d]", resp.StatusCode, i)
			}
		})
	}
}
