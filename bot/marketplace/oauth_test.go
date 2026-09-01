package marketplace

import "testing"

func TestOAuthFlowSnapshot(t *testing.T) {
	f := newOAuthFlow()
	s := f.snapshot()
	if s["active"] != false || s["status"] != "" {
		t.Fatalf("初始快照异常: %+v", s)
	}

	f.mu.Lock()
	f.active = true
	f.deviceCode = "dc"
	f.userCode = "ABCD-EFGH"
	f.verificationURI = "https://github.com/login/device"
	f.expiresAt = f.expiresAt.Add(900)
	f.intervalSec = 5
	f.mu.Unlock()

	s = f.snapshot()
	if s["user_code"] != "ABCD-EFGH" || s["verification_uri"] != "https://github.com/login/device" || s["interval"] != 5 {
		t.Fatalf("pending 快照异常: %+v", s)
	}

	f.setDone("authorized", "jeanhua", "")
	s = f.snapshot()
	if s["active"] != false || s["status"] != "authorized" || s["user"] != "jeanhua" {
		t.Fatalf("authorized 快照异常: %+v", s)
	}
}
