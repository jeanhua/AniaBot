package functool

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/jeanhua/AniaBot/bot/utils"
)

func startMockMemeServer() *httptest.Server {
	var serverURL string
	mux := http.NewServeMux()

	mux.HandleFunc("/v1/meme.php", func(w http.ResponseWriter, r *http.Request) {
		// parse num query (default to 3)
		num := 3
		if q := r.URL.Query().Get("num"); q != "" {
			// ignore parse errors, keep default on error
			if n, err := fmt.Sscanf(q, "%d", &num); err != nil || n != 1 {
				num = 3
			}
		}

		data := make([]map[string]string, 0, num)
		for i := 0; i < num; i++ {
			data = append(data, map[string]string{
				"img_url": fmt.Sprintf("%s/images/%d.jpg", serverURL, i),
			})
		}
		resp := map[string]interface{}{"data": data}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	// Serve image endpoints (respond to HEAD/GET)
	mux.HandleFunc("/images/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		// small dummy body for GET
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, "JPEGDATA")
			return
		}
		// HEAD: just OK
		w.WriteHeader(http.StatusOK)
	})

	server := httptest.NewServer(mux)
	serverURL = server.URL
	return server
}

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

	server := startMockMemeServer()
	defer server.Close()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			modifier, err := utils.NewURLModifier(server.URL + "/v1/meme.php")
			if err != nil {
				t.Fatalf("NewURLModifier error: %v", err)
			}
			modifier.SetQuery("msg", tt.text)
			modifier.SetQuery("num", "5")

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
	server := startMockMemeServer()
	defer server.Close()

	modifier, err := utils.NewURLModifier(server.URL + "/v1/meme.php")
	if err != nil {
		t.Fatalf("NewURLModifier error: %v", err)
	}
	modifier.SetQuery("msg", "开心")
	modifier.SetQuery("num", "4")

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
			// Try HEAD, fallback to GET
			resp, err := httpClient.Head(item.ImageUrl)
			if err != nil {
				t.Logf("  [%d] HEAD failed: %v, trying GET", i, err)
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
