package oplog

import (
	"context"
	"log/slog"
	"slices"
	"strconv"
	"strings"

	"github.com/jeanhua/AniaBot/common/storage"
)

// kvBackend KV 版日志存储（回退方案）：每条日志独立占用一个 KV 记录
// （key 为 e:<序号>），写入 O(1)；序号计数器持久化在 seq 键。
type kvBackend struct {
	store  storage.PersistentStorage
	logger *slog.Logger
}

const (
	entryKeyPrefix = "e:"  // 逐条日志的键前缀：e:<序号>
	seqKey         = "seq" // 自增序号（十进制）
)

func newKVBackend(store storage.PersistentStorage, logger *slog.Logger) *kvBackend {
	return &kvBackend{store: store, logger: logger}
}

// entryKey 由序号生成日志记录键。
func entryKey(seq uint64) string {
	return entryKeyPrefix + strconv.FormatUint(seq, 10)
}

func (b *kvBackend) maxSeq() uint64 {
	s, ok := b.store.GetString(context.Background(), seqKey)
	if !ok {
		return 0
	}
	n, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0
	}
	return n
}

func (b *kvBackend) insert(seq uint64, e Entry) {
	ctx := context.Background()
	if !b.store.Set(ctx, entryKey(seq), e) {
		b.logger.Error("oplog 落盘失败", "id", e.ID)
	}
	b.store.SetString(ctx, seqKey, strconv.FormatUint(seq, 10))
}

func (b *kvBackend) evict(maxSeq uint64, maxEntries int) {
	seqs := b.listSeqs()
	if len(seqs) <= maxEntries {
		return
	}
	ctx := context.Background()
	for _, n := range seqs[:len(seqs)-maxEntries] {
		b.store.Del(ctx, entryKey(n))
	}
}

func (b *kvBackend) query(f Filter, beforeSeq uint64, limit int) []Entry {
	seqs := b.listSeqs() // 升序，新在后
	capacity := limit
	if capacity <= 0 || capacity > len(seqs) {
		capacity = len(seqs)
	}
	out := make([]Entry, 0, capacity)
	for i := len(seqs) - 1; i >= 0; i-- {
		if beforeSeq > 0 && seqs[i] >= beforeSeq {
			continue
		}
		e, ok := b.load(seqs[i])
		if !ok {
			continue
		}
		if !f.match(e) {
			continue
		}
		out = append(out, e)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

func (b *kvBackend) load(seq uint64) (Entry, bool) {
	var e Entry
	if !b.store.Get(context.Background(), entryKey(seq), &e) {
		return Entry{}, false
	}
	return e, true
}

// listSeqs 返回全部日志记录的序号，按升序排列（旧在前）。
func (b *kvBackend) listSeqs() []uint64 {
	keys, err := b.store.Keys(context.Background(), entryKeyPrefix)
	if err != nil {
		b.logger.Error("oplog 列举键失败", "err", err)
		return nil
	}
	seqs := make([]uint64, 0, len(keys))
	for _, k := range keys {
		n, err := strconv.ParseUint(strings.TrimPrefix(k, entryKeyPrefix), 10, 64)
		if err != nil {
			continue
		}
		seqs = append(seqs, n)
	}
	slices.Sort(seqs)
	return seqs
}
