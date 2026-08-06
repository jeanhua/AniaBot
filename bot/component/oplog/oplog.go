// Package oplog 提供操作日志的持久化记录能力。
//
// 记录各类管理操作的审计轨迹：面板登录、配置修改、定时任务/记忆/技能/
// 知识库/团队管理、AI 工具修改配置、重启与自动更新等。
// 存储为双后端结构：SQL 后端下每条日志一行（ania_op_log 表，过滤条件下推
// WHERE、容量淘汰走范围删除）；非 SQL 后端回退为逐条 KV 记录（key 为
// e:<序号>）。日志 ID 为自增序号的 base36，两种后端一致。
//
// 使用方式为包级单例：core 启动时调用 [Init] 注入存储，之后各调用方
// （面板 handler、AI 工具等）直接调 [Record]/[Query]。未初始化时
// Record 静默丢弃（测试等场景无需初始化）。
package oplog

import (
	"context"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jeanhua/AniaBot/common/storage"
)

// 操作分类
const (
	CategoryAuth      = "auth"      // 登录 / 密码
	CategoryConfig    = "config"    // 配置修改（含预设 / 扩展配置文件）
	CategoryClock     = "clock"     // 定时任务管理
	CategorySkill     = "skill"     // skill 管理
	CategoryMemory    = "memory"    // 记忆管理
	CategoryTeam      = "team"      // Agent 团队管理
	CategoryKnowledge = "knowledge" // 知识库管理
	CategoryQuota     = "quota"     // 配额管理
	CategorySystem    = "system"    // 系统（启动 / 重启 / 设置向导）
	CategoryUpdate    = "update"    // 自动更新
	CategoryAI        = "ai"        // AI 工具发起的操作
)

// MaxDetailRunes 单条日志详情的符文数上限，避免单条日志体积失控。
const MaxDetailRunes = 500

// Entry 一条操作日志
type Entry struct {
	ID       string    `json:"id"`
	Time     time.Time `json:"time"`
	Category string    `json:"category"` // 见 Category* 常量
	Action   string    `json:"action"`   // 操作名，如 login / config_update / clock_create
	Detail   string    `json:"detail"`   // 操作详情（截断）
}

const defaultMax = 500

// backend 日志存储后端。写路径由包级互斥串行化，后端实现无需额外加锁。
type backend interface {
	// maxSeq 返回已分配的最大序号（启动初始化计数器用）；无记录时返回 0。
	maxSeq() uint64
	// insert 追加一条日志。
	insert(seq uint64, e Entry)
	// evict 淘汰旧记录，仅保留序号最大的 maxEntries 条；maxSeq 为当前最大序号。
	evict(maxSeq uint64, maxEntries int)
	// query 按条件过滤日志（新在前），beforeSeq>0 时仅返回序号更小的记录，
	// 最多 limit 条（limit<=0 不限）。f.match 为最终判定。
	query(f Filter, beforeSeq uint64, limit int) []Entry
}

var (
	mu     sync.Mutex
	be     backend
	seq    uint64
	maxEnt = defaultMax
	logger = slog.Default()
)

// Init 初始化操作日志存储。store 应为已隔离好命名空间的子存储；
// maxEntries<=0 时取默认值。SQL 后端（storage.SQLBackend 探测成功且建表成功）
// 走 ania_op_log 行级存储，否则回退逐条 KV 记录。
func Init(store storage.PersistentStorage, maxEntries int, l *slog.Logger) {
	mu.Lock()
	defer mu.Unlock()
	if l != nil {
		logger = l
	}
	if maxEntries > 0 {
		maxEnt = maxEntries
	}
	var b backend = newKVBackend(store, logger)
	if db, dialect, ok := storage.SQLBackend(store); ok {
		if err := storage.EnsureTables(context.Background(), db, dialect, opLogTables...); err != nil {
			logger.Error("创建操作日志表失败，回退 KV 存储", "error", err.Error())
		} else {
			b = newSQLBackend(db, logger)
		}
	}
	be = b
	seq = b.maxSeq()
}

// Record 追加一条操作日志并落盘，返回写入的 Entry（含分配的 ID）。
// 未调用 Init 时静默丢弃（返回零值 Entry）。
func Record(category, action, detail string) Entry {
	mu.Lock()
	defer mu.Unlock()
	if be == nil {
		return Entry{}
	}

	seq++
	e := Entry{
		ID:       strconv.FormatUint(seq, 36),
		Time:     time.Now(),
		Category: category,
		Action:   action,
		Detail:   Truncate(detail, MaxDetailRunes),
	}
	be.insert(seq, e)
	be.evict(seq, maxEnt)
	return e
}

// Filter 操作日志的查询条件，零值字段不参与过滤。
type Filter struct {
	Category string    // 操作分类（精确匹配）
	Start    time.Time // 起始时间（含），零值不限
	End      time.Time // 截止时间（含），零值不限
	Keyword  string    // 操作名 / 详情包含的关键词（不区分大小写）
	Before   string    // 分页游标：仅返回比该日志 ID 更旧的记录（空为从最新开始）
	Limit    int       // 返回条数上限，<=0 时取默认值
}

// match 判断一条日志是否满足过滤条件。
func (f Filter) match(e Entry) bool {
	if f.Category != "" && e.Category != f.Category {
		return false
	}
	if !f.Start.IsZero() && e.Time.Before(f.Start) {
		return false
	}
	if !f.End.IsZero() && e.Time.After(f.End) {
		return false
	}
	if f.Keyword != "" {
		kw := strings.ToLower(f.Keyword)
		if !strings.Contains(strings.ToLower(e.Action), kw) &&
			!strings.Contains(strings.ToLower(e.Detail), kw) {
			return false
		}
	}
	return true
}

// Query 按条件过滤日志（新在前），最多返回 f.Limit 条（<=0 时取默认值）。
// f.Before 非空时作为分页游标，仅返回比它更旧的记录。未初始化时返回 nil。
func Query(f Filter) []Entry {
	limit := f.Limit
	if limit <= 0 {
		limit = defaultMax
	}
	mu.Lock()
	defer mu.Unlock()
	if be == nil {
		return nil
	}
	return be.query(f, parseBeforeSeq(f.Before), limit)
}

// parseBeforeSeq 把分页游标（日志 ID，序号的 base36）解析为序号；非法时为 0（不生效）。
func parseBeforeSeq(before string) uint64 {
	if before == "" {
		return 0
	}
	n, err := strconv.ParseUint(before, 36, 64)
	if err != nil {
		return 0
	}
	return n
}

// Truncate 按符文数截断字符串，超出时追加省略标记。max<=0 时不截断。
func Truncate(s string, max int) string {
	if max <= 0 {
		return s
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "…"
}
