package pluginaichat

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/jeanhua/AniaBot/bot/component/aichat"
	"github.com/jeanhua/AniaBot/common/plugininfo"
	"github.com/jeanhua/AniaBot/common/storage"
)

// quotaKeyDate 配额计数相对键：daily:<日期>:<会话key> 与 daily:<日期>:global。
// 键带日期天然按天过期：当天结束新键从零开始，旧键由 pruneOld 惰性清理。
const quotaKeyDate = "daily:"

// quotaManager 每日 Token 配额管理器：按「每会话每日」与「全局每日」两个维度
// 限制 AI 消耗。计数持久化到 PersistentStorage（quota: 命名空间，sqlite/mysql
// 落盘，重启不丢）。
//
// 并发语义：插件层主对话请求在会话锁内串行，会话维度无超售；全局维度
// Check-Add 之间存在小竞态窗口（其他会话可插入消耗），配额为宽松语义
// （非硬实时），被拒后下一条请求即生效。所有计数在进程内 mu 下串行读写。
type quotaManager struct {
	store       storage.PersistentStorage // Clone("quota:")
	logger      *slog.Logger
	dailyLimit  int              // 每会话每日 token 上限，<=0 不限制
	globalLimit int              // 全局每日 token 上限，<=0 不限制
	now         func() time.Time // 可注入时钟（测试用），默认 time.Now

	mu sync.Mutex // 全局计数器与跨会话操作串行化
}

func newQuotaManager(store storage.PersistentStorage, logger *slog.Logger, dailyLimit, globalLimit int) *quotaManager {
	return &quotaManager{
		store:       store.Clone("quota:"),
		logger:      logger,
		dailyLimit:  dailyLimit,
		globalLimit: globalLimit,
		now:         time.Now,
	}
}

// Check 检查指定会话是否还有今日配额；拒绝时返回提示文案与 true。
// 未启用配额（双维度均不限）时恒不拒绝。
func (q *quotaManager) Check(sessionKey string) (string, bool) {
	if q == nil || (q.dailyLimit <= 0 && q.globalLimit <= 0) {
		return "", false
	}
	q.mu.Lock()
	defer q.mu.Unlock()

	date := q.now().Format("2006-01-02")
	if q.dailyLimit > 0 {
		if used := q.readLocked(quotaKeyDate + date + ":" + sessionKey); used >= int64(q.dailyLimit) {
			return fmt.Sprintf("今日会话配额已用尽（每会话每日上限 %d tokens），明天再来吧", q.dailyLimit), true
		}
	}
	if q.globalLimit > 0 {
		if used := q.readLocked(quotaKeyDate + date + ":global"); used >= int64(q.globalLimit) {
			return fmt.Sprintf("今日全局配额已用尽（全局每日上限 %d tokens），明天再来吧", q.globalLimit), true
		}
	}
	return "", false
}

// Add 累加一次 AI 调用的 token 消耗到所属会话与全局计数。
// TotalTokens 缺失（上游未上报）时用 Prompt+Completion 兜底；均为 0 不记录。
func (q *quotaManager) Add(sessionKey string, usage aichat.TokenUsage) {
	if q == nil || (q.dailyLimit <= 0 && q.globalLimit <= 0) {
		return
	}
	tokens := int64(usage.TotalTokens)
	if tokens <= 0 {
		tokens = int64(usage.PromptTokens + usage.CompletionTokens)
	}
	if tokens <= 0 {
		return
	}

	q.mu.Lock()
	defer q.mu.Unlock()
	date := q.now().Format("2006-01-02")
	if q.dailyLimit > 0 {
		q.incrLocked(quotaKeyDate+date+":"+sessionKey, tokens)
	}
	if q.globalLimit > 0 {
		q.incrLocked(quotaKeyDate+date+":global", tokens)
	}
}

// Summary 当日配额汇总（全局 + 各会话明细），供面板「配额管理」页展示。
func (q *quotaManager) Summary() (plugininfo.QuotaSummaryInfo, error) {
	info := plugininfo.QuotaSummaryInfo{Date: q.now().Format("2006-01-02")}
	if q == nil {
		return info, errors.New("配额功能未启用")
	}
	q.mu.Lock()
	defer q.mu.Unlock()

	prefix := quotaKeyDate + info.Date + ":"
	keys, err := q.store.Keys(context.Background(), prefix)
	if err != nil {
		return info, err
	}
	for _, k := range keys {
		used := q.readLocked(k)
		scope := strings.TrimPrefix(k, prefix)
		if scope == "global" {
			info.GlobalUsed = used
			continue
		}
		si := plugininfo.QuotaSessionInfo{
			Key:     scope,
			Used:    used,
			Limit:   int64(q.dailyLimit),
			Reached: q.dailyLimit > 0 && used >= int64(q.dailyLimit),
		}
		if kind, target, ok := strings.Cut(scope, ":"); ok {
			si.Target = target
			if kind == "g" {
				si.Kind = "group"
			} else if kind == "f" {
				si.Kind = "friend"
			}
		}
		si.Remaining = si.Limit - si.Used
		if si.Remaining < 0 {
			si.Remaining = 0
		}
		info.Sessions = append(info.Sessions, si)
	}
	// 用量降序，面板优先展示消耗大户
	slices.SortFunc(info.Sessions, func(a, b plugininfo.QuotaSessionInfo) int {
		return int(b.Used - a.Used)
	})

	info.GlobalLimit = int64(q.globalLimit)
	info.GlobalRemaining = info.GlobalLimit - info.GlobalUsed
	if info.GlobalRemaining < 0 {
		info.GlobalRemaining = 0
	}
	info.GlobalReached = q.globalLimit > 0 && info.GlobalUsed >= int64(q.globalLimit)
	return info, nil
}

// Reset 清零配额计数：scope 为 "all" 时清空当日全部计数（全局与会话），
// 否则仅清除指定会话（g:/f: 前缀）。返回是否成功。
func (q *quotaManager) Reset(scope string) bool {
	if q == nil {
		return false
	}
	q.mu.Lock()
	defer q.mu.Unlock()

	date := q.now().Format("2006-01-02")
	if scope == "all" {
		keys, err := q.store.Keys(context.Background(), quotaKeyDate+date+":")
		if err != nil {
			q.logger.Error("列出配额键失败", "error", err)
			return false
		}
		for _, k := range keys {
			q.store.Del(context.Background(), k)
		}
		return true
	}
	return q.store.Del(context.Background(), quotaKeyDate+date+":"+scope)
}

// pruneOld 惰性清理过期日期键，仅保留最近 keepDays 天的计数
// （默认值传 3：配额键只影响当日判断，留 3 天余量供面板回看）。
func (q *quotaManager) pruneOld(keepDays int) {
	if q == nil {
		return
	}
	q.mu.Lock()
	defer q.mu.Unlock()

	keys, err := q.store.Keys(context.Background(), quotaKeyDate)
	if err != nil {
		q.logger.Warn("清理过期配额计数失败", "error", err)
		return
	}
	cutoff := q.now().AddDate(0, 0, -keepDays).Format("2006-01-02")
	for _, k := range keys {
		// 相对键形如 daily:2006-01-02:...，提取日期部分比较
		date := strings.TrimPrefix(k, quotaKeyDate)
		if i := strings.IndexByte(date, ':'); i >= 0 {
			date = date[:i]
		}
		if date < cutoff {
			q.store.Del(context.Background(), k)
		}
	}
}

func (q *quotaManager) readLocked(key string) int64 {
	var used int64
	q.store.Get(context.Background(), key, &used)
	return used
}

func (q *quotaManager) incrLocked(key string, delta int64) {
	used := q.readLocked(key) + delta
	if ok := q.store.Set(context.Background(), key, used); !ok {
		q.logger.Error("配额计数落盘失败", "key", key)
	}
}

// ---- 面板「配额管理」页接口（实现 adminpanel.QuotaSource）----

// QuotaSummary 返回当日配额汇总（全局 + 各会话明细）。
func (p *AIChatPlugin) QuotaSummary() (plugininfo.QuotaSummaryInfo, error) {
	if p.quotaManager == nil {
		return plugininfo.QuotaSummaryInfo{}, errors.New("配额功能未启用（plugin.ai_chat_bot.quota.enable=false）")
	}
	return p.quotaManager.Summary()
}

// QuotaReset 清零配额计数：scope 为 "all" 清空当日全部，否则仅清除指定会话。
func (p *AIChatPlugin) QuotaReset(scope string) error {
	if p.quotaManager == nil {
		return errors.New("配额功能未启用")
	}
	if scope != "all" && !validScope(scope) {
		return fmt.Errorf("非法 scope: %s（应为 g:群号 / f:QQ号 / all）", scope)
	}
	if !p.quotaManager.Reset(scope) {
		return errors.New("清零失败，请查看日志")
	}
	return nil
}
