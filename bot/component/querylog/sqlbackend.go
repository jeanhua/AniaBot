package querylog

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"strings"

	"github.com/jeanhua/AniaBot/common/storage"
)

// Query 日志的行级存储 schema：每条日志一行，seq 自增主键（计数由 Logger
// 在互斥内分配，启动时 SELECT MAX(seq) 初始化）。payload 为 Entry 完整 JSON；
// 其余列是过滤条件的冗余下推列——SQL 仅用于收窄候选，Filter.match 仍为最终判定。
var queryLogTables = []storage.TableDDL{
	{
		Name: "ania_query_log",
		SQLite: []string{
			`CREATE TABLE IF NOT EXISTS ania_query_log (` +
				`seq INTEGER NOT NULL PRIMARY KEY, ` +
				`chat_type TEXT NOT NULL, ` +
				`target_id TEXT NOT NULL, ` +
				`senders TEXT NOT NULL, ` +
				`status TEXT NOT NULL, ` +
				`query_text TEXT NOT NULL, ` +
				`created_at INTEGER NOT NULL, ` +
				`payload TEXT NOT NULL)`,
			`CREATE INDEX IF NOT EXISTS idx_ania_query_log_time ON ania_query_log(created_at)`,
		},
		MySQL: []string{
			`CREATE TABLE IF NOT EXISTS ania_query_log (` +
				`seq BIGINT NOT NULL, ` +
				`chat_type VARCHAR(16) COLLATE utf8mb4_bin NOT NULL, ` +
				`target_id VARCHAR(255) COLLATE utf8mb4_bin NOT NULL, ` +
				`senders MEDIUMTEXT NOT NULL, ` +
				`status VARCHAR(16) COLLATE utf8mb4_bin NOT NULL, ` +
				`query_text MEDIUMTEXT NOT NULL, ` +
				`created_at BIGINT NOT NULL, ` +
				`payload MEDIUMTEXT NOT NULL, ` +
				`PRIMARY KEY (seq), ` +
				`KEY idx_ania_query_log_time (created_at)` +
				`) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		},
	},
}

// sqlBackend SQL 版日志存储：过滤条件下推 WHERE、容量淘汰走范围删除，
// 避免 KV 后端每次读取全量列键 + 逐键加载的开销。SQLite 单连接下读取
// 遵循"收集→关闭 rows→解析"纪律。
type sqlBackend struct {
	db     *sql.DB
	logger *slog.Logger
}

func newSQLBackend(db *sql.DB, logger *slog.Logger) *sqlBackend {
	return &sqlBackend{db: db, logger: logger}
}

func (b *sqlBackend) maxSeq() uint64 {
	var n uint64
	if err := b.db.QueryRowContext(context.Background(),
		`SELECT COALESCE(MAX(seq), 0) FROM ania_query_log`).Scan(&n); err != nil {
		b.logger.Error("querylog 读取最大序号失败", "error", err)
		return 0
	}
	return n
}

// logColumns 从 Entry 导出冗余下推列与完整 payload。senders 存包裹式 CSV
// （",a,b,"），精确匹配某发言者用 LIKE '%,id,%'，不会误中子串。
func logColumns(e Entry) (chatType, targetID, senders, status, queryText string, createdAt int64, payload string, err error) {
	data, err := json.Marshal(e)
	if err != nil {
		return "", "", "", "", "", 0, "", err
	}
	senders = ","
	for _, s := range e.Senders {
		senders += s + ","
	}
	return e.ChatType, e.TargetID, senders, string(e.Status), e.Query, e.Time.Unix(), string(data), nil
}

func (b *sqlBackend) insert(seq uint64, e Entry) {
	chatType, targetID, senders, status, queryText, createdAt, payload, err := logColumns(e)
	if err != nil {
		b.logger.Error("querylog 序列化失败", "id", e.ID, "error", err)
		return
	}
	if _, err := b.db.ExecContext(context.Background(),
		`INSERT INTO ania_query_log (seq, chat_type, target_id, senders, status, query_text, created_at, payload) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		seq, chatType, targetID, senders, status, queryText, createdAt, payload); err != nil {
		b.logger.Error("querylog 落盘失败", "id", e.ID, "error", err)
	}
}

func (b *sqlBackend) load(seq uint64) (Entry, bool) {
	var payload string
	err := b.db.QueryRowContext(context.Background(),
		`SELECT payload FROM ania_query_log WHERE seq = ?`, seq).Scan(&payload)
	if err != nil {
		return Entry{}, false
	}
	var e Entry
	if err := json.Unmarshal([]byte(payload), &e); err != nil {
		b.logger.Error("querylog 反序列化失败", "seq", seq, "error", err)
		return Entry{}, false
	}
	return e, true
}

func (b *sqlBackend) overwrite(seq uint64, e Entry) {
	chatType, targetID, senders, status, queryText, createdAt, payload, err := logColumns(e)
	if err != nil {
		b.logger.Error("querylog 序列化失败", "id", e.ID, "error", err)
		return
	}
	if _, err := b.db.ExecContext(context.Background(),
		`UPDATE ania_query_log SET chat_type = ?, target_id = ?, senders = ?, status = ?, query_text = ?, created_at = ?, payload = ? WHERE seq = ?`,
		chatType, targetID, senders, status, queryText, createdAt, payload, seq); err != nil {
		b.logger.Error("querylog 落盘失败", "id", e.ID, "error", err)
	}
}

func (b *sqlBackend) evict(maxSeq uint64, maxEntries int) {
	if maxEntries <= 0 || maxSeq <= uint64(maxEntries) {
		return
	}
	// 序号单调递增不复用：淘汰线以下的一定是最旧的记录
	if _, err := b.db.ExecContext(context.Background(),
		`DELETE FROM ania_query_log WHERE seq <= ?`, maxSeq-uint64(maxEntries)); err != nil {
		b.logger.Error("querylog 淘汰失败", "error", err)
	}
}

func (b *sqlBackend) recent(limit int) []Entry {
	q := `SELECT payload FROM ania_query_log ORDER BY seq DESC`
	args := []any{}
	if limit > 0 {
		q += ` LIMIT ?`
		args = append(args, limit)
	}
	return b.loadPayloads(q, args, func(Entry) bool { return true }, limit)
}

func (b *sqlBackend) query(f Filter, beforeSeq uint64, limit int) []Entry {
	var where []string
	var args []any
	if f.ChatType != "" {
		where = append(where, `chat_type = ?`)
		args = append(args, f.ChatType)
	}
	if f.TargetID != "" {
		where = append(where, `target_id = ?`)
		args = append(args, f.TargetID)
	}
	if f.Sender != "" {
		where = append(where, `senders LIKE ? ESCAPE '\'`)
		args = append(args, "%,"+escapeLike(f.Sender)+",%")
	}
	if !f.Start.IsZero() {
		where = append(where, `created_at >= ?`)
		args = append(args, f.Start.Unix())
	}
	if !f.End.IsZero() {
		where = append(where, `created_at <= ?`)
		args = append(args, f.End.Unix())
	}
	if f.Keyword != "" {
		// LOWER() 对齐 MySQL utf8mb4_bin 的大小写敏感与现有不区分大小写语义
		// （均为 ASCII 级折叠；非 ASCII 大小写对的极端差异由 Go 侧 match 终判兜住）
		where = append(where, `LOWER(query_text) LIKE LOWER(?) ESCAPE '\'`)
		args = append(args, "%"+escapeLike(f.Keyword)+"%")
	}
	if beforeSeq > 0 {
		where = append(where, `seq < ?`)
		args = append(args, beforeSeq)
	}
	q := `SELECT payload FROM ania_query_log`
	if len(where) > 0 {
		q += ` WHERE ` + strings.Join(where, ` AND `)
	}
	q += ` ORDER BY seq DESC`
	// 表总量受容量淘汰约束（默认 200 条），不加 LIMIT 全扫有界；
	// 凑满 limit 条匹配即停（SQL 下推只是收窄，match 才是终判）
	return b.loadPayloads(q, args, f.match, limit)
}

// loadPayloads 执行查询并逐行解码，仅保留 match 通过的记录，凑满 limit 条
// （limit<=0 不限）即停止读取。
func (b *sqlBackend) loadPayloads(q string, args []any, match func(Entry) bool, limit int) []Entry {
	rows, err := b.db.QueryContext(context.Background(), q, args...)
	if err != nil {
		b.logger.Error("querylog 查询失败", "error", err)
		return nil
	}
	var payloads []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			rows.Close()
			b.logger.Error("querylog 查询失败", "error", err)
			return nil
		}
		payloads = append(payloads, p)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		b.logger.Error("querylog 查询失败", "error", err)
		return nil
	}
	rows.Close()

	out := make([]Entry, 0, len(payloads))
	for _, p := range payloads {
		var e Entry
		if err := json.Unmarshal([]byte(p), &e); err != nil {
			b.logger.Error("querylog 反序列化失败，跳过该条", "error", err)
			continue
		}
		if !match(e) {
			continue
		}
		out = append(out, e)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

// escapeLike 转义 LIKE 模式中的特殊字符（配合 ESCAPE '\'）。
func escapeLike(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(s)
}
