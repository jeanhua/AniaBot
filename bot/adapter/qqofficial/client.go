package qqofficial

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-resty/resty/v2"
)

// qqClient QQ 官方 OpenAPI 轻量客户端（resty 手写，不引入官方 SDK）。
// 鉴权：Authorization: QQBot {access_token}（token 由 tokenManager 自动刷新）。
type qqClient struct {
	http    *resty.Client
	apiBase string // 末尾无斜杠，如 https://api.sgroup.qq.com
	tokens  *tokenManager
}

// apiError OpenAPI 业务错误：响应体 err_code 非 0。
// 官方建议依据 err_code 判定成败（message 可能随时调整），故错误类型携带 code。
type apiError struct {
	status  int // HTTP 状态码（诊断用）
	code    int
	message string
	traceID string
}

func (e *apiError) Error() string {
	if e.traceID != "" {
		return fmt.Sprintf("qq official api error %d (http %d): %s (trace_id=%s)", e.code, e.status, e.message, e.traceID)
	}
	return fmt.Sprintf("qq official api error %d (http %d): %s", e.code, e.status, e.message)
}

// errCode 提取 OpenAPI err_code；非业务错误返回 ok=false。
func errCode(err error) (int, bool) {
	var ae *apiError
	if !errors.As(err, &ae) {
		return 0, false
	}
	return ae.code, true
}

func newQQClient(apiBase string, tokens *tokenManager) *qqClient {
	return &qqClient{
		http:    resty.New().SetTimeout(30_000_000_000), // 30s
		apiBase: strings.TrimSuffix(apiBase, "/"),
		tokens:  tokens,
	}
}

// post 调用一个 JSON POST 接口；result 非 nil 时把成功响应体解析进去。
// 401（token 失效）自动刷新 token 并重试一次。
func (c *qqClient) post(ctx context.Context, path string, body any, result any) error {
	err := c.doPost(ctx, path, body, result)
	if err != nil {
		var ae *apiError
		if errors.As(err, &ae) && ae.status == http.StatusUnauthorized {
			c.tokens.Invalidate()
			return c.doPost(ctx, path, body, result)
		}
	}
	return err
}

func (c *qqClient) doPost(ctx context.Context, path string, body any, result any) error {
	token, err := c.tokens.Token(ctx)
	if err != nil {
		return err
	}
	req := c.http.R().SetContext(ctx).
		SetHeader("Authorization", "QQBot "+token).
		SetHeader("Content-Type", "application/json; charset=utf-8")
	if body != nil {
		req = req.SetBody(body)
	}
	resp, err := req.Post(c.apiBase + path)
	if err != nil {
		return err
	}
	return unpackResponse(resp, result)
}

// getJSON 调用一个 JSON GET 接口（如 /gateway）。
func (c *qqClient) getJSON(ctx context.Context, path string, result any) error {
	token, err := c.tokens.Token(ctx)
	if err != nil {
		return err
	}
	resp, err := c.http.R().SetContext(ctx).
		SetHeader("Authorization", "QQBot "+token).
		Get(c.apiBase + path)
	if err != nil {
		return err
	}
	return unpackResponse(resp, result)
}

// unpackResponse 解析 OpenAPI 响应：err_code 非 0 视为业务错误；
// 响应体不含 err_code（成功）时解析进 result。
// 注意自行解析 resp.Body()（resty 只把 2xx body 填进 SetResult，同 telegram unpack 的教训）。
func unpackResponse(resp *resty.Response, result any) error {
	body := resp.Body()
	var eb apiErrorBody
	if len(body) > 0 {
		if err := json.Unmarshal(body, &eb); err != nil {
			// 响应体不可解析（网关错误页/截断）：保留 HTTP 状态码，错误码置 0
			desc := strings.TrimSpace(string(body))
			if len(desc) > 200 {
				desc = desc[:200]
			}
			return &apiError{status: resp.StatusCode(), code: 0, message: desc}
		}
	}
	if eb.ErrCode != 0 {
		return &apiError{status: resp.StatusCode(), code: eb.ErrCode, message: eb.Message, traceID: eb.TraceID}
	}
	if resp.StatusCode() == http.StatusUnauthorized {
		return &apiError{status: resp.StatusCode(), code: eb.ErrCode, message: eb.Message, traceID: eb.TraceID}
	}
	if resp.StatusCode() >= 400 {
		return &apiError{status: resp.StatusCode(), code: eb.ErrCode, message: firstNonEmpty(eb.Message, http.StatusText(resp.StatusCode())), traceID: eb.TraceID}
	}
	if result == nil || len(body) == 0 {
		return nil
	}
	if err := json.Unmarshal(body, result); err != nil {
		return fmt.Errorf("qq official: 解析响应失败: %w", err)
	}
	return nil
}

func firstNonEmpty(s, fallback string) string {
	if s != "" {
		return s
	}
	return fallback
}
