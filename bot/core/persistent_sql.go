package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/jeanhua/AniaBot/common/storage"
)

// sqlDialect 描述不同 SQL 后端之间无法统一的语法差异（建表 DDL 与 UPSERT）。
// 其余语句在 SQLite 与 MySQL 间完全一致，故可共用同一份实现。
type sqlDialect struct {
	name      string
	upsertSQL string
	ddl       []string
}

var (
	sqliteDialect = sqlDialect{
		name: "sqlite",
		upsertSQL: `INSERT INTO ania_kv (namespace, key_name, val, updated_at) ` +
			`VALUES (?, ?, ?, CURRENT_TIMESTAMP) ` +
			`ON CONFLICT(namespace, key_name) DO UPDATE SET val = excluded.val, updated_at = CURRENT_TIMESTAMP`,
		ddl: []string{
			`CREATE TABLE IF NOT EXISTS ania_kv (` +
				`namespace TEXT NOT NULL, ` +
				`key_name TEXT NOT NULL, ` +
				`val TEXT NOT NULL, ` +
				`updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP, ` +
				`PRIMARY KEY (namespace, key_name))`,
			`CREATE INDEX IF NOT EXISTS idx_ania_kv_ns ON ania_kv(namespace)`,
		},
	}

	mysqlDialect = sqlDialect{
		name: "mysql",
		// 使用 REPLACE INTO 而非 INSERT ... ON DUPLICATE KEY UPDATE val = VALUES(val)，
		// 后者的 VALUES(col) 自 MySQL 8.0.20 起被弃用；REPLACE 在 5.7+/MariaDB 均稳定可用。
		upsertSQL: `REPLACE INTO ania_kv (namespace, key_name, val, updated_at) ` +
			`VALUES (?, ?, ?, CURRENT_TIMESTAMP)`,
		ddl: []string{
			// namespace/key_name 显式声明 utf8mb4_bin（字节序、大小写敏感），
			// 与 SQLite TEXT 的 BINARY 语义对齐；否则 MySQL 默认 CI 排序会破坏命名空间隔离
			// 与 Clear 的区间删除（prefixRange 仅在字节序下成立）。
			"CREATE TABLE IF NOT EXISTS ania_kv (" +
				"namespace VARCHAR(255) COLLATE utf8mb4_bin NOT NULL, " +
				"key_name VARCHAR(255) COLLATE utf8mb4_bin NOT NULL, " +
				"val MEDIUMTEXT NOT NULL, " +
				"updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP, " +
				"PRIMARY KEY (namespace, key_name), " +
				"KEY idx_ania_kv_ns (namespace)" +
				") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4",
		},
	}
)

// aniaSqlPersistentStorage 基于 database/sql 的持久化存储实现。
// SQLite 与 MySQL 共用此实现，仅方言（DDL/UPSERT）不同。命名空间语义与
// 缓存 AniaRedisStorage/AniaMemoryStorage 的 prefix 完全一致，便于在两层之间迁移。
type aniaSqlPersistentStorage struct {
	namespace string // 命名空间前缀，语义同缓存的 prefix
	db        *sql.DB
	dialect  sqlDialect
	logger   *slog.Logger
}

// newSqlPersistentStorage 在打开的 *sql.DB 上建表并返回一个持久化存储实例。
func newSqlPersistentStorage(ctx context.Context, db *sql.DB, dialect sqlDialect, logger *slog.Logger) (*aniaSqlPersistentStorage, error) {
	for _, stmt := range dialect.ddl {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return nil, fmt.Errorf("init persistent storage schema (%s): %w", dialect.name, err)
		}
	}
	return &aniaSqlPersistentStorage{db: db, dialect: dialect, logger: logger}, nil
}

func (s *aniaSqlPersistentStorage) Clone(prefix string) storage.PersistentStorage {
	return &aniaSqlPersistentStorage{
		namespace: s.namespace + prefix + ":",
		db:        s.db,
		dialect:   s.dialect,
		logger:    s.logger,
	}
}

func (s *aniaSqlPersistentStorage) GetString(ctx context.Context, key string) (string, bool) {
	var val string
	err := s.db.QueryRowContext(ctx,
		`SELECT val FROM ania_kv WHERE namespace = ? AND key_name = ?`,
		s.namespace, key,
	).Scan(&val)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			s.logger.Error("持久化存储读取失败", "namespace", s.namespace, "key", key, "error", err)
		}
		return "", false
	}
	return val, true
}

func (s *aniaSqlPersistentStorage) SetString(ctx context.Context, key, val string) bool {
	_, err := s.db.ExecContext(ctx, s.dialect.upsertSQL, s.namespace, key, val)
	if err != nil {
		s.logger.Error("持久化存储写入失败", "namespace", s.namespace, "key", key, "error", err)
		return false
	}
	return true
}

func (s *aniaSqlPersistentStorage) Get(ctx context.Context, key string, out any) bool {
	val, ok := s.GetString(ctx, key)
	if !ok {
		return false
	}
	if err := json.Unmarshal([]byte(val), out); err != nil {
		s.logger.Error("持久化存储反序列化失败", "namespace", s.namespace, "key", key, "error", err)
		return false
	}
	return true
}

func (s *aniaSqlPersistentStorage) Set(ctx context.Context, key string, val any) bool {
	data, err := json.Marshal(val)
	if err != nil {
		s.logger.Error("持久化存储序列化失败", "namespace", s.namespace, "key", key, "error", err)
		return false
	}
	return s.SetString(ctx, key, string(data))
}

func (s *aniaSqlPersistentStorage) Has(ctx context.Context, key string) bool {
	var one int
	err := s.db.QueryRowContext(ctx,
		`SELECT 1 FROM ania_kv WHERE namespace = ? AND key_name = ?`,
		s.namespace, key,
	).Scan(&one)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			s.logger.Error("持久化存储查询失败", "namespace", s.namespace, "key", key, "error", err)
		}
		return false
	}
	return true
}

func (s *aniaSqlPersistentStorage) Del(ctx context.Context, key string) bool {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM ania_kv WHERE namespace = ? AND key_name = ?`,
		s.namespace, key,
	)
	if err != nil {
		s.logger.Error("持久化存储删除失败", "namespace", s.namespace, "key", key, "error", err)
		return false
	}
	return true
}

func (s *aniaSqlPersistentStorage) Keys(ctx context.Context, prefix string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT key_name FROM ania_kv WHERE namespace = ?`,
		s.namespace,
	)
	if err != nil {
		return nil, fmt.Errorf("scan keys: %w", err)
	}
	defer rows.Close()

	var keys []string
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return nil, fmt.Errorf("scan keys: %w", err)
		}
		if prefix == "" || strings.HasPrefix(k, prefix) {
			keys = append(keys, k)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("scan keys: %w", err)
	}
	sort.Strings(keys)
	return keys, nil
}

func (s *aniaSqlPersistentStorage) Clear(ctx context.Context) bool {
	// 清空当前命名空间及其所有子命名空间，与缓存 Clear 语义一致。
	if s.namespace == "" {
		_, err := s.db.ExecContext(ctx, `DELETE FROM ania_kv`)
		if err != nil {
			s.logger.Error("持久化存储清空失败", "error", err)
			return false
		}
		return true
	}
	lo, hi, ok := prefixRange(s.namespace)
	var err error
	if ok {
		// 区间删除：namespace >= lo AND namespace < hi 恰好覆盖 lo 本身及所有以 lo 为前缀的子命名空间
		_, err = s.db.ExecContext(ctx,
			`DELETE FROM ania_kv WHERE namespace >= ? AND namespace < ?`,
			lo, hi,
		)
	} else {
		// 回退：命名空间末字节为 0xFF 的极端情况，逐命名空间清理
		err = s.clearByIteration(ctx)
	}
	if err != nil {
		s.logger.Error("持久化存储清空失败", "namespace", s.namespace, "error", err)
		return false
	}
	return true
}

func (s *aniaSqlPersistentStorage) clearByIteration(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT namespace FROM ania_kv`)
	if err != nil {
		return err
	}
	// 先收集命名空间并关闭 rows 释放连接，再执行删除：
	// SQLite 下 MaxOpenConns(1)，若 rows 仍持有唯一连接时执行 Exec 会自死锁。
	var namespaces []string
	for rows.Next() {
		var ns string
		if err := rows.Scan(&ns); err != nil {
			rows.Close()
			return err
		}
		if strings.HasPrefix(ns, s.namespace) {
			namespaces = append(namespaces, ns)
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, ns := range namespaces {
		if _, err := s.db.ExecContext(ctx, `DELETE FROM ania_kv WHERE namespace = ?`, ns); err != nil {
			return err
		}
	}
	return nil
}

// prefixRange 将前缀转换为 [lo, hi) 区间，用于高效的范围删除/扫描。
// hi 为前缀末字节自增 1 后的字符串；当末字节为 0xFF 无法自增时返回 ok=false。
// 由于 namespace 由 base64（插件名）与 Clone 前缀组成，几乎均为 ASCII，此技巧可安全使用。
func prefixRange(prefix string) (lo, hi string, ok bool) {
	if prefix == "" {
		return "", "", false
	}
	b := []byte(prefix)
	if b[len(b)-1] == 0xFF {
		return prefix, "", false
	}
	b[len(b)-1]++
	return prefix, string(b), true
}
