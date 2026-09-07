package weixin

import (
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// stubQRServer 模拟 iLink 登录端点：发二维码 → 状态序列 → confirmed。
type stubQRServer struct {
	*stubServer // 复用 httptest 包装（见下方）
}

func TestQRLoginSessionPanelHappyPath(t *testing.T) {
	var pollCount int32
	srv := startStub(t, func(path string, query string) (string, int) {
		switch {
		case strings.Contains(path, "get_bot_qrcode"):
			return `{"qrcode":"qr-1","qrcode_img_content":"https://login.example/qr-1"}`, 0
		case strings.Contains(path, "get_qrcode_status"):
			if strings.Contains(query, "verify_code=") {
				t.Errorf("配对码流程不应出现配对码: %s", query)
				return `{"status":"wait"}`, 0
			}
			if atomic.AddInt32(&pollCount, 1) == 1 {
				return `{"status":"wait"}`, 0
			}
			return `{"status":"confirmed","bot_token":"tok-2","ilink_bot_id":"b@im.bot","baseurl":"https://dedicated.example","ilink_user_id":"u@im.wechat"}`, 0
		}
		return `{}`, 0
	})

	a := newTestAdapter(t, srv.URL)
	qr, err := a.QRLoginStart()
	if err != nil {
		t.Fatalf("QRLoginStart: %v", err)
	}
	if !strings.HasPrefix(qr, "data:image/png;base64,") {
		t.Fatalf("qr data url = %q", qr)
	}

	deadline := time.Now().Add(5 * time.Second)
	for {
		state, detail, _ := a.QRLoginStatus()
		if state == LoginStateConnected {
			if !strings.Contains(detail, "b@im.bot") {
				t.Fatalf("detail = %q", detail)
			}
			break
		}
		if state == LoginStateFailed {
			t.Fatalf("login failed: %s", detail)
		}
		if time.Now().After(deadline) {
			t.Fatalf("login not confirmed in time, state=%s detail=%s", state, detail)
		}
		time.Sleep(20 * time.Millisecond)
	}

	// 凭据已落盘 + self 已更新
	st, err := a.state.load()
	if err != nil || st.Token != "tok-2" || st.BaseURL != "https://dedicated.example" {
		t.Fatalf("saved state = %+v err=%v", st, err)
	}
	if a.SelfID() != "wx:b@im.bot" {
		t.Fatalf("self = %q", a.SelfID())
	}
}

func TestQRLoginSessionVerifyCode(t *testing.T) {
	var gotVerify atomic.Value
	var pollCount int32
	srv := startStub(t, func(path string, query string) (string, int) {
		switch {
		case strings.Contains(path, "get_bot_qrcode"):
			return `{"qrcode":"qr-2","qrcode_img_content":"https://login.example/qr-2"}`, 0
		case strings.Contains(path, "get_qrcode_status"):
			if strings.Contains(query, "verify_code=9527") {
				gotVerify.Store("9527")
				return `{"status":"scaned"}`, 0
			}
			if atomic.AddInt32(&pollCount, 1) <= 2 {
				return `{"status":"need_verifycode"}`, 0
			}
			return `{"status":"confirmed","bot_token":"tok-3","ilink_bot_id":"b2@im.bot","ilink_user_id":"u@im.wechat"}`, 0
		}
		return `{}`, 0
	})

	a := newTestAdapter(t, srv.URL)
	if _, err := a.QRLoginStart(); err != nil {
		t.Fatalf("QRLoginStart: %v", err)
	}
	// 等待进入 need_verify 状态后提交配对码
	deadline := time.Now().Add(5 * time.Second)
	for {
		state, _, _ := a.QRLoginStatus()
		if state == LoginStateNeedVerify {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("state=%s, want need_verify", state)
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err := a.QRLoginSubmitVerify("9527"); err != nil {
		t.Fatalf("QRLoginSubmitVerify: %v", err)
	}
	// 无会话时提交报错
	if err := (&weixinAdapter{}).QRLoginSubmitVerify("1"); err == nil {
		t.Fatal("expected error without active session")
	}

	for {
		state, detail, _ := a.QRLoginStatus()
		if state == LoginStateConnected {
			break
		}
		if state == LoginStateFailed {
			t.Fatalf("login failed: %s", detail)
		}
		if time.Now().After(deadline) {
			t.Fatalf("login not confirmed in time, state=%s", state)
		}
		time.Sleep(20 * time.Millisecond)
	}
	if v, _ := gotVerify.Load().(string); v != "9527" {
		t.Fatalf("server did not receive verify code, got %v", v)
	}
}

func TestQRLoginStatusIdle(t *testing.T) {
	a := newTestAdapter(t, "https://unused.example")
	state, detail, qr := a.QRLoginStatus()
	if state != LoginStateIdle || detail == "" || qr != "" {
		t.Fatalf("idle status = %q %q %q", state, detail, qr)
	}
}
