// balance.go —— API 余额查询。
//
// 面板概览页展示 AI API 余额。余额接口各厂商差异巨大，因此请求方式由用户在
// 面板配置中自定义一段 JS（bot.balance.js），后端用 goja 执行：脚本内通过
// 全局 cfg（AI 对话插件的 base_url/api_key/model）与同步 fetch() 发起请求，
// 最后一条表达式的值作为余额显示。结果按 bot.balance.cache_sec 缓存，
// 避免面板轮询频繁打余额接口。配置即时读取（configstore 直查 DB），修改无需重启。
package adminpanel

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/dop251/goja"
)

// DefaultBalanceJS 默认的余额查询脚本（适配 DeepSeek 风格 /user/balance 接口，
// 也是配置项 bot.balance.js 的默认值；core 包注册配置字段时引用）。
const DefaultBalanceJS = `// 可用全局变量：
//   cfg.base_url / cfg.api_key / cfg.model —— AI 对话插件的 API 配置
//   fetch(url, { method, headers, body }) —— 同步 HTTP 请求，返回 { status, body, json() }
//   console.log(...) —— 输出到 Bot 日志
// 最后一条表达式的值会作为余额显示（数字 / 字符串 / 对象均可）
const resp = fetch(cfg.base_url.replace(/\/+$/, '') + '/user/balance', {
  headers: { Authorization: 'Bearer ' + cfg.api_key },
})
if (resp.status !== 200) throw new Error('HTTP ' + resp.status + ': ' + resp.body.slice(0, 200))
const data = resp.json()
const list = (data.data && data.data.balances) || []
const b = list.length ? list[0] : {}
const cur = b.currency === 'CNY' ? '¥' : (b.currency || '')
cur + ' ' + (b.total_balance != null ? b.total_balance : '?')
`

// balanceExecTimeout JS 脚本整体执行超时（含脚本内所有 HTTP 请求）
const balanceExecTimeout = 30 * time.Second

// balanceHTTPTimeout 脚本内单次 fetch 的 HTTP 超时
const balanceHTTPTimeout = 15 * time.Second

// balanceMaxBody 单次 fetch 响应体大小上限
const balanceMaxBody = 4 << 20 // 4 MiB

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

// queryBalance 读配置并执行余额查询脚本，带缓存；脚本/请求错误不返回 HTTP 错误，
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
	script, _ := s.opt.Config.Get("bot.balance.js")
	code, _ := script.(string)
	if strings.TrimSpace(code) == "" {
		code = DefaultBalanceJS
	}
	value, err := s.runBalanceScript(code)
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

// runBalanceScript 用 goja 执行用户脚本，注入 cfg / fetch / console，返回最后一条
// 表达式的显示文本。超时通过 vm.Interrupt 强制中断。
func (s *Server) runBalanceScript(code string) (string, error) {
	vm := goja.New()
	timer := time.AfterFunc(balanceExecTimeout, func() { vm.Interrupt("余额查询脚本执行超时") })
	defer timer.Stop()

	cfg := map[string]string{
		"base_url": configString(s.opt.Config, "plugin.ai_chat_bot.base_url"),
		"api_key":  configString(s.opt.Config, "plugin.ai_chat_bot.api_key"),
		"model":    configString(s.opt.Config, "plugin.ai_chat_bot.model"),
	}
	if err := vm.Set("cfg", cfg); err != nil {
		return "", err
	}
	if err := vm.Set("fetch", s.makeBalanceFetch(vm)); err != nil {
		return "", err
	}
	console := vm.NewObject()
	_ = console.Set("log", func(args ...any) {
		s.opt.Logger.Info("余额查询脚本", "console", fmt.Sprint(args...))
	})
	if err := vm.Set("console", console); err != nil {
		return "", err
	}

	v, err := vm.RunString(code)
	if err != nil {
		return "", fmt.Errorf("脚本执行失败: %w", err)
	}
	return balanceDisplay(v.Export())
}

// balanceDisplay 把脚本返回值转成显示文本：数字去尾零、对象 JSON 化。
func balanceDisplay(v any) (string, error) {
	switch t := v.(type) {
	case nil:
		return "", fmt.Errorf("脚本没有返回值（最后一条表达式的值作为余额显示）")
	case string:
		if strings.TrimSpace(t) == "" {
			return "", fmt.Errorf("脚本返回了空字符串")
		}
		return t, nil
	case float64:
		return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.4f", t), "0"), "."), nil
	case int64:
		return fmt.Sprintf("%d", t), nil
	case bool:
		return fmt.Sprintf("%v", t), nil
	default:
		data, err := json.Marshal(t)
		if err != nil {
			return "", fmt.Errorf("脚本返回值无法显示: %w", err)
		}
		return string(data), nil
	}
}

// makeBalanceFetch 构造注入脚本的同步 fetch(url, options)：
// options 支持 method（默认 GET）、headers（对象）、body（字符串）；
// 返回 { status, headers, body, json() }。
func (s *Server) makeBalanceFetch(vm *goja.Runtime) func(goja.FunctionCall) goja.Value {
	client := &http.Client{Timeout: balanceHTTPTimeout}
	return func(call goja.FunctionCall) goja.Value {
		url := call.Argument(0).String()
		if url == "" || url == "undefined" {
			panic(vm.NewTypeError("fetch: 缺少 url 参数"))
		}
		method := http.MethodGet
		headers := map[string]string{}
		var body io.Reader
		defined := func(v goja.Value) bool { return v != nil && !goja.IsUndefined(v) && !goja.IsNull(v) }
		if opt := call.Argument(1); defined(opt) {
			obj := opt.ToObject(vm)
			if m := obj.Get("method"); defined(m) {
				method = strings.ToUpper(m.String())
			}
			if h := obj.Get("headers"); defined(h) {
				if exported := h.Export(); exported != nil {
					if m, ok := exported.(map[string]any); ok {
						for k, v := range m {
							headers[k] = fmt.Sprint(v)
						}
					}
				}
			}
			if b := obj.Get("body"); defined(b) {
				body = strings.NewReader(b.String())
			}
		}

		req, err := http.NewRequest(method, url, body)
		if err != nil {
			panic(vm.NewGoError(fmt.Errorf("fetch: 构造请求失败: %w", err)))
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		if body != nil && req.Header.Get("Content-Type") == "" {
			req.Header.Set("Content-Type", "application/json")
		}
		resp, err := client.Do(req)
		if err != nil {
			panic(vm.NewGoError(fmt.Errorf("fetch: 请求失败: %w", err)))
		}
		defer resp.Body.Close()
		data, err := io.ReadAll(io.LimitReader(resp.Body, balanceMaxBody))
		if err != nil {
			panic(vm.NewGoError(fmt.Errorf("fetch: 读取响应失败: %w", err)))
		}
		bodyStr := string(data)

		out := vm.NewObject()
		_ = out.Set("status", resp.StatusCode)
		_ = out.Set("body", bodyStr)
		_ = out.Set("headers", resp.Header)
		_ = out.Set("json", func() (any, error) {
			var v any
			if err := json.Unmarshal([]byte(bodyStr), &v); err != nil {
				return nil, fmt.Errorf("fetch: 响应不是合法 JSON: %w", err)
			}
			return v, nil
		})
		return out
	}
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
