package oplog

import (
	"database/sql"
	"testing"

	"github.com/jeanhua/AniaBot/common/storage"

	_ "modernc.org/sqlite"
)

// sqlFakeStore 包装 fakeStore 并附加 SQL 能力，使 Init() 探测走 SQL 后端。
type sqlFakeStore struct {
	*fakeStore
	db *sql.DB
}

func (s *sqlFakeStore) SQLDB() *sql.DB                 { return s.db }
func (s *sqlFakeStore) SQLDialect() storage.SQLDialect { return storage.SQLDialectSQLite }

// initSQL 构造走 SQL 后端的操作日志存储（sqlite :memory:，单连接）。
func initSQL(t *testing.T, maxEntries int) {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	Init(&sqlFakeStore{fakeStore: newFakeStore(), db: db}, maxEntries, nil)
}

func TestSQLBackendRecordQuery(t *testing.T) {
	initSQL(t, 10)

	e1 := Record(CategoryAuth, "login", "面板登录成功")
	e2 := Record(CategoryConfig, "config_update", "面板更新配置: plugin.ai_chat_bot.model")

	if e1.ID == "" || e2.ID == "" || e1.ID == e2.ID {
		t.Fatalf("ID 分配异常: %q %q", e1.ID, e2.ID)
	}

	all := Query(Filter{})
	if len(all) != 2 || all[0].ID != e2.ID || all[1].ID != e1.ID {
		t.Fatalf("查询顺序异常（应新在前）: %+v", all)
	}
	if all[1].Detail != e1.Detail || all[1].Category != CategoryAuth {
		t.Fatalf("payload 往返异常: %+v", all[1])
	}

	// 分类下推 + 终判
	authOnly := Query(Filter{Category: CategoryAuth})
	if len(authOnly) != 1 || authOnly[0].ID != e1.ID {
		t.Fatalf("分类过滤异常: %+v", authOnly)
	}

	// 关键词下推（大小写不敏感）
	kw := Query(Filter{Keyword: "CONFIG_UPDATE"})
	if len(kw) != 1 || kw[0].ID != e2.ID {
		t.Fatalf("关键词过滤异常: %+v", kw)
	}

	// 游标分页
	page := Query(Filter{Before: e2.ID})
	if len(page) != 1 || page[0].ID != e1.ID {
		t.Fatalf("游标分页异常: %+v", page)
	}
}

func TestSQLBackendEvict(t *testing.T) {
	initSQL(t, 3)
	for i := 0; i < 5; i++ {
		Record(CategorySystem, "start", "启动")
	}
	all := Query(Filter{})
	if len(all) != 3 {
		t.Fatalf("容量淘汰异常：期望 3 条，实际 %d", len(all))
	}
}
