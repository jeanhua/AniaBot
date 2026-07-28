package adminpanel

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/jeanhua/AniaBot/common/storage"
)

const (
	sessionCookieName = "ania_session"
	sessionTTL        = 24 * time.Hour

	// sessionRenewThreshold 剩余有效期不足该阈值时自动续期（滑动过期），
	// 活跃用户不会被踢下线，闲置满 sessionTTL 才过期。
	sessionRenewThreshold = 12 * time.Hour

	passwordHashKey  = "password_hash"
	sessionKeyPrefix = "session:"
	adminNamespace   = "__admin"

	maskPlaceholder = "********"
)

// authManager 管理面板密码与内存会话。
type authManager struct {
	store  storage.PersistentStorage
	logger *slog.Logger

	mu       sync.Mutex
	sessions map[string]time.Time // token -> 过期时间
}

func newAuthManager(root storage.PersistentStorage, logger *slog.Logger) *authManager {
	a := &authManager{
		store:    root.Clone(adminNamespace),
		logger:   logger,
		sessions: map[string]time.Time{},
	}
	a.ensureInitialPassword()
	return a
}

// ensureInitialPassword 首次启动时生成随机初始密码并打印到控制台。
func (a *authManager) ensureInitialPassword() {
	if a.store.Has(context.Background(), passwordHashKey) {
		return
	}
	password := randomPassword(10)
	if !a.SetPassword(password) {
		a.logger.Error("无法保存面板初始密码，请检查持久化存储")
		return
	}
	fmt.Println("============================================================")
	fmt.Println("  Web 控制面板初始密码（仅显示一次，登录后可修改）:")
	fmt.Printf("    %s\n", password)
	fmt.Println("============================================================")
	a.logger.Info("已生成 Web 控制面板初始密码（见上方控制台输出）")
}

func randomPassword(n int) string {
	const alphabet = "abcdefghjkmnpqrstuvwxyzABCDEFGHJKMNPQRSTUVWXYZ23456789"
	buf := make([]byte, n)
	for i := range buf {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
		if err != nil {
			buf[i] = alphabet[time.Now().UnixNano()%int64(len(alphabet))]
			continue
		}
		buf[i] = alphabet[num.Int64()]
	}
	return string(buf)
}

// hashPassword 返回 "salt:hash"（hex），salt 每次随机生成。
func hashPassword(password string) string {
	salt := make([]byte, 16)
	_, _ = rand.Read(salt)
	sum := sha256.Sum256(append(salt, []byte(password)...))
	return hex.EncodeToString(salt) + ":" + hex.EncodeToString(sum[:])
}

func verifyPassword(stored, password string) bool {
	parts := strings.SplitN(stored, ":", 2)
	if len(parts) != 2 {
		return false
	}
	salt, err := hex.DecodeString(parts[0])
	if err != nil {
		return false
	}
	sum := sha256.Sum256(append(salt, []byte(password)...))
	return hex.EncodeToString(sum[:]) == parts[1]
}

// SetPassword 更新密码（哈希落盘）。
func (a *authManager) SetPassword(password string) bool {
	return a.store.SetString(context.Background(), passwordHashKey, hashPassword(password))
}

// ResetPassword 直接覆盖面板密码哈希（供命令行找回密码使用，无需校验旧密码）。
// 同时吊销所有已签发的会话：与面板内改密（DropAllSessions）行为一致，
// 否则重置后旧会话 cookie 仍在 TTL 内有效，无法把可疑登录踢下线。
func ResetPassword(root storage.PersistentStorage, password string) bool {
	store := root.Clone(adminNamespace)
	ctx := context.Background()
	if !store.SetString(ctx, passwordHashKey, hashPassword(password)) {
		return false
	}
	if keys, err := store.Keys(ctx, sessionKeyPrefix); err == nil {
		for _, k := range keys {
			store.Del(ctx, k)
		}
	}
	return true
}

// CheckPassword 校验密码。
func (a *authManager) CheckPassword(password string) bool {
	stored, ok := a.store.GetString(context.Background(), passwordHashKey)
	if !ok {
		return false
	}
	return verifyPassword(stored, password)
}

// NewSession 签发新会话，返回 token。会话持久化到存储中，Bot 重启后仍然有效。
func (a *authManager) NewSession() string {
	buf := make([]byte, 32)
	_, _ = rand.Read(buf)
	token := hex.EncodeToString(buf)
	exp := time.Now().Add(sessionTTL)
	a.mu.Lock()
	a.sessions[token] = exp
	a.mu.Unlock()
	a.store.SetString(context.Background(), sessionKeyPrefix+token, exp.Format(time.RFC3339Nano))
	a.cleanupExpired()
	return token
}

// ValidSession 校验 token 是否有效（内存优先，未命中回查持久化存储）。
// 校验通过且剩余有效期不足 sessionRenewThreshold 时自动续期至 sessionTTL，
// 并返回 renewed=true，调用方应同步刷新客户端 Cookie。
func (a *authManager) ValidSession(token string) (valid, renewed bool) {
	now := time.Now()
	a.mu.Lock()
	exp, ok := a.sessions[token]
	a.mu.Unlock()
	if ok {
		if !now.Before(exp) {
			return false, false
		}
		if exp.Sub(now) < sessionRenewThreshold {
			a.renewSession(token)
			return true, true
		}
		return true, false
	}
	// 内存未命中（如 Bot 重启后）：回查持久化存储
	raw, found := a.store.GetString(context.Background(), sessionKeyPrefix+token)
	if !found {
		return false, false
	}
	expTime, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil || now.After(expTime) {
		a.store.Del(context.Background(), sessionKeyPrefix+token)
		return false, false
	}
	if expTime.Sub(now) < sessionRenewThreshold {
		a.renewSession(token)
		return true, true
	}
	a.mu.Lock()
	a.sessions[token] = expTime
	a.mu.Unlock()
	return true, false
}

// renewSession 将内存与持久化存储中的会话过期时间顺延至 now+sessionTTL。
func (a *authManager) renewSession(token string) {
	exp := time.Now().Add(sessionTTL)
	a.mu.Lock()
	a.sessions[token] = exp
	a.mu.Unlock()
	a.store.SetString(context.Background(), sessionKeyPrefix+token, exp.Format(time.RFC3339Nano))
}

// DropSession 销毁会话。
func (a *authManager) DropSession(token string) {
	a.mu.Lock()
	delete(a.sessions, token)
	a.mu.Unlock()
	a.store.Del(context.Background(), sessionKeyPrefix+token)
}

// DropAllSessions 销毁所有会话（修改密码后调用，强制各端重新登录）。
func (a *authManager) DropAllSessions() {
	a.mu.Lock()
	a.sessions = map[string]time.Time{}
	a.mu.Unlock()
	keys, err := a.store.Keys(context.Background(), sessionKeyPrefix)
	if err != nil {
		a.logger.Error("清理会话失败", "error", err)
		return
	}
	for _, k := range keys {
		a.store.Del(context.Background(), k)
	}
}

// cleanupExpired 清理内存与存储中过期的会话。
func (a *authManager) cleanupExpired() {
	now := time.Now()
	a.mu.Lock()
	for t, exp := range a.sessions {
		if now.After(exp) {
			delete(a.sessions, t)
		}
	}
	a.mu.Unlock()
	keys, err := a.store.Keys(context.Background(), sessionKeyPrefix)
	if err != nil {
		return
	}
	for _, k := range keys {
		raw, ok := a.store.GetString(context.Background(), k)
		if !ok {
			continue
		}
		exp, err := time.Parse(time.RFC3339Nano, raw)
		if err != nil || now.After(exp) {
			a.store.Del(context.Background(), k)
		}
	}
}
