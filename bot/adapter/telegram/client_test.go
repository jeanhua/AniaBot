package telegram

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/jeanhua/AniaBot/common/adapter"
	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/jeanhua/AniaBot/common/msgchain"
)

// fakeAPI 假 Bot API 服务器：记录全部请求（JSON body / multipart 表单与文件字节），
// 按方法返回预设响应。路由 /bot<token>/<method> 与 /file/bot<token>/<path>。
type fakeAPI struct {
	mu       sync.Mutex
	requests []recordedRequest
	updates  []Update       // getUpdates 返回（取走后清空，模拟一次一消费）
	flaky429 map[string]int // method -> 后续调用返回 429 的剩余次数
}

type recordedRequest struct {
	method    string
	json      map[string]any
	form      map[string]string
	fileField string // multipart 文件部分的字段名（photo/document/voice/audio/video）
	file      []byte
}

func newFakeAPI() *fakeAPI {
	return &fakeAPI{flaky429: map[string]int{}}
}

func (f *fakeAPI) reply(w http.ResponseWriter, result string) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"ok":true,"result":` + result + `}`))
}

func (f *fakeAPI) replyJSON(w http.ResponseWriter, v any) {
	b, _ := json.Marshal(v)
	f.reply(w, string(b))
}

func (f *fakeAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 文件下载端点
	if strings.HasPrefix(r.URL.Path, "/file/botTEST/") {
		_, _ = w.Write([]byte("IMAGEBYTES"))
		return
	}
	method := strings.TrimPrefix(r.URL.Path, "/botTEST/")
	rec := recordedRequest{method: method}
	if r.Method == http.MethodPost {
		ct := r.Header.Get("Content-Type")
		if strings.HasPrefix(ct, "application/json") {
			_ = json.NewDecoder(r.Body).Decode(&rec.json)
		} else if strings.HasPrefix(ct, "multipart/") {
			_ = r.ParseMultipartForm(8 << 20)
			rec.form = map[string]string{}
			for k, vs := range r.MultipartForm.Value {
				rec.form[k] = vs[0]
			}
			for name, fhs := range r.MultipartForm.File {
				if len(fhs) == 0 {
					continue
				}
				rec.fileField = name
				file, _ := fhs[0].Open()
				rec.file, _ = io.ReadAll(file)
				_ = file.Close()
				break
			}
		}
	}
	f.mu.Lock()
	f.requests = append(f.requests, rec)
	flaky := f.flaky429[method]
	if flaky > 0 {
		f.flaky429[method] = flaky - 1
		f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":false,"error_code":429,"description":"Too Many Requests","parameters":{"retry_after":1}}`))
		return
	}
	f.mu.Unlock()

	switch method {
	case "getMe":
		f.reply(w, `{"id":100,"is_bot":true,"first_name":"Ania","username":"mybot"}`)
	case "getUpdates":
		f.mu.Lock()
		ups := f.updates
		f.updates = nil
		f.mu.Unlock()
		f.replyJSON(w, ups)
	case "getFile":
		f.reply(w, `{"file_id":"f1","file_path":"photos/a.jpg"}`)
	case "sendMessage", "editMessageText", "sendPhoto", "sendDocument", "sendVoice", "sendVideo":
		f.reply(w, `{"message_id":42}`)
	default:
		http.NotFound(w, r)
	}
}

// req 返回第 i 个记录的请求。
func (f *fakeAPI) req(i int) recordedRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.requests[i]
}

// count 某方法的调用次数。
func (f *fakeAPI) count(method string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, r := range f.requests {
		if r.method == method {
			n++
		}
	}
	return n
}

// testAdapterWithServer 构造接入假服务器的适配器（self 预置，避免 getMe）。
func testAdapterWithServer(f *fakeAPI) (*telegramAdapter, *httptest.Server) {
	srv := httptest.NewServer(f)
	a := NewAdapter(nil)
	a.client = &telegramClient{http: resty.New(), apiBase: srv.URL, token: "TEST"}
	a.self = &User{ID: 100, Username: "mybot", FirstName: "Ania"}
	return a, srv
}

// TestClientCall JSON 方法调用：sendMessage 返回 message_id，参数透传。
func TestClientCall(t *testing.T) {
	f := newFakeAPI()
	a, srv := testAdapterWithServer(f)
	defer srv.Close()

	var res messageSendResult
	if err := a.client.call(t.Context(), "sendMessage", map[string]any{"chat_id": int64(-100), "text": "hi"}, &res); err != nil {
		t.Fatalf("sendMessage 失败: %v", err)
	}
	if res.MessageID != 42 {
		t.Fatalf("message_id = %d, want 42", res.MessageID)
	}
	rec := f.req(0)
	if rec.method != "sendMessage" {
		t.Fatalf("method = %s, want sendMessage", rec.method)
	}
	if rec.json["chat_id"] != float64(-100) || rec.json["text"] != "hi" {
		t.Fatalf("参数透传错误: %+v", rec.json)
	}
}

// TestClientMultipartUpload multipart 上传：表单字段与文件字节正确。
func TestClientMultipartUpload(t *testing.T) {
	f := newFakeAPI()
	a, srv := testAdapterWithServer(f)
	defer srv.Close()

	var res messageSendResult
	upload := &telegramUpload{Field: "photo", FileName: "photo.jpg", Reader: strings.NewReader("HELLOPNG")}
	if err := a.client.callMultipart(t.Context(), "sendPhoto",
		map[string]string{"chat_id": "-100", "caption": "看", "reply_parameters": `{"message_id":5}`},
		upload, &res); err != nil {
		t.Fatalf("sendPhoto 失败: %v", err)
	}
	rec := f.req(0)
	if rec.form["chat_id"] != "-100" || rec.form["caption"] != "看" || rec.form["reply_parameters"] != `{"message_id":5}` {
		t.Fatalf("表单字段错误: %+v", rec.form)
	}
	if rec.fileField != "photo" {
		t.Fatalf("multipart 文件字段名 = %q, want photo（方法专属字段名）", rec.fileField)
	}
	if string(rec.file) != "HELLOPNG" {
		t.Fatalf("上传字节 = %q, want HELLOPNG", rec.file)
	}
}

// TestClientRateLimitRetry 429 按 retry-after 重试一次后成功。
func TestClientRateLimitRetry(t *testing.T) {
	f := newFakeAPI()
	f.flaky429["sendMessage"] = 1
	a, srv := testAdapterWithServer(f)
	defer srv.Close()

	var res messageSendResult
	id, err := retryOnce(t.Context(), func() (int, error) {
		if err := a.client.call(t.Context(), "sendMessage", map[string]any{"chat_id": int64(-100), "text": "hi"}, &res); err != nil {
			return 0, err
		}
		return res.MessageID, nil
	})
	if err != nil {
		t.Fatalf("429 重试后仍失败: %v", err)
	}
	if id != 42 {
		t.Fatalf("id = %d, want 42（429 重试后成功）", id)
	}
	if n := f.count("sendMessage"); n != 2 {
		t.Fatalf("sendMessage 调用 = %d, want 2（首次 429 + 重试成功）", n)
	}
}

// TestDownloadResource getFile → 下载 → data URI。
func TestDownloadResource(t *testing.T) {
	f := newFakeAPI()
	a, srv := testAdapterWithServer(f)
	defer srv.Close()

	uri := a.downloadResource(t.Context(), "f1")
	if !strings.HasPrefix(uri, "data:image/png;base64,") {
		t.Fatalf("data URI 前缀错误: %q", uri[:40])
	}
	if got := strings.TrimPrefix(uri, "data:image/png;base64,"); got != "SU1BR0VCWVRFUw==" {
		t.Fatalf("解码字节错误: %q", got)
	}
	if uri := a.downloadResource(t.Context(), ""); uri != "" {
		t.Fatalf("空 fileID 应返回空串, got %q", uri)
	}
}

// TestSendChainIntegration 出站集成：text+image+text 序列、首条消息携带
// reply_parameters、短文本作 caption、base64 上传、@ 解析为 @username。
func TestSendChainIntegration(t *testing.T) {
	f := newFakeAPI()
	a, srv := testAdapterWithServer(f)
	defer srv.Close()
	a.chatMemberCache.Store("-100:222", mentionCache{username: "alice", at: time.Now()})

	chain := msgchain.Builder().
		Group().
		Reply(message.QID("tg:-100:5")).
		Mention(message.QID("tg:222")).
		Text("你好").
		ImageBase64("aGVsbG8="). // base64://aGVsbG8= → "hello"
		Text("尾部").
		Build()
	id, ok := a.SendGroupMsg(message.QID("tg:-100"), chain)
	if !ok {
		t.Fatal("发送失败")
	}
	if id.String() != "tg:-100:42" {
		t.Fatalf("返回 ID = %s, want tg:-100:42", id)
	}

	// 请求序列：sendPhoto（caption=@alice 你好 + reply）→ sendMessage（尾部）
	if f.count("sendPhoto") != 1 || f.count("sendMessage") != 1 {
		t.Fatalf("请求序列错误: %d sendPhoto / %d sendMessage", f.count("sendPhoto"), f.count("sendMessage"))
	}
	r0 := f.req(0)
	if r0.method != "sendPhoto" {
		t.Fatalf("首条应为 sendPhoto, got %s", r0.method)
	}
	if r0.form["caption"] != "@alice 你好" {
		t.Fatalf("caption = %q, want @alice 你好（@ 展开 + 短文本作 caption）", r0.form["caption"])
	}
	if r0.form["reply_parameters"] != `{"message_id":5}` {
		t.Fatalf("reply_parameters = %q（首条消息携带回复目标）", r0.form["reply_parameters"])
	}
	if string(r0.file) != "hello" {
		t.Fatalf("上传字节 = %q, want hello", r0.file)
	}
	r1 := f.req(1)
	if r1.method != "sendMessage" || r1.json["text"] != "尾部" {
		t.Fatalf("第二条应为 sendMessage(尾部), got %+v", r1)
	}
	if _, ok := r1.json["reply_parameters"]; ok {
		t.Fatal("后续消息不应再携带 reply_parameters")
	}
}

// TestSendChainLongText 长文本分包：5000 字符 → 2 条 sendMessage。
func TestSendChainLongText(t *testing.T) {
	f := newFakeAPI()
	a, srv := testAdapterWithServer(f)
	defer srv.Close()

	long := strings.Repeat("a", 5000)
	chain := msgchain.Builder().Group().Text(long).Build()
	if _, ok := a.SendGroupMsg(message.QID("tg:-100"), chain); !ok {
		t.Fatal("发送失败")
	}
	if n := f.count("sendMessage"); n != 2 {
		t.Fatalf("sendMessage 调用 = %d, want 2（分包）", n)
	}
}

// TestPollGetUpdates getUpdates 长轮询：offset/timeout 透传，返回更新数组。
func TestPollGetUpdates(t *testing.T) {
	f := newFakeAPI()
	f.updates = []Update{{UpdateID: 1, Message: textMsg(111, "private", 222, "hi")}, {UpdateID: 2}}
	a, srv := testAdapterWithServer(f)
	defer srv.Close()

	ups, err := a.getUpdates(t.Context(), 5, 30)
	if err != nil {
		t.Fatalf("getUpdates 失败: %v", err)
	}
	if len(ups) != 2 || ups[0].UpdateID != 1 {
		t.Fatalf("返回更新错误: %+v", ups)
	}
	rec := f.req(0)
	if rec.json["offset"] != float64(5) || rec.json["timeout"] != float64(30) {
		t.Fatalf("参数错误: %+v", rec.json)
	}
}

// TestProcessUpdatesDispatch 假服务器一批更新 → 分发一次、offset 推进。
func TestProcessUpdatesDispatch(t *testing.T) {
	f := newFakeAPI()
	f.updates = []Update{{UpdateID: 1, Message: textMsg(111, "private", 222, "hi")}}
	a, srv := testAdapterWithServer(f)
	defer srv.Close()

	delivered := make(chan struct{}, 4)
	a.SetTrigger(adapter.TriggerWrapper{
		OnFriendMsg: func(message.Message) { delivered <- struct{}{} },
	})
	// 完整轮询一拍：getUpdates → processUpdates（offset 0 → 2）
	ups, err := a.getUpdates(t.Context(), 0, 30)
	if err != nil {
		t.Fatalf("getUpdates 失败: %v", err)
	}
	offset := a.processUpdates(ups, 0)
	if offset != 2 {
		t.Fatalf("offset = %d, want 2", offset)
	}
	if !waitDeliver(t, delivered, 2*time.Second) {
		t.Fatal("消息应被分发")
	}
}
