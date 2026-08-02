package qqofficial

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
)

// tokenURL getAppAccessToken 固定域名（沙箱与正式环境共用）。
const tokenURL = "https://bots.qq.com/app/getAppAccessToken"

// tokenRefreshMargin 提前刷新的余量：官方约定「到期前 60 秒内请求会签发新 token，
// 旧 token 在这 60 秒内仍可用」，故在到期前 5 分钟即刷新，保证平滑切换。
const tokenRefreshMargin = 5 * time.Minute

// tokenManager access_token 管理器：lazy 获取 + 到期自动刷新。
// access_token 生命周期约 7200 秒，有效期内重复获取返回相同值；
// 凭据错误（100016 等）直接返回错误，不做无限重试。
type tokenManager struct {
	appID  string
	secret string
	http   *resty.Client
	url    string // getAppAccessToken 地址（默认 tokenURL，测试可注入）

	mu        sync.Mutex
	token     string
	expiresAt time.Time
}

func newTokenManager(appID, secret string, http *resty.Client) *tokenManager {
	return &tokenManager{appID: appID, secret: secret, http: http, url: tokenURL}
}

// Token 返回当前可用的 access_token，临近过期或失效时自动刷新。
func (t *tokenManager) Token(ctx context.Context) (string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.token != "" && time.Until(t.expiresAt) > tokenRefreshMargin {
		return t.token, nil
	}
	return t.refreshLocked(ctx)
}

// Invalidate 使缓存的 token 失效（如收到 401 后），下次 Token() 强制刷新。
func (t *tokenManager) Invalidate() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.expiresAt = time.Time{}
}

// refreshLocked 调用 getAppAccessToken 刷新凭证（调用方需持有锁）。
func (t *tokenManager) refreshLocked(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	body := map[string]string{"appId": t.appID, "clientSecret": t.secret}
	resp, err := t.http.R().SetContext(ctx).SetBody(body).Post(t.url)
	if err != nil {
		return "", fmt.Errorf("请求 getAppAccessToken 失败: %w", err)
	}
	var tr tokenResponse
	if err := json.Unmarshal(resp.Body(), &tr); err != nil {
		return "", fmt.Errorf("解析 getAppAccessToken 响应失败: %w", err)
	}
	if tr.AccessToken == "" {
		// 失败响应体为 {"err_code":100016,"message":"invalid appid or secret",...} 形态
		var eb apiErrorBody
		_ = json.Unmarshal(resp.Body(), &eb)
		if eb.ErrCode != 0 || eb.Message != "" {
			return "", fmt.Errorf("获取 access_token 失败 (err_code=%d): %s", eb.ErrCode, eb.Message)
		}
		return "", fmt.Errorf("获取 access_token 失败：响应缺少 access_token 字段 (http %d)", resp.StatusCode())
	}
	expiresIn := parseExpiresIn(tr.ExpiresIn)
	if expiresIn <= 0 {
		expiresIn = 7200
	}
	t.token = tr.AccessToken
	t.expiresAt = time.Now().Add(time.Duration(expiresIn) * time.Second)
	return t.token, nil
}

// parseExpiresIn 解析 expires_in：官方文档字段描述为 number，调用示例为字符串 "7200"。
func parseExpiresIn(v any) int {
	switch e := v.(type) {
	case float64:
		return int(e)
	case string:
		if n, err := strconv.Atoi(e); err == nil {
			return n
		}
	case json.Number:
		if n, err := e.Int64(); err == nil {
			return int(n)
		}
	}
	return 0
}
