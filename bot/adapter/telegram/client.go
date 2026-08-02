package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	"golang.org/x/net/proxy"
)

// telegramClient Telegram Bot API 轻量客户端（resty 手写，不引入社区 SDK）。
// URL 约定：POST {apiBase}/bot{token}/{method}（token 放路径是官方经典方式）。
type telegramClient struct {
	http    *resty.Client
	apiBase string // 末尾无斜杠，如 https://api.telegram.org
	token   string
}

// telegramRateLimit 429 限流错误：携带 Telegram 建议的等待秒数。
type telegramRateLimit struct {
	retryAfter int
}

func (e *telegramRateLimit) Error() string {
	return fmt.Sprintf("telegram rate limited, retry after %ds", e.retryAfter)
}

// telegramAPIError 业务错误（ok=false）：携带 error_code 与描述，如 403 权限不足。
// 真实 Bot API 返回 ok=false 时 error_code 恒非零；code=0 说明响应体异常
// （网关/反代错误页、截断等，HTTP status 往往非 200），视为瞬时故障处理。
type telegramAPIError struct {
	status      int // HTTP 状态码（诊断用）
	code        int
	description string
}

func (e *telegramAPIError) Error() string {
	if e.status != 0 && e.status != http.StatusOK {
		return fmt.Sprintf("telegram api error %d (http %d): %s", e.code, e.status, e.description)
	}
	return fmt.Sprintf("telegram api error %d: %s", e.code, e.description)
}

// newTelegramClient 创建客户端。proxyURL 支持 http:// / https:// / socks5://，
// 空串直连；HTTP 代理经 resty SetProxy，socks5 需自建 DialContext（resty 不支持）。
func newTelegramClient(token, apiBase, proxyURL string, timeout time.Duration) *telegramClient {
	rc := resty.New().
		SetTimeout(timeout).
		SetRetryCount(0) // 429/网络错误由调用方控制重试，不依赖 resty 内置
	if proxyURL != "" {
		if strings.HasPrefix(proxyURL, "socks5://") || strings.HasPrefix(proxyURL, "socks5h://") {
			if u, err := url.Parse(proxyURL); err == nil {
				if d, err := proxy.FromURL(u, proxy.Direct); err == nil {
					rc = rc.SetTransport(&http.Transport{DialContext: d.(proxy.ContextDialer).DialContext})
				}
			}
		} else {
			rc = rc.SetProxy(proxyURL)
		}
	}
	return &telegramClient{http: rc, apiBase: strings.TrimSuffix(apiBase, "/"), token: token}
}

// call 调用一个 JSON 方法（getMe/getUpdates/sendMessage/editMessageText/getFile/getChat/getChatMember）。
// params 非 nil 时序列化为 JSON body；result 非 nil 时解包 ok.result 写入。
func (c *telegramClient) call(ctx context.Context, method string, params map[string]any, result any) error {
	req := c.http.R().SetContext(ctx).SetResult(&apiResponse{})
	if params != nil {
		req = req.SetBody(params)
	}
	resp, err := req.Post(c.url(method))
	if err != nil {
		return err
	}
	return c.unpack(method, resp, result)
}

// telegramUpload 上传文件的载体。Field 为 multipart 文件字段名
// （Telegram 各媒体方法要求方法专属字段名：sendPhoto→photo、sendDocument→document、
// sendVoice→voice、sendAudio→audio、sendVideo→video）。
type telegramUpload struct {
	Field    string
	FileName string
	Reader   io.Reader
}

// callMultipart 调用一个 multipart 上传方法（sendPhoto/sendDocument/sendVoice/sendAudio/sendVideo）。
// form 为普通表单字段（chat_id/caption/reply_parameters 等，字符串化）；upload 非 nil 时附加上传文件。
func (c *telegramClient) callMultipart(ctx context.Context, method string, form map[string]string, upload *telegramUpload, result any) error {
	req := c.http.R().SetContext(ctx).SetResult(&apiResponse{}).SetFormData(form)
	if upload != nil {
		req = req.SetFileReader(upload.Field, upload.FileName, upload.Reader)
	}
	resp, err := req.Post(c.url(method))
	if err != nil {
		return err
	}
	return c.unpack(method, resp, result)
}

// url 拼接方法 URL。
func (c *telegramClient) url(method string) string {
	return c.apiBase + "/bot" + c.token + "/" + method
}

// unpack 解包 apiResponse：ok=false 时区分 429（telegramRateLimit）与业务错误。
func (c *telegramClient) unpack(method string, resp *resty.Response, result any) error {
	ar, ok := resp.Result().(*apiResponse)
	if !ok || ar == nil {
		return errors.New("telegram: 无法解析 API 响应")
	}
	if !ar.OK {
		if ar.ErrorCode == 429 {
			retry := 1
			if ar.Parameters != nil && ar.Parameters.RetryAfter > 0 {
				retry = ar.Parameters.RetryAfter
			}
			return &telegramRateLimit{retryAfter: retry}
		}
		return &telegramAPIError{status: resp.StatusCode(), code: ar.ErrorCode, description: ar.Description}
	}
	if result == nil || len(ar.Result) == 0 {
		return nil
	}
	if err := json.Unmarshal(ar.Result, result); err != nil {
		return fmt.Errorf("telegram: 解析 %s 响应失败: %w", method, err)
	}
	return nil
}

// isBadRequest 判断是否为 Telegram 业务 400 错误（参数非法，如 MarkdownV2
// 解析失败），调用方可降级重试。网络错误/429 不在此列。
func isBadRequest(err error) bool {
	var ae *telegramAPIError
	return errors.As(err, &ae) && ae.code == 400
}

// isRetryableError 判断是否值得降级重试：MarkdownV2 解析失败（400）或
// 网关异常响应（ok:false 无错误码/不可解析，多为反代瞬时故障，HTTP 状态非 200）。
func isRetryableError(err error) bool {
	var ae *telegramAPIError
	if !errors.As(err, &ae) {
		return false
	}
	return ae.code == 400 || ae.code == 0
}

// retryAPIError 对一次发送/编辑调用按错误类型降级重试：
//  1. MarkdownV2 解析失败（400）→ 去掉 parse_mode 纯文本重发；
//  2. 网关异常响应（code 0）→ 视为瞬时故障原样重试一次（保留 parse_mode），
//     若重试后仍为 400 再走纯文本降级。
//
// 429 限流重试（retryOnce）由调用方负责，网络错误不重试。
func retryAPIError(ctx context.Context, params map[string]any, call func(map[string]any) error) error {
	err := call(params)
	if err == nil || !isRetryableError(err) {
		return err
	}
	var ae *telegramAPIError
	if errors.As(err, &ae) && ae.code == 0 {
		if err = call(params); err == nil {
			return nil
		}
	}
	if isBadRequest(err) {
		delete(params, "parse_mode")
		return call(params)
	}
	return err
}

// retryOnce 429 时按 retry-after 等待后重试一次（等待受 ctx 约束）。
func retryOnce[T any](ctx context.Context, fn func() (T, error)) (T, error) {
	v, err := fn()
	if err == nil {
		return v, nil
	}
	var rl *telegramRateLimit
	if !errors.As(err, &rl) {
		return v, err
	}
	wait := time.Duration(rl.retryAfter) * time.Second
	if wait > 30*time.Second {
		wait = 30 * time.Second
	}
	select {
	case <-time.After(wait):
	case <-ctx.Done():
		return v, ctx.Err()
	}
	return fn()
}
