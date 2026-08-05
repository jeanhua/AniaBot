package pluginaichat

import (
	"context"
	"database/sql"
	"testing"

	"github.com/jeanhua/AniaBot/bot/component/aichat"
	"github.com/jeanhua/AniaBot/bot/component/llmtool"
	"github.com/jeanhua/AniaBot/common/storage"

	_ "modernc.org/sqlite"
)

// newTestSQLDB 打开内存 SQLite 并建好对话历史表；必须单连接，
// 否则每个连接各见一个独立的内存库。
func newTestSQLDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	db.SetMaxOpenConns(1)
	if err := storage.EnsureTables(context.Background(), db, storage.SQLDialectSQLite, chatHistoryTables...); err != nil {
		db.Close()
		t.Fatalf("ensure tables: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func historyTestMessages() []aichat.Message {
	return []aichat.Message{
		aichat.TextMessage(aichat.RoleUser, "你好"),
		{
			Role: aichat.RoleAssistant,
			Parts: []aichat.ContentPart{
				aichat.TextPart("这是图片"),
				aichat.TextPart("[图片 ab12cd34]"),
			},
			ToolCalls:        []llmtool.ToolCall{{ID: "call_1", Name: "get_time", Arguments: `{}`}},
			ReasoningContent: "推理过程",
		},
		{Role: aichat.RoleTool, ToolCallID: "call_1", Parts: []aichat.ContentPart{aichat.TextPart("工具结果")}},
	}
}

// assertMessagesEqual 逐条比较消息内容（DeepEqual 对 nil/空切片敏感，逐字段断言更直观）。
func assertMessagesEqual(t *testing.T, got, want []aichat.Message) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("消息数 = %d, want %d\n got: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i].Role != want[i].Role ||
			got[i].ToolCallID != want[i].ToolCallID ||
			got[i].ReasoningContent != want[i].ReasoningContent {
			t.Fatalf("第 %d 条消息字段不符:\n got %+v\nwant %+v", i, got[i], want[i])
		}
		if len(got[i].Parts) != len(want[i].Parts) {
			t.Fatalf("第 %d 条消息 Parts 数不符:\n got %+v\nwant %+v", i, got[i].Parts, want[i].Parts)
		}
		for j := range want[i].Parts {
			if got[i].Parts[j] != want[i].Parts[j] {
				t.Fatalf("第 %d 条消息 Part %d 不符: got %+v want %+v", i, j, got[i].Parts[j], want[i].Parts[j])
			}
		}
		if len(got[i].ToolCalls) != len(want[i].ToolCalls) {
			t.Fatalf("第 %d 条消息 ToolCalls 数不符: got %+v want %+v", i, got[i].ToolCalls, want[i].ToolCalls)
		}
		for j := range want[i].ToolCalls {
			if got[i].ToolCalls[j] != want[i].ToolCalls[j] {
				t.Fatalf("第 %d 条消息 ToolCall %d 不符: got %+v want %+v", i, j, got[i].ToolCalls[j], want[i].ToolCalls[j])
			}
		}
	}
}

func TestSQLHistoryStoreAppendLoadRoundTrip(t *testing.T) {
	ctx := context.Background()
	db := newTestSQLDB(t)
	s := newSQLHistoryStore(db, "g:1001", testLogger())

	msgs := historyTestMessages()
	s.Append(ctx, msgs[:1])
	s.Append(ctx, msgs[1:])

	got, err := s.Load(ctx)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	assertMessagesEqual(t, got, msgs)

	// 会话计数与行序正确
	var count int
	if err := db.QueryRowContext(ctx,
		`SELECT msg_count FROM ania_chat_session WHERE session_id = 'g:1001'`).Scan(&count); err != nil || count != 3 {
		t.Fatalf("msg_count = %d,%v want 3", count, err)
	}
	rows, err := db.QueryContext(ctx,
		`SELECT seq, role FROM ania_chat_message WHERE session_id = 'g:1001' ORDER BY seq`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	wantRoles := []string{"user", "assistant", "tool"}
	i := 0
	for rows.Next() {
		var seq int
		var role string
		if err := rows.Scan(&seq, &role); err != nil {
			t.Fatal(err)
		}
		if seq != i || role != wantRoles[i] {
			t.Fatalf("第 %d 行 seq=%d role=%q, want seq=%d role=%q", i, seq, role, i, wantRoles[i])
		}
		i++
	}
	if i != 3 {
		t.Fatalf("消息行数 = %d, want 3", i)
	}
}

func TestSQLHistoryStoreSessionIsolation(t *testing.T) {
	ctx := context.Background()
	db := newTestSQLDB(t)
	a := newSQLHistoryStore(db, "g:1", testLogger())
	b := newSQLHistoryStore(db, "f:1", testLogger())

	a.Append(ctx, []aichat.Message{aichat.TextMessage(aichat.RoleUser, "群消息")})
	b.Append(ctx, []aichat.Message{aichat.TextMessage(aichat.RoleUser, "私聊消息")})

	gotA, _ := a.Load(ctx)
	gotB, _ := b.Load(ctx)
	if len(gotA) != 1 || gotA[0].Parts[0].Text != "群消息" {
		t.Fatalf("群会话历史串扰: %+v", gotA)
	}
	if len(gotB) != 1 || gotB[0].Parts[0].Text != "私聊消息" {
		t.Fatalf("私聊会话历史串扰: %+v", gotB)
	}
	// 各自序号独立从 0 开始
	var seqA, seqB int
	db.QueryRowContext(ctx, `SELECT seq FROM ania_chat_message WHERE session_id = 'g:1'`).Scan(&seqA)
	db.QueryRowContext(ctx, `SELECT seq FROM ania_chat_message WHERE session_id = 'f:1'`).Scan(&seqB)
	if seqA != 0 || seqB != 0 {
		t.Fatalf("序号应各自从 0 开始: g=%d f=%d", seqA, seqB)
	}
}

func TestSQLHistoryStoreReplace(t *testing.T) {
	ctx := context.Background()
	db := newTestSQLDB(t)
	s := newSQLHistoryStore(db, "g:1", testLogger())

	msgs := historyTestMessages()
	s.Append(ctx, msgs)
	// 压缩后的历史：1 条摘要消息
	summary := []aichat.Message{aichat.TextMessage(aichat.RoleUser, "以下是之前的对话摘要：...")}
	s.Replace(ctx, summary)

	got, _ := s.Load(ctx)
	assertMessagesEqual(t, got, summary)

	// seq 从 0 重排、计数重置、无旧行残留
	var count, rowCount int
	db.QueryRowContext(ctx, `SELECT msg_count FROM ania_chat_session WHERE session_id = 'g:1'`).Scan(&count)
	db.QueryRowContext(ctx, `SELECT COUNT(*) FROM ania_chat_message WHERE session_id = 'g:1'`).Scan(&rowCount)
	if count != 1 || rowCount != 1 {
		t.Fatalf("Replace 后 msg_count=%d 行数=%d, want 1/1", count, rowCount)
	}

	// Replace 后继续 Append，序号应接续新计数
	s.Append(ctx, []aichat.Message{aichat.TextMessage(aichat.RoleAssistant, "继续")})
	got, _ = s.Load(ctx)
	if len(got) != 2 {
		t.Fatalf("Replace 后 Append 历史长度 = %d, want 2", len(got))
	}
	var maxSeq int
	db.QueryRowContext(ctx, `SELECT MAX(seq) FROM ania_chat_message WHERE session_id = 'g:1'`).Scan(&maxSeq)
	if maxSeq != 1 {
		t.Fatalf("Replace 后追加的序号 = %d, want 1", maxSeq)
	}
}

func TestSQLHistoryStoreReplaceEmpty(t *testing.T) {
	ctx := context.Background()
	db := newTestSQLDB(t)
	s := newSQLHistoryStore(db, "g:1", testLogger())

	s.Append(ctx, historyTestMessages())
	// 压缩/截断到 0 条的极端场景：等效清空，但会话行保留、计数归零
	s.Replace(ctx, nil)

	got, _ := s.Load(ctx)
	if len(got) != 0 {
		t.Fatalf("Replace(nil) 后历史应空, got %+v", got)
	}
	var count int
	if err := db.QueryRowContext(ctx,
		`SELECT msg_count FROM ania_chat_session WHERE session_id = 'g:1'`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("Replace(nil) 后 msg_count = %d,%v want 0", count, err)
	}
}

func TestSQLHistoryStoreClear(t *testing.T) {
	ctx := context.Background()
	db := newTestSQLDB(t)
	s := newSQLHistoryStore(db, "g:1", testLogger())

	s.Append(ctx, historyTestMessages())
	s.Clear(ctx)

	got, _ := s.Load(ctx)
	if len(got) != 0 {
		t.Fatalf("Clear 后历史应空, got %+v", got)
	}
	var msgRows, sessRows int
	db.QueryRowContext(ctx, `SELECT COUNT(*) FROM ania_chat_message WHERE session_id = 'g:1'`).Scan(&msgRows)
	db.QueryRowContext(ctx, `SELECT COUNT(*) FROM ania_chat_session WHERE session_id = 'g:1'`).Scan(&sessRows)
	if msgRows != 0 || sessRows != 0 {
		t.Fatalf("Clear 后两表应无该会话行: messages=%d sessions=%d", msgRows, sessRows)
	}

	// Clear 后重新 Append：会话行重建、序号从 0 开始
	s.Append(ctx, []aichat.Message{aichat.TextMessage(aichat.RoleUser, "新对话")})
	got, _ = s.Load(ctx)
	if len(got) != 1 {
		t.Fatalf("Clear 后重建历史长度 = %d, want 1", len(got))
	}
	var seq int
	db.QueryRowContext(ctx, `SELECT seq FROM ania_chat_message WHERE session_id = 'g:1'`).Scan(&seq)
	if seq != 0 {
		t.Fatalf("重建后序号 = %d, want 0", seq)
	}
}

// TestHistoryStoreConformance 同一操作序列分别作用于 KV 与 SQL 实现，
// Load 结果应一致（KV 为整段 JSON 读改写，SQL 为行级增量）。
func TestHistoryStoreConformance(t *testing.T) {
	ctx := context.Background()
	kv := newPersistentHistoryStore(newPFake(), "chat:g:1", testLogger())
	sqlStore := newSQLHistoryStore(newTestSQLDB(t), "g:1", testLogger())

	msgs := historyTestMessages()
	ops := []struct {
		name string
		run  func(h aichat.HistoryStore)
	}{
		{"append1", func(h aichat.HistoryStore) { h.Append(ctx, msgs[:2]) }},
		{"append2", func(h aichat.HistoryStore) { h.Append(ctx, msgs[2:]) }},
		{"replace", func(h aichat.HistoryStore) { h.Replace(ctx, msgs[:1]) }},
		{"append3", func(h aichat.HistoryStore) { h.Append(ctx, msgs[1:2]) }},
		{"clear", func(h aichat.HistoryStore) { h.Clear(ctx) }},
		{"append4", func(h aichat.HistoryStore) { h.Append(ctx, msgs[2:]) }},
	}

	for _, op := range ops {
		op.run(kv)
		op.run(sqlStore)
		kvGot, kvErr := kv.Load(ctx)
		sqlGot, sqlErr := sqlStore.Load(ctx)
		if kvErr != nil || sqlErr != nil {
			t.Fatalf("%s: Load 出错 kv=%v sql=%v", op.name, kvErr, sqlErr)
		}
		assertMessagesEqual(t, sqlGot, kvGot)
	}
}
