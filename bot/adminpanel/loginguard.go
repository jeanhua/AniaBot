package adminpanel

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	// loginMaxFails 计数窗口内连续失败达到该次数后锁定登录。
	loginMaxFails = 5
	// loginFailWindow 失败计数窗口：窗口内的失败才累计，超过窗口未再失败则清零重计。
	loginFailWindow = 10 * time.Minute
	// loginLockDuration 触发锁定后的锁定时长。
	loginLockDuration = 10 * time.Minute
	// loginFailDelay 登录失败响应前的固定延迟，拖慢在线爆破尝试。
	loginFailDelay = 500 * time.Millisecond

	// loginGuardSweepThreshold 记录数超过该阈值时顺手清理过期记录，防止内存无界增长。
	loginGuardSweepThreshold = 1024
)

// loginAttempt 单个来源的登录失败记录。
type loginAttempt struct {
	fails       int
	firstFailAt time.Time
	lockedUntil time.Time
}

// loginGuard 面板登录防爆破：按来源 IP 统计失败次数，超限后锁定一段时间。
// 纯内存实现，Bot 重启后计数清零（重启本身低频，可接受）。
type loginGuard struct {
	mu       sync.Mutex
	attempts map[string]*loginAttempt

	maxFails int
	window   time.Duration
	lockDur  time.Duration
}

func newLoginGuard() *loginGuard {
	return &loginGuard{
		attempts: map[string]*loginAttempt{},
		maxFails: loginMaxFails,
		window:   loginFailWindow,
		lockDur:  loginLockDuration,
	}
}

// locked 返回该来源当前是否处于锁定期，以及剩余锁定时间。
func (g *loginGuard) locked(key string) (bool, time.Duration) {
	g.mu.Lock()
	defer g.mu.Unlock()
	a, ok := g.attempts[key]
	if !ok {
		return false, 0
	}
	if d := time.Until(a.lockedUntil); d > 0 {
		return true, d
	}
	return false, 0
}

// recordFail 记录一次失败；若因此进入锁定，返回 lockedNow=true 与锁定时长。
func (g *loginGuard) recordFail(key string) (lockedNow bool, lockDur time.Duration) {
	now := time.Now()
	g.mu.Lock()
	defer g.mu.Unlock()
	if len(g.attempts) > loginGuardSweepThreshold {
		g.sweepLocked(now)
	}
	a, ok := g.attempts[key]
	if !ok || now.Sub(a.firstFailAt) > g.window {
		a = &loginAttempt{firstFailAt: now}
		g.attempts[key] = a
	}
	a.fails++
	if a.fails >= g.maxFails {
		a.lockedUntil = now.Add(g.lockDur)
		// 计数清零重计：锁定期满后再失败 maxFails 次才再次锁定
		a.fails = 0
		a.firstFailAt = now
		return true, g.lockDur
	}
	return false, 0
}

// recordSuccess 登录成功后清除该来源的失败记录。
func (g *loginGuard) recordSuccess(key string) {
	g.mu.Lock()
	delete(g.attempts, key)
	g.mu.Unlock()
}

// sweepLocked 清理已过期（未锁定且超出计数窗口）的记录，调用方需持锁。
func (g *loginGuard) sweepLocked(now time.Time) {
	for k, a := range g.attempts {
		if now.Before(a.lockedUntil) {
			continue
		}
		if now.Sub(a.firstFailAt) > g.window {
			delete(g.attempts, k)
		}
	}
}

// clientIP 提取客户端来源 IP。仅当直连对端为回环地址时（本机反代场景）才信任
// X-Forwarded-For / X-Real-IP，否则一律使用 RemoteAddr，
// 防止外部伪造头部变换身份绕过登录锁定。
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			if first, _, _ := strings.Cut(xff, ","); strings.TrimSpace(first) != "" {
				return strings.TrimSpace(first)
			}
		}
		if xri := strings.TrimSpace(r.Header.Get("X-Real-IP")); xri != "" {
			return xri
		}
	}
	return host
}
