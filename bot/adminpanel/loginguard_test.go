package adminpanel

import (
	"net/http/httptest"
	"testing"
	"time"
)

// newTestGuard 构造参数更小的 guard，便于测试。
func newTestGuard(maxFails int, window, lockDur time.Duration) *loginGuard {
	return &loginGuard{
		attempts: map[string]*loginAttempt{},
		maxFails: maxFails,
		window:   window,
		lockDur:  lockDur,
	}
}

// TestLoginGuardLockout 连续失败达到阈值后锁定，锁定剩余时间有效。
func TestLoginGuardLockout(t *testing.T) {
	g := newTestGuard(3, time.Minute, 10*time.Minute)
	for i := 0; i < 2; i++ {
		lockedNow, _ := g.recordFail("1.2.3.4")
		if lockedNow {
			t.Fatalf("第 %d 次失败不应触发锁定", i+1)
		}
		if locked, _ := g.locked("1.2.3.4"); locked {
			t.Fatalf("第 %d 次失败后不应处于锁定", i+1)
		}
	}
	lockedNow, lockDur := g.recordFail("1.2.3.4")
	if !lockedNow || lockDur != 10*time.Minute {
		t.Fatalf("第 3 次失败应触发锁定: lockedNow=%v lockDur=%v", lockedNow, lockDur)
	}
	locked, remain := g.locked("1.2.3.4")
	if !locked || remain <= 0 || remain > 10*time.Minute {
		t.Fatalf("锁定状态异常: locked=%v remain=%v", locked, remain)
	}
}

// TestLoginGuardSuccessResets 登录成功后失败计数清零。
func TestLoginGuardSuccessResets(t *testing.T) {
	g := newTestGuard(3, time.Minute, 10*time.Minute)
	g.recordFail("1.2.3.4")
	g.recordFail("1.2.3.4")
	g.recordSuccess("1.2.3.4")
	// 重新计满 3 次才锁定
	for i := 0; i < 3; i++ {
		lockedNow, _ := g.recordFail("1.2.3.4")
		if i < 2 && lockedNow {
			t.Fatalf("成功清零后第 %d 次失败不应锁定", i+1)
		}
		if i == 2 && !lockedNow {
			t.Fatal("成功清零后重新计满 3 次应锁定")
		}
	}
}

// TestLoginGuardWindowExpiry 超出计数窗口后失败次数重新计。
func TestLoginGuardWindowExpiry(t *testing.T) {
	g := newTestGuard(3, 30*time.Millisecond, time.Minute)
	g.recordFail("1.2.3.4")
	g.recordFail("1.2.3.4")
	time.Sleep(50 * time.Millisecond)
	// 窗口已过，这一次失败应重新从 1 计，不触发锁定
	if lockedNow, _ := g.recordFail("1.2.3.4"); lockedNow {
		t.Fatal("计数窗口过期后不应累计旧失败次数")
	}
}

// TestLoginGuardLockExpiry 锁定期满后自动解锁。
func TestLoginGuardLockExpiry(t *testing.T) {
	g := newTestGuard(2, time.Minute, 30*time.Millisecond)
	g.recordFail("1.2.3.4")
	if lockedNow, _ := g.recordFail("1.2.3.4"); !lockedNow {
		t.Fatal("应触发锁定")
	}
	if locked, _ := g.locked("1.2.3.4"); !locked {
		t.Fatal("锁定期内 locked 应返回 true")
	}
	time.Sleep(50 * time.Millisecond)
	if locked, _ := g.locked("1.2.3.4"); locked {
		t.Fatal("锁定期满后应自动解锁")
	}
}

// TestLoginGuardSweep 超阈值时清理过期记录、保留锁定记录。
func TestLoginGuardSweep(t *testing.T) {
	g := newTestGuard(5, time.Minute, time.Minute)
	now := time.Now()
	for i := 0; i < loginGuardSweepThreshold+10; i++ {
		g.attempts[string(rune('a'+i%26))+string(rune(i))] = &loginAttempt{fails: 1, firstFailAt: now.Add(-2 * time.Minute)}
	}
	g.attempts["locked"] = &loginAttempt{lockedUntil: now.Add(time.Minute)}
	g.recordFail("trigger")
	if len(g.attempts) > 3 { // locked + trigger (+ 可能的边界残留)
		t.Fatalf("过期记录应被清理，剩 %d 条", len(g.attempts))
	}
	if _, ok := g.attempts["locked"]; !ok {
		t.Fatal("锁定中的记录不应被清理")
	}
}

// TestHashVerifyPassword 哈希-校验往返：正确密码通过，错误密码与篡改存储值拒绝。
func TestHashVerifyPassword(t *testing.T) {
	stored := hashPassword("s3cret")
	if !verifyPassword(stored, "s3cret") {
		t.Fatal("正确密码应通过校验")
	}
	if verifyPassword(stored, "s3cret!") {
		t.Fatal("错误密码不应通过校验")
	}
	if verifyPassword("not-a-valid-format", "x") {
		t.Fatal("非法格式不应通过校验")
	}
	if verifyPassword("zz:00", "x") {
		t.Fatal("非法 salt 不应通过校验")
	}
	// 哈希部分长度不足 sha256.Size
	if verifyPassword("00000000000000000000000000000000:00", "x") {
		t.Fatal("长度不符的哈希不应通过校验")
	}
	// 同一密码两次哈希结果应不同（随机盐）
	if hashPassword("s3cret") == stored {
		t.Fatal("相同密码的两次哈希应因随机盐而不同")
	}
}

// TestClientIP 直连为回环时才信任 X-Forwarded-For / X-Real-IP。
func TestClientIP(t *testing.T) {
	cases := []struct {
		name       string
		remoteAddr string
		xff        string
		xri        string
		want       string
	}{
		{"外网直连忽略XFF", "203.0.113.9:12345", "1.1.1.1", "", "203.0.113.9"},
		{"回环反代取XFF首个", "127.0.0.1:5678", "198.51.100.7, 10.0.0.1", "", "198.51.100.7"},
		{"回环反代取XRealIP", "127.0.0.1:5678", "", "198.51.100.8", "198.51.100.8"},
		{"IPv6回环信任XFF", "[::1]:5678", "198.51.100.9", "", "198.51.100.9"},
		{"回环无头部用RemoteAddr", "127.0.0.1:5678", "", "", "127.0.0.1"},
		{"无端口RemoteAddr", "203.0.113.10", "1.1.1.1", "", "203.0.113.10"},
		{"XFF首个为空则跳过", "127.0.0.1:5678", " , 10.0.0.1", "198.51.100.11", "198.51.100.11"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := httptest.NewRequest("POST", "/api/login", nil)
			r.RemoteAddr = c.remoteAddr
			if c.xff != "" {
				r.Header.Set("X-Forwarded-For", c.xff)
			}
			if c.xri != "" {
				r.Header.Set("X-Real-IP", c.xri)
			}
			if got := clientIP(r); got != c.want {
				t.Fatalf("clientIP = %q, want %q", got, c.want)
			}
		})
	}
}
