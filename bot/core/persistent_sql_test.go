package core

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync/atomic"
	"testing"

	"github.com/jeanhua/AniaBot/common/storage"
	"github.com/spf13/viper"
)

// testDiscardLogger 测试用丢弃日志，避免污染输出。
func testDiscardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

var testNSCounter uint64

// makeSqliteStore 返回一个全新的内存 SQLite 持久化存储（根命名空间为 ""，每个用例相互独立）。
func makeSqliteStore(t *testing.T) storage.PersistentStorage {
	t.Helper()
	s, err := NewAniaSqliteStorage(context.Background(), ":memory:", testDiscardLogger())
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	return s
}

// makeMysqlStore 返回一个带唯一命名空间前缀的 MySQL 持久化存储（仅在 ANIA_MYSQL_DSN 时运行）。
func makeMysqlStore(t *testing.T) storage.PersistentStorage {
	t.Helper()
	dsn := os.Getenv("ANIA_MYSQL_DSN")
	if dsn == "" {
		t.Skip("设置 ANIA_MYSQL_DSN 以运行 MySQL 一致性测试")
	}
	root, err := NewAniaMysqlStorage(context.Background(), dsn, MysqlPoolConfig{MaxOpenConns: 4}, testDiscardLogger())
	if err != nil {
		t.Fatalf("open mysql: %v", err)
	}
	id := atomic.AddUint64(&testNSCounter, 1)
	store := root.Clone(fmt.Sprintf("t%d", id))
	t.Cleanup(func() { store.Clear(context.Background()) })
	return store
}

// testPersistentConformance 对任意 PersistentStorage 实现执行一致性校验。
// 适用于 SQLite 与 MySQL 两个后端，确保行为一致。
func testPersistentConformance(t *testing.T, store storage.PersistentStorage) {
	ctx := context.Background()

	t.Run("StringRoundTrip", func(t *testing.T) {
		if !store.SetString(ctx, "k1", "v1") {
			t.Fatal("SetString returned false")
		}
		v, ok := store.GetString(ctx, "k1")
		if !ok || v != "v1" {
			t.Fatalf("GetString = %q,%v want v1,true", v, ok)
		}
	})

	t.Run("MissingKey", func(t *testing.T) {
		if _, ok := store.GetString(ctx, "does-not-exist"); ok {
			t.Fatal("expected GetString missing => false")
		}
		if store.Has(ctx, "does-not-exist") {
			t.Fatal("expected Has missing => false")
		}
	})

	t.Run("Overwrite", func(t *testing.T) {
		store.SetString(ctx, "ow", "a")
		store.SetString(ctx, "ow", "b")
		v, _ := store.GetString(ctx, "ow")
		if v != "b" {
			t.Fatalf("overwrite = %q want b", v)
		}
	})

	t.Run("JSONRoundTrip", func(t *testing.T) {
		type S struct {
			N int
			T string
		}
		if !store.Set(ctx, "obj", S{N: 42, T: "hi"}) {
			t.Fatal("Set returned false")
		}
		var got S
		if !store.Get(ctx, "obj", &got) {
			t.Fatal("Get returned false")
		}
		if got.N != 42 || got.T != "hi" {
			t.Fatalf("Get = %+v want {42 hi}", got)
		}
	})

	t.Run("Has", func(t *testing.T) {
		store.SetString(ctx, "h", "1")
		if !store.Has(ctx, "h") {
			t.Fatal("expected Has true")
		}
	})

	t.Run("Del", func(t *testing.T) {
		store.SetString(ctx, "d", "1")
		if !store.Del(ctx, "d") {
			t.Fatal("Del returned false")
		}
		if store.Has(ctx, "d") {
			t.Fatal("expected key gone after Del")
		}
		// 删除不存在的键也应返回 true（无错误）
		if !store.Del(ctx, "d") {
			t.Fatal("Del of missing key should return true")
		}
	})

	t.Run("Keys", func(t *testing.T) {
		sub := store.Clone("keys")
		sub.SetString(ctx, "a", "1")
		sub.SetString(ctx, "b", "2")
		sub.SetString(ctx, "pref1", "3")
		sub.SetString(ctx, "pref2", "4")

		pref, err := sub.Keys(ctx, "pref")
		if err != nil {
			t.Fatal(err)
		}
		if len(pref) != 2 || pref[0] != "pref1" || pref[1] != "pref2" {
			t.Fatalf("Keys(pref) = %v want [pref1 pref2]", pref)
		}

		all, err := sub.Keys(ctx, "")
		if err != nil {
			t.Fatal(err)
		}
		if len(all) != 4 {
			t.Fatalf("Keys() = %v want 4 keys", all)
		}
		// 返回的键可直接回传给 Get
		for _, k := range all {
			if _, ok := sub.GetString(ctx, k); !ok {
				t.Fatalf("returned key %q not readable", k)
			}
		}
	})

	t.Run("CloneIsolation", func(t *testing.T) {
		a := store.Clone("isoA")
		b := store.Clone("isoB")
		a.SetString(ctx, "x", "fromA")
		b.SetString(ctx, "x", "fromB")
		va, _ := a.GetString(ctx, "x")
		vb, _ := b.GetString(ctx, "x")
		if va != "fromA" || vb != "fromB" {
			t.Fatalf("clone leak: a=%q b=%q", va, vb)
		}
		b.SetString(ctx, "onlyB", "1")
		if a.Has(ctx, "onlyB") {
			t.Fatal("sibling clone should not see other's key")
		}
	})

	t.Run("ClearSubtree", func(t *testing.T) {
		root := store.Clone("clear")
		root.SetString(ctx, "k", "1")
		sub := root.Clone("sub")
		sub.SetString(ctx, "k2", "2")
		if !root.Clear(ctx) {
			t.Fatal("Clear returned false")
		}
		if root.Has(ctx, "k") {
			t.Fatal("root key not cleared")
		}
		if sub.Has(ctx, "k2") {
			t.Fatal("sub-namespace key not cleared by root Clear")
		}
	})

	t.Run("NamespaceIsolation", func(t *testing.T) {
		p1 := store.Clone("plugin1")
		p2 := store.Clone("plugin2")
		p1.SetString(ctx, "shared", "p1")
		p2.SetString(ctx, "shared", "p2")
		v1, _ := p1.GetString(ctx, "shared")
		v2, _ := p2.GetString(ctx, "shared")
		if v1 != "p1" || v2 != "p2" {
			t.Fatalf("namespace collision: p1=%q p2=%q", v1, v2)
		}
	})

	t.Run("CaseSensitiveNamespaces", func(t *testing.T) {
		// 仅大小写不同的命名空间不得冲突（要求字节序/大小写敏感排序，
		// MySQL 列需声明 COLLATE utf8mb4_bin，SQLite TEXT 默认 BINARY）。
		upper := store.Clone("CaseNS")
		lower := store.Clone("casens")
		upper.SetString(ctx, "k", "UPPER")
		lower.SetString(ctx, "k", "lower")
		vu, _ := upper.GetString(ctx, "k")
		vl, _ := lower.GetString(ctx, "k")
		if vu != "UPPER" || vl != "lower" {
			t.Fatalf("case collision: upper=%q lower=%q", vu, vl)
		}
	})
}

func TestSqlitePersistent_Conformance(t *testing.T) {
	testPersistentConformance(t, makeSqliteStore(t))
}

func TestMysqlPersistent_Conformance(t *testing.T) {
	testPersistentConformance(t, makeMysqlStore(t))
}

// 根命名空间（""）的 Clear 应清空整表，包括所有子命名空间。
func TestSqlitePersistent_RootClearWipesAll(t *testing.T) {
	ctx := context.Background()
	root := makeSqliteStore(t) // namespace == ""
	root.SetString(ctx, "k", "1")
	root.Clone("a").SetString(ctx, "k", "1")
	root.Clone("b").SetString(ctx, "k", "1")
	if !root.Clear(ctx) {
		t.Fatal("Clear returned false")
	}
	if root.Has(ctx, "k") || root.Clone("a").Has(ctx, "k") || root.Clone("b").Has(ctx, "k") {
		t.Fatal("root Clear should wipe all namespaces")
	}
}

// prefixRange 的边界：空串与末字节 0xFF。
func TestPrefixRange(t *testing.T) {
	if lo, hi, ok := prefixRange(""); ok || lo != "" || hi != "" {
		t.Fatalf("empty prefix: lo=%q hi=%q ok=%v", lo, hi, ok)
	}
	lo, hi, ok := prefixRange("abc:")
	if !ok || lo != "abc:" || hi != "abc;" {
		t.Fatalf("abc: => lo=%q hi=%q ok=%v", lo, hi, ok)
	}
	if _, _, ok := prefixRange("x\xff"); ok {
		t.Fatal("0xFF suffix should return ok=false")
	}
}

// --- 工厂测试 ---

func TestNewCacheStorage_Memory(t *testing.T) {
	cfg := viper.New()
	cfg.Set("bot.store.cache.driver", "memory")
	s, err := newCacheStorage(context.Background(), cfg, testDiscardLogger())
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := s.(*AniaMemoryStorage); !ok {
		t.Fatalf("expected *AniaMemoryStorage, got %T", s)
	}
}

func TestNewCacheStorage_Redis(t *testing.T) {
	if addr := os.Getenv("ANIA_REDIS_ADDR"); addr == "" {
		t.Skip("设置 ANIA_REDIS_ADDR 以运行 Redis 缓存工厂测试")
	}
	cfg := viper.New()
	cfg.Set("bot.store.cache.driver", "redis")
	cfg.Set("bot.store.cache.redis.address", os.Getenv("ANIA_REDIS_ADDR"))
	if _, err := newCacheStorage(context.Background(), cfg, testDiscardLogger()); err != nil {
		t.Skipf("redis 不可用: %v", err)
	}
}

func TestNewCacheStorage_UnknownDriver(t *testing.T) {
	cfg := viper.New()
	cfg.Set("bot.store.cache.driver", "bogus")
	if _, err := newCacheStorage(context.Background(), cfg, testDiscardLogger()); err == nil {
		t.Fatal("expected error for unknown cache driver")
	}
}

func TestNewPersistentStorage_DefaultSqlite(t *testing.T) {
	cfg := viper.New()
	cfg.Set("bot.store.persistent.sqlite.path", ":memory:")
	s, err := newPersistentStorage(context.Background(), cfg, testDiscardLogger())
	if err != nil {
		t.Fatal(err)
	}
	// 默认驱动应为 sqlite，且可正常读写
	if !s.SetString(context.Background(), "k", "v") {
		t.Fatal("SetString failed")
	}
}

func TestNewPersistentStorage_UnknownDriver(t *testing.T) {
	cfg := viper.New()
	cfg.Set("bot.store.persistent.driver", "bogus")
	if _, err := newPersistentStorage(context.Background(), cfg, testDiscardLogger()); err == nil {
		t.Fatal("expected error for unknown persistent driver")
	}
}
