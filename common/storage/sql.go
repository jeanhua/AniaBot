package storage

import (
	"context"
	"database/sql"
	"fmt"
)

// SQLDialect SQL 后端方言标识。
type SQLDialect string

const (
	SQLDialectSQLite SQLDialect = "sqlite"
	SQLDialectMySQL  SQLDialect = "mysql"
)

// SQLPersistentStorage 可选能力：SQL 后端的持久化存储实现。
//
// 框架内置的持久化后端（SQLite / MySQL）共享同一个 *sql.DB；需要关系型语义
// （逐行读写、索引过滤、范围删除）的插件/组件可通过 [SQLBackend] 探测拿到
// 连接与方言，自行建表与 CRUD——与 adapter.QQExt / bot.QQ 的可选能力探测
// 惯例一致。非 SQL 后端（自定义 PersistentStorage 实现）不实现该接口，
// 探测方应回退到纯 KV 方案，功能不缺失。
type SQLPersistentStorage interface {
	PersistentStorage
	// SQLDB 返回后端共享的数据库连接。Clone 出的任意层级子存储与根存储
	// 共享同一连接；调用方不应关闭它。
	SQLDB() *sql.DB
	// SQLDialect 返回后端方言标识，用于选择方言相关的 SQL 写法。
	SQLDialect() SQLDialect
}

// SQLBackend 探测 store 是否为 SQL 后端，是则返回共享连接与方言。
func SQLBackend(store PersistentStorage) (*sql.DB, SQLDialect, bool) {
	s, ok := store.(SQLPersistentStorage)
	if !ok {
		return nil, "", false
	}
	db := s.SQLDB()
	if db == nil {
		return nil, "", false
	}
	return db, s.SQLDialect(), true
}

// TableDDL 一张关系表的双方言建表语句。
// 语句必须幂等（CREATE TABLE/INDEX IF NOT EXISTS）；SQLite/MySQL 各为一组，
// 可包含索引等多条语句，按顺序执行。
type TableDDL struct {
	Name   string
	SQLite []string
	MySQL  []string
}

// EnsureTables 按方言逐表执行 DDL；任一语句失败即返回携带表名与方言的包装错误。
// 未知方言直接报错（调用方据此回退 KV 方案）。
func EnsureTables(ctx context.Context, db *sql.DB, dialect SQLDialect, tables ...TableDDL) error {
	for _, t := range tables {
		var stmts []string
		switch dialect {
		case SQLDialectSQLite:
			stmts = t.SQLite
		case SQLDialectMySQL:
			stmts = t.MySQL
		default:
			return fmt.Errorf("ensure table %s: 未知 SQL 方言 %q", t.Name, dialect)
		}
		for _, stmt := range stmts {
			if _, err := db.ExecContext(ctx, stmt); err != nil {
				return fmt.Errorf("ensure table %s (%s): %w", t.Name, dialect, err)
			}
		}
	}
	return nil
}
