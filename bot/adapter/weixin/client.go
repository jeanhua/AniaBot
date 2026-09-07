package weixin

import (
	"context"
	"crypto/md5"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
)

// iLink Bot API 常量（参考 openclaw-weixin）。
const (
	// DefaultAPIBase 默认 API 地址（登录确认后服务端可能下发专属 baseurl）
	DefaultAPIBase = "https://ilinkai.weixin.qq.com"
	// DefaultCDNBase 默认 CDN 地址（媒体上传/下载）
	DefaultCDNBase = "https://novac2c.cdn.weixin.qq.com/c2c"
	// ILinkAppID 固定的 iLink 应用标识
	ILinkAppID = "bot"
	// DefaultBotType 登录二维码的 bot 类型
	DefaultBotType = "3"
	// StaleTokenErrcode 凭证失效错误码（需重新扫码登录）
	StaleTokenErrcode = -14
)

// clientVersion 适配器版本对应的 iLink-App-ClientVersion（0x00MMNNPP 编码）。
const clientVersion = 1<<16 | 0<<8 | 0 // 1.0.0

// BotAgent 出站自我声明（后台日志归因，仅观测用途）。
const BotAgent = "AniaBot/1.0"

// client iLink Bot API 轻量客户端（resty 手写，不引入社区 SDK）。
type client struct {
	http    *resty.Client
	apiBase string // 末尾无斜杠
	cdnBase string // 末尾无斜杠
	token   string
}

// newClient 创建客户端。token 为扫码登录获得的 bot token（Bearer）。
func newClient(token, apiBase, cdnBase string) *client {
	return &client{
		http:    resty.New().SetTimeout(40 * time.Second).SetRetryCount(0),
		apiBase: strings.TrimSuffix(apiBase, "/"),
		cdnBase: strings.TrimSuffix(cdnBase, "/"),
		token:   token,
	}
}

// baseInfo 公共请求元信息。
func baseInfo() BaseInfo { return BaseInfo{ChannelVersion: "1.0.0", BotAgent: BotAgent} }

// commonHeaders 匿名请求头（登录二维码等无 token 请求）。
func (c *client) commonHeaders() map[string]string {
	return map[string]string{
		"iLink-App-Id":            ILinkAppID,
		"iLink-App-ClientVersion": strconv.Itoa(clientVersion),
	}
}

// authHeaders 带 token 的请求头。X-WECHAT-UIN 为随机 uint32 十进制串的 base64
// （协议要求该头存在，服务端不校验其值与身份的关联）。
func (c *client) authHeaders() map[string]string {
	h := map[string]string{
		"Content-Type":      "application/json",
		"AuthorizationType": "ilink_bot_token",
	}
	var buf [4]byte
	_, _ = rand.Read(buf[:])
	uin := binary.BigEndian.Uint32(buf[:])
	h["X-WECHAT-UIN"] = base64.StdEncoding.EncodeToString([]byte(strconv.FormatUint(uint64(uin), 10)))
	for k, v := range c.commonHeaders() {
		h[k] = v
	}
	if t := strings.TrimSpace(c.token); t != "" {
		h["Authorization"] = "Bearer " + t
	}
	return h
}

// post JSON POST；HTTP 非 2xx 或响应体 ret/errcode 非零时返回错误。
// result 非 nil 时写入响应体。
func (c *client) post(ctx context.Context, endpoint string, body any, timeout time.Duration, result any) error {
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	resp, err := c.http.R().SetContext(ctx).SetHeaders(c.authHeaders()).SetBody(body).Post(c.apiBase + "/" + strings.TrimPrefix(endpoint, "/"))
	if err != nil {
		return err
	}
	return unpack(endpoint, resp, result)
}

// unpack 解析响应：HTTP 非 2xx 或业务 ret/errcode 非零均视为错误。
func unpack(endpoint string, resp *resty.Response, result any) error {
	if !resp.IsSuccess() {
		snippet := string(resp.Body())
		if len(snippet) > 200 {
			snippet = snippet[:200]
		}
		return fmt.Errorf("weixin %s: http %d: %s", endpoint, resp.StatusCode(), snippet)
	}
	if result != nil && len(resp.Body()) > 0 {
		if err := json.Unmarshal(resp.Body(), result); err != nil {
			return fmt.Errorf("weixin %s: 解析响应失败: %w", endpoint, err)
		}
	}
	// 业务错误码：ret/errcode 非零（-14 凭证失效由调用方识别处理）
	var ec struct {
		Ret     int    `json:"ret"`
		Errcode int    `json:"errcode"`
		Errmsg  string `json:"errmsg"`
	}
	if len(resp.Body()) > 0 && json.Unmarshal(resp.Body(), &ec) == nil {
		if ec.Ret != 0 {
			return &apiError{code: ec.Ret, msg: ec.Errmsg, endpoint: endpoint}
		}
		if ec.Errcode != 0 {
			return &apiError{code: ec.Errcode, msg: ec.Errmsg, endpoint: endpoint}
		}
	}
	return nil
}

// apiError iLink 业务错误（ret/errcode 非零）。
type apiError struct {
	code     int
	msg      string
	endpoint string
}

func (e *apiError) Error() string {
	return fmt.Sprintf("weixin %s: ret=%d errmsg=%s", e.endpoint, e.code, e.msg)
}

// StaleToken 判断是否凭证失效（errcode -14，需重新扫码）。
func StaleToken(err error) bool {
	var ae *apiError
	return errors.As(err, &ae) && ae.code == StaleTokenErrcode
}

// ---------- 匿名接口（扫码登录） ----------

// getBotQRCode 获取登录二维码。botType 为 ilink bot 类型（默认 "3"）。
func (c *client) getBotQRCode(ctx context.Context, botType string, localTokens []string) (*QRCodeResponse, error) {
	if botType == "" {
		botType = DefaultBotType
	}
	var res QRCodeResponse
	body := map[string]any{"local_token_list": localTokens}
	if err := c.post(ctx, "ilink/bot/get_bot_qrcode?bot_type="+botType, body, 15*time.Second, &res); err != nil {
		return nil, err
	}
	if res.Qrcode == "" {
		return nil, fmt.Errorf("weixin get_bot_qrcode: 响应缺少 qrcode")
	}
	return &res, nil
}

// getQRCodeStatus 长轮询二维码扫码状态（服务端最多挂起约 35s）。
func (c *client) getQRCodeStatus(ctx context.Context, qrcode, verifyCode string) (*QRCodeStatusResponse, error) {
	ep := "ilink/bot/get_qrcode_status?qrcode=" + queryEscape(qrcode)
	if verifyCode != "" {
		ep += "&verify_code=" + queryEscape(verifyCode)
	}
	var res QRCodeStatusResponse
	// 二维码状态接口带匿名头即可（openclaw 用 GET + 公共头，无 token）
	resp, err := c.http.R().SetContext(ctx).SetHeaders(c.commonHeaders()).Get(c.apiBase + "/" + ep)
	if err != nil {
		return nil, err
	}
	if err := unpack(ep, resp, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

// ---------- 已登录接口 ----------

// getUpdates 长轮询拉取新消息。getUpdatesBuf 为上次响应的游标（首次传空）。
func (c *client) getUpdates(ctx context.Context, getUpdatesBuf string, timeout time.Duration) (*GetUpdatesResp, error) {
	body := GetUpdatesReq{GetUpdatesBuf: getUpdatesBuf, BaseInfo: baseInfo()}
	var res GetUpdatesResp
	if err := c.post(ctx, "ilink/bot/getupdates", body, timeout, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

// sendMessage 发送一条消息。
func (c *client) sendMessage(ctx context.Context, msg *WeixinMessage) error {
	body := SendMessageReq{Msg: msg, BaseInfo: baseInfo()}
	var res SendMessageResp
	return c.post(ctx, "ilink/bot/sendmessage", body, 15*time.Second, &res)
}

// getUploadURL 获取 CDN 上传预签名。
func (c *client) getUploadURL(ctx context.Context, req *GetUploadUrlReq) (*GetUploadUrlResp, error) {
	req.BaseInfo = baseInfo()
	var res GetUploadUrlResp
	if err := c.post(ctx, "ilink/bot/getuploadurl", req, 15*time.Second, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

// notifyStart / notifyStop 上下线通知（失败仅记日志，不影响主流程）。
func (c *client) notifyStart(ctx context.Context) error {
	var res NotifyStartResp
	return c.post(ctx, "ilink/bot/msg/notifystart", map[string]any{"base_info": baseInfo()}, 10*time.Second, &res)
}

func (c *client) notifyStop(ctx context.Context) error {
	var res NotifyStopResp
	return c.post(ctx, "ilink/bot/msg/notifystop", map[string]any{"base_info": baseInfo()}, 10*time.Second, &res)
}

// queryEscape url.Values 轻量替代（仅登录接口使用，避免 net/url 导入噪音）。
func queryEscape(s string) string {
	const hexDigits = "0123456789ABCDEF"
	var sb strings.Builder
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') ||
			ch == '-' || ch == '_' || ch == '.' || ch == '~' {
			sb.WriteByte(ch)
		} else {
			sb.WriteByte('%')
			sb.WriteByte(hexDigits[ch>>4])
			sb.WriteByte(hexDigits[ch&0xF])
		}
	}
	return sb.String()
}

// download CDN 下载原始字节（不解密），20s 超时。
func (c *client) download(ctx context.Context, url string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	resp, err := c.http.R().SetContext(ctx).SetDoNotParseResponse(true).Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.RawBody().Close()
	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("weixin cdn download: http %d", resp.StatusCode())
	}
	return io.ReadAll(resp.RawBody())
}

// upload Ciphertext POST 到 CDN 上传地址，返回下载加密参数（x-encrypted-param 响应头）。
func (c *client) upload(ctx context.Context, url string, ciphertext []byte) (string, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(time.Second):
			}
		}
		ctx2, cancel := context.WithTimeout(ctx, 60*time.Second)
		resp, err := c.http.R().SetContext(ctx2).
			SetHeader("Content-Type", "application/octet-stream").
			SetBody(ciphertext).Post(url)
		cancel()
		if err != nil {
			lastErr = err
			continue
		}
		if resp.StatusCode() >= 400 && resp.StatusCode() < 500 {
			msg := resp.Header().Get("x-error-message")
			if msg == "" {
				msg = string(resp.Body())
			}
			return "", fmt.Errorf("weixin cdn upload: http %d: %s", resp.StatusCode(), msg)
		}
		if resp.StatusCode() != http.StatusOK {
			lastErr = fmt.Errorf("weixin cdn upload: http %d: %s", resp.StatusCode(), resp.Header().Get("x-error-message"))
			continue
		}
		param := resp.Header().Get("x-encrypted-param")
		if param == "" {
			lastErr = fmt.Errorf("weixin cdn upload: 响应缺少 x-encrypted-param 头")
			continue
		}
		return param, nil
	}
	return "", lastErr
}

// md5Hex 计算字节内容的 MD5 hex。
func md5Hex(b []byte) string {
	sum := md5.Sum(b)
	return hex.EncodeToString(sum[:])
}
