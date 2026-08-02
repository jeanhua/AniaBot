package qqofficial

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-resty/resty/v2"
)

// TestResolveSegmentBytes 各段源的字节解析。
func TestResolveSegmentBytes(t *testing.T) {
	raw := []byte("hello")
	if b, ok := resolveSegmentBytes("base64://" + base64.StdEncoding.EncodeToString(raw)); !ok || string(b) != "hello" {
		t.Errorf("base64:// 解析失败: %q/%v", b, ok)
	}
	if b, ok := resolveSegmentBytes("data:image/png;base64," + base64.StdEncoding.EncodeToString(raw)); !ok || string(b) != "hello" {
		t.Errorf("data: 解析失败: %q/%v", b, ok)
	}
	dir := t.TempDir()
	fp := filepath.Join(dir, "a.bin")
	if err := os.WriteFile(fp, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	if b, ok := resolveSegmentBytes("file://" + fp); !ok || string(b) != "hello" {
		t.Errorf("file:// 解析失败: %q/%v", b, ok)
	}
	if _, ok := resolveSegmentBytes("https://e.com/a.png"); ok {
		t.Error("http URL 不在字节解析路径")
	}
	if _, ok := resolveSegmentBytes("base64://!!!bad"); ok {
		t.Error("非法 base64 应失败")
	}
}

// TestUploadByBytes 分片上传全流程：prepare → PUT 分片 → part_finish → 合并。
func TestUploadByBytes(t *testing.T) {
	data := []byte(strings.Repeat("x", 100))
	var putBodies [][]byte
	var partFinishCount int
	var mergeBody []byte

	putSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		putBodies = append(putBodies, b)
		w.WriteHeader(200)
	}))
	defer putSrv.Close()

	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/upload_prepare"):
			pre := uploadPrepareResponse{UploadID: "up-1", BlockSize: "50"}
			pre.Parts = []struct {
				Index        int    `json:"index"`
				PresignedURL string `json:"presigned_url"`
				BlockSize    string `json:"block_size"`
			}{
				{Index: 0, PresignedURL: putSrv.URL + "/p0", BlockSize: "50"},
				{Index: 1, PresignedURL: putSrv.URL + "/p1", BlockSize: "50"},
			}
			_ = json.NewEncoder(w).Encode(pre)
		case strings.HasSuffix(r.URL.Path, "/upload_part_finish"):
			partFinishCount++
			_, _ = w.Write([]byte(`{}`))
		case strings.HasSuffix(r.URL.Path, "/files"):
			mergeBody, _ = io.ReadAll(r.Body)
			_, _ = w.Write([]byte(`{"file_uuid":"u","file_info":"FI_CHUNK","ttl":300}`))
		default:
			w.WriteHeader(404)
		}
	}))
	defer apiSrv.Close()

	tm := newTokenManager("appid", "secret", resty.New())
	tm.token = "t"
	tm.expiresAt = time.Now().Add(time.Hour)
	a := NewAdapter(nil)
	a.client = newQQClient(apiSrv.URL, tm)

	fi, err := a.uploadByBytes(t.Context(), "/v2/groups/G", 1, data, "a.png")
	if err != nil {
		t.Fatalf("分片上传失败: %v", err)
	}
	if fi != "FI_CHUNK" {
		t.Fatalf("file_info = %q", fi)
	}
	if len(putBodies) != 2 || len(putBodies[0]) != 50 || len(putBodies[1]) != 50 {
		t.Fatalf("PUT 分片 = %v", putBodies)
	}
	if partFinishCount != 2 {
		t.Fatalf("part_finish 次数 = %d", partFinishCount)
	}
	var merge uploadFileRequest
	if err := json.Unmarshal(mergeBody, &merge); err != nil {
		t.Fatal(err)
	}
	if merge.UploadID != "up-1" || merge.FileType != 1 || merge.FileName != "a.png" {
		t.Fatalf("合并请求 = %+v", merge)
	}
}

// TestUploadMediaURL URL 源走直传（一次 /files 调用）。
func TestUploadMediaURL(t *testing.T) {
	var gotBody []byte
	a, srv := mockQQServer(t, func(method, path string, body []byte) (int, []byte) {
		gotBody = body
		return 200, []byte(`{"file_uuid":"u","file_info":"FI_URL","ttl":300}`)
	})
	defer srv.Close()

	fi, err := a.uploadMedia(t.Context(), "G", true, 1, "https://e.com/a.png", "")
	if err != nil || fi != "FI_URL" {
		t.Fatalf("URL 直传失败: %q/%v", fi, err)
	}
	var req uploadFileRequest
	if err := json.Unmarshal(gotBody, &req); err != nil {
		t.Fatal(err)
	}
	if req.URL != "https://e.com/a.png" || req.FileType != 1 || req.SrvSendMsg {
		t.Fatalf("直传请求 = %+v", req)
	}
}
