package storage

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

// kvStub 最小 PersistentStorage 实现（非 SQL 后端），用于探测回退路径。
type kvStub struct {
	PersistentStorage
}

// sqlStub 实现了 SQLPersistentStorage 的测试桩。
type sqlStub struct {
	PersistentStorage
	db      *sql.DB
	dialect SQLDialect
}

func (s sqlStub) SQLDB() *sql.DB         { return s.db }
func (s sqlStub) SQLDialect() SQLDialect { return s.dialect }

// openMemorySQLite 打开内存 SQLite；必须单连接，否则每个连接各见一个独立的内存库。
func openMemorySQLite(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	return db
}

func TestSQLBackendProbe(t *testing.T) {
	if _, _, ok := SQLBackend(kvStub{}); ok {
		t.Fatal("非 SQL 后端不应探测成功")
	}
	if _, _, ok := SQLBackend(sqlStub{db: nil, dialect: SQLDialectSQLite}); ok {
		t.Fatal("nil 连接不应探测成功")
	}
	db := openMemorySQLite(t)
	gotDB, dialect, ok := SQLBackend(sqlStub{db: db, dialect: SQLDialectMySQL})
	if !ok || gotDB != db || dialect != SQLDialectMySQL {
		t.Fatalf("SQLBackend = %v,%q,%v", gotDB, dialect, ok)
	}
}

func TestEnsureTables(t *testing.T) {
	ctx := context.Background()
	db := openMemorySQLite(t)

	ddl := TableDDL{
		Name: "ania_test",
		SQLite: []string{
			`CREATE TABLE IF NOT EXISTS ania_test (id INTEGER PRIMARY KEY, val TEXT NOT NULL)`,
			`CREATE INDEX IF NOT EXISTS idx_ania_test_val ON ania_test(val)`,
		},
		MySQL: []string{
			`CREATE TABLE IF NOT EXISTS ania_test (id BIGINT PRIMARY KEY, val MEDIUMTEXT NOT NULL) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		},
	}

	// 幂等：重复执行不报错
	for i := range 2 {
		if err := EnsureTables(ctx, db, SQLDialectSQLite, ddl); err != nil {
			t.Fatalf("EnsureTables 第 %d 次执行失败: %v", i+1, err)
		}
	}

	if _, err := db.ExecContext(ctx, `INSERT INTO ania_test (id, val) VALUES (1, 'a')`); err != nil {
		t.Fatalf("insert: %v", err)
	}
	var val string
	if err := db.QueryRowContext(ctx, `SELECT val FROM ania_test WHERE id = 1`).Scan(&val); err != nil || val != "a" {
		t.Fatalf("select = %q,%v", val, err)
	}
}

func TestEnsureTablesErrors(t *testing.T) {
	ctx := context.Background()
	db := openMemorySQLite(t)

	// 未知方言
	err := EnsureTables(ctx, db, "postgres", TableDDL{Name: "t1"})
	if err == nil || !strings.Contains(err.Error(), "t1") {
		t.Fatalf("未知方言应报含表名的错误: %v", err)
	}

	// 非法 SQL：错误需携带表名与方言
	err = EnsureTables(ctx, db, SQLDialectSQLite, TableDDL{
		Name:   "ania_bad",
		SQLite: []string{`CREATE TABLE 语法错误`},
	})
	if err == nil || !strings.Contains(err.Error(), "ania_bad") || !strings.Contains(err.Error(), "sqlite") {
		t.Fatalf("错误应含表名与方言: %v", err)
	}
}
