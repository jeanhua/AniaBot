// balance.go —— API 余额查询。
//
// 面板概览页展示 AI API 余额。余额接口各厂商差异较大，但绝大多数都是
// 「带鉴权头请求一个地址，从 JSON 响应里取一个字段」，因此请求方式由面板
// 配置项以声明式描述（bot.balance.*）：地址/请求头/请求体支持
// ${base_url} ${api_key} ${model} 占位符（取自 AI 对话插件的 API 配置），
// 显示模板中的 {gjson 路径} 会被替换为响应 JSON 中对应字段的值。
// 结果按 bot.balance.cache_sec 缓存，避免面板轮询频繁打余额接口。
// 配置即时读取（configstore 直查 DB），修改无需重启。
package adminpanel

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/tidwall/gjson"
)

// 余额查询的默认配置（适配 DeepSeek 风格 /user/balance 接口，
// 也是 bot.balance.url / headers / format 配置项的默认值；core 包注册配置字段时引用）。
const (
	DefaultBalanceURL     = "${base_url}/user/balance"
	DefaultBalanceHeaders = `{"Authorization":"Bearer ${api_key}"}`
	DefaultBalanceFormat  = `¥ {data.balances.0.total_balance}`
)

// balanceHTTPTimeout 单次余额请求的 HTTP 超时
const balanceHTTPTimeout = 15 * time.Second

// balanceMaxBody 余额接口响应体大小上限
const balanceMaxBody = 4 << 20 // 4 MiB

// balanceFormatFieldRe 匹配显示模板中的 {gjson 路径} 占位符
var balanceFormatFieldRe = regexp.MustCompile(`\{([^{}]+)\}`)

// BalanceResult 余额查询结果（GET /api/balance 的响应体）。
type BalanceResult struct {
	Enabled   bool   `json:"enabled"`         // 是否启用了余额查询
	Value     string `json:"value"`           // 余额显示文本
	Error     string `json:"error,omitempty"` // 查询失败原因
	UpdatedAt string `json:"updated_at"`      // 上次实际发起查询的时间（RFC3339）
	Cached    bool   `json:"cached"`          // 本次返回是否为缓存结果
	TTL       int    `json:"ttl"`             // 缓存时长（秒）
}

// balanceCache 面板级余额结果缓存（挂在 Server 上）。
type balanceCache struct {
	mu      sync.Mutex
	result  BalanceResult
	fetched time.Time
}

func (s *Server) balanceTTL() time.Duration {
	if v, ok := s.opt.Config.Get("bot.balance.cache_sec"); ok {
		if sec, ok2 := toInt(v); ok2 && sec > 0 {
			return time.Duration(sec) * time.Second
		}
	}
	return 5 * time.Minute
}

// handleBalance 查询 AI API 余额（query 参数 refresh=1 强制刷新缓存）。
func (s *Server) handleBalance(w http.ResponseWriter, r *http.Request) {
	force := r.URL.Query().Get("refresh") == "1"
	writeJSON(w, http.StatusOK, s.queryBalance(force))
}

// queryBalance 读配置并执行余额查询，带缓存；请求错误不返回 HTTP 错误，
// 而是放进结果体的 error 字段由前端展示。
func (s *Server) queryBalance(force bool) BalanceResult {
	enabled, _ := s.opt.Config.Get("bot.balance.enable")
	if on, ok := enabled.(bool); !ok || !on {
		return BalanceResult{Enabled: false}
	}
	ttl := s.balanceTTL()

	s.balance.mu.Lock()
	defer s.balance.mu.Unlock()
	cached := s.balance.result
	if !force && !s.balance.fetched.IsZero() && time.Since(s.balance.fetched) < ttl {
		cached.Cached = true
		return cached
	}

	res := BalanceResult{Enabled: true, UpdatedAt: time.Now().Format(time.RFC3339), TTL: int(ttl.Seconds())}
	value, err := s.runBalanceQuery()
	if err != nil {
		s.opt.Logger.Warn("API 余额查询失败", "error", err)
		res.Error = err.Error()
		// 查询失败时保留上一份成功值，避免面板闪烁
		if cached.Value != "" {
			res.Value = cached.Value
		}
	} else {
		res.Value = value
	}
	s.balance.result = res
	s.balance.fetched = time.Now()
	return res
}

// runBalanceQuery 按 bot.balance.* 声明式配置发起 HTTP 请求并渲染余额显示文本。
func (s *Server) runBalanceQuery() (string, error) {
	// 占位符取自 AI 对话插件的 API 配置
	ph := strings.NewReplacer(
		"${base_url}", strings.TrimRight(configString(s.opt.Config, "plugin.ai_chat_bot.base_url"), "/"),
		"${api_key}", configString(s.opt.Config, "plugin.ai_chat_bot.api_key"),
		"${model}", configString(s.opt.Config, "plugin.ai_chat_bot.model"),
	)

	url := ph.Replace(s.balanceConfigString("bot.balance.url", DefaultBalanceURL))
	if url == "" {
		return "", fmt.Errorf("请求地址为空（检查 bot.balance.url 与 plugin.ai_chat_bot.base_url）")
	}
	method := strings.ToUpper(s.balanceConfigString("bot.balance.method", http.MethodGet))

	var body io.Reader
	if b := ph.Replace(configString(s.opt.Config, "bot.balance.body")); strings.TrimSpace(b) != "" {
		body = strings.NewReader(b)
	}

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return "", fmt.Errorf("构造请求失败: %w", err)
	}
	if raw := s.balanceConfigString("bot.balance.headers", DefaultBalanceHeaders); strings.TrimSpace(raw) != "" {
		var headers map[string]any
		if err := json.Unmarshal([]byte(raw), &headers); err != nil {
			return "", fmt.Errorf("请求头不是合法 JSON 对象: %w", err)
		}
		for k, v := range headers {
			req.Header.Set(k, ph.Replace(fmt.Sprint(v)))
		}
	}
	if body != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	client := &http.Client{Timeout: balanceHTTPTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, balanceMaxBody))
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate(string(data), 200))
	}
	if !gjson.ValidBytes(data) {
		return "", fmt.Errorf("响应不是合法 JSON: %s", truncate(string(data), 200))
	}

	format := s.balanceConfigString("bot.balance.format", DefaultBalanceFormat)
	return renderBalanceFormat(format, data)
}

// renderBalanceFormat 渲染余额显示模板：{gjson 路径} 替换为响应 JSON 中对应字段的值
// （数字保留原始字面量，字符串不带引号）。任一路径不存在即报错，便于排查配置。
func renderBalanceFormat(format string, body []byte) (string, error) {
	var missing []string
	out := balanceFormatFieldRe.ReplaceAllStringFunc(format, func(m string) string {
		path := strings.TrimSpace(m[1 : len(m)-1])
		r := gjson.GetBytes(body, path)
		if !r.Exists() {
			missing = append(missing, path)
			return ""
		}
		if r.Type == gjson.Number && r.Raw != "" {
			return r.Raw // 数字保留原始字面量（42.50 不变成 42.5）
		}
		return r.String()
	})
	if len(missing) > 0 {
		return "", fmt.Errorf("响应 JSON 中不存在路径: %s", strings.Join(missing, ", "))
	}
	if strings.TrimSpace(out) == "" {
		return "", fmt.Errorf("余额显示为空（检查 bot.balance.format）")
	}
	return out, nil
}

// balanceConfigString 读余额相关字符串配置，空值时回退到默认值。
func (s *Server) balanceConfigString(key, def string) string {
	if v := configString(s.opt.Config, key); strings.TrimSpace(v) != "" {
		return v
	}
	return def
}

// truncate 截断字符串到 n 个字节，用于错误信息中附带响应片段。
func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}

// configString 从配置中心读字符串配置（缺失返回空串）。
func configString(cfg interface {
	Get(string) (any, bool)
}, key string) string {
	v, ok := cfg.Get(key)
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

// toInt 把 JSON 解码出的数值（float64）或 int 转为 int。
func toInt(v any) (int, bool) {
	switch t := v.(type) {
	case float64:
		return int(t), true
	case int:
		return t, true
	}
	return 0, false
}
