package tasklog

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/jeanhua/AniaBot/common/storage"
)

// fakeStore 进程内 PersistentStorage 实现，仅用于单测。
type fakeStore struct {
	data map[string]string
}

func newFakeStore() *fakeStore { return &fakeStore{data: map[string]string{}} }

func (s *fakeStore) GetString(_ context.Context, key string) (string, bool) {
	v, ok := s.data[key]
	return v, ok
}
func (s *fakeStore) SetString(_ context.Context, key, val string) bool {
	s.data[key] = val
	return true
}
func (s *fakeStore) Get(ctx context.Context, key string, out any) bool {
	v, ok := s.data[key]
	if !ok {
		return false
	}
	return json.Unmarshal([]byte(v), out) == nil
}
func (s *fakeStore) Set(ctx context.Context, key string, val any) bool {
	b, err := json.Marshal(val)
	if err != nil {
		return false
	}
	s.data[key] = string(b)
	return true
}
func (s *fakeStore) Has(_ context.Context, key string) bool { _, ok := s.data[key]; return ok }
func (s *fakeStore) Del(_ context.Context, key string) bool {
	delete(s.data, key)
	return true
}
func (s *fakeStore) Keys(_ context.Context, prefix string) ([]string, error) {
	var keys []string
	for k := range s.data {
		if prefix == "" || strings.HasPrefix(k, prefix) {
			keys = append(keys, k)
		}
	}
	return keys, nil
}
func (s *fakeStore) Clear(_ context.Context) bool                  { s.data = map[string]string{}; return true }
func (s *fakeStore) Clone(prefix string) storage.PersistentStorage { return s } // 测试不复用前缀

func TestRecordAndRecent(t *testing.T) {
	l := New(newFakeStore(), 3, nil)

	l.Record(Entry{TaskID: "t1", Status: StatusSuccess, TaskTitle: "A"})
	l.Record(Entry{TaskID: "t2", Status: StatusTimeout, TaskTitle: "B"})
	l.Record(Entry{TaskID: "t3", Status: StatusError, TaskTitle: "C"})

	got := l.Recent(10)
	if len(got) != 3 {
		t.Fatalf("want 3 entries, got %d", len(got))
	}
	// newest first：最后写入的 t3 应在最前
	if got[0].TaskID != "t3" || got[2].TaskID != "t1" {
		t.Fatalf("order wrong: %+v", got)
	}
	// ID 自增
	if got[0].ID == got[1].ID || got[1].ID == got[2].ID {
		t.Fatalf("ids not unique: %+v", got)
	}
}

func TestRollingCap(t *testing.T) {
	l := New(newFakeStore(), 2, nil)
	for i := 0; i < 5; i++ {
		l.Record(Entry{TaskID: "t", Status: StatusSuccess})
	}
	got := l.Recent(0)
	if len(got) != 2 {
		t.Fatalf("want capped to 2, got %d", len(got))
	}
}

func TestUpdate(t *testing.T) {
	l := New(newFakeStore(), 10, nil)
	e := l.Record(Entry{TaskID: "t1", Status: StatusRunning})
	l.Update(e.ID, func(en *Entry) {
		en.Status = StatusSuccess
		en.DurationMs = 42
	})
	for _, x := range l.Recent(0) {
		if x.ID == e.ID {
			if x.Status != StatusSuccess || x.DurationMs != 42 {
				t.Fatalf("update not applied: %+v", x)
			}
			return
		}
	}
	t.Fatalf("entry %s not found after update", e.ID)
}

func TestMigrateLegacyEntries(t *testing.T) {
	store := newFakeStore()
	// 模拟旧版数据：entries 键存整体数组，ID 为序号 base36
	legacy := []Entry{
		{ID: "2", TaskID: "b", Status: StatusSuccess},
		{ID: "1", TaskID: "a", Status: StatusError},
	}
	store.Set(context.Background(), "entries", legacy)
	store.SetString(context.Background(), "seq", "2")

	l := New(store, 10, nil)

	if store.Has(context.Background(), "entries") {
		t.Fatal("迁移后旧 entries 键应被删除")
	}
	recent := l.Recent(0)
	if len(recent) != 2 || recent[0].TaskID != "b" || recent[1].TaskID != "a" {
		t.Fatalf("迁移后数据异常: %+v", recent)
	}

	// 序号应延续，新记录不与旧记录冲突
	e := l.Record(Entry{TaskID: "c", Status: StatusSuccess})
	if e.ID == "1" || e.ID == "2" {
		t.Fatalf("迁移后 ID 冲突: %q", e.ID)
	}

	// Update 应能命中迁移过来的记录
	l.Update("1", func(en *Entry) { en.Status = StatusSuccess })
	for _, en := range l.Recent(0) {
		if en.ID == "1" && en.Status != StatusSuccess {
			t.Fatalf("迁移记录的 Update 未生效: %+v", en)
		}
	}
}

func TestRecentForTask(t *testing.T) {
	l := New(newFakeStore(), 10, nil)
	l.Record(Entry{TaskID: "a", Status: StatusSuccess})
	l.Record(Entry{TaskID: "b", Status: StatusSuccess})
	l.Record(Entry{TaskID: "a", Status: StatusTimeout})
	got := l.RecentForTask("a", 0)
	if len(got) != 2 {
		t.Fatalf("want 2 for task a, got %d", len(got))
	}
	if got[0].Status != StatusTimeout {
		t.Fatalf("want newest first, got %+v", got)
	}
}
