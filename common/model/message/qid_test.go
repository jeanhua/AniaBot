package message

import (
	"encoding/json"
	"testing"
)

func TestQQIDPrefixConstructors(t *testing.T) {
	if got := FromUint64(123456); got != "qq:123456" {
		t.Fatalf("FromUint64 = %q, want qq:123456", got)
	}
	if got := FromString("789"); got != "qq:789" {
		t.Fatalf("FromString numeric = %q, want qq:789", got)
	}
	if got := FromString("fs:ou_abc"); got != "fs:ou_abc" {
		t.Fatalf("FromString non-QQ = %q", got)
	}
	if got := FromString("qq:123"); got != "qq:123" {
		t.Fatalf("FromString prefixed = %q", got)
	}
	if got := FromUint64(123).Uint64(); got != 123 {
		t.Fatalf("Uint64 = %d, want 123", got)
	}
	if got := FromUint64(123).TrimQQPrefix(); got != "123" {
		t.Fatalf("TrimQQPrefix = %q, want 123", got)
	}
}

func TestQIDUnmarshalJSONNormalizesQQ(t *testing.T) {
	var q QID
	if err := json.Unmarshal([]byte(`"123"`), &q); err != nil || q != "qq:123" {
		t.Fatalf("unmarshal quoted = %q, %v", q, err)
	}
	if err := json.Unmarshal([]byte(`456`), &q); err != nil || q != "qq:456" {
		t.Fatalf("unmarshal number = %q, %v", q, err)
	}
	if err := json.Unmarshal([]byte(`"fs:ou_abc"`), &q); err != nil || q != "fs:ou_abc" {
		t.Fatalf("unmarshal non-QQ = %q, %v", q, err)
	}
}

func TestNormalizeQQMessage(t *testing.T) {
	msg := Message{
		MessageId: QID("1"),
		UserId:    QID("2"),
		GroupId:   QID("3"),
		SelfId:    QID("4"),
		Sender:    MessageSender{UserId: QID("5")},
		Message: []OB11Segment{
			{Type: SegmentMention, Data: map[string]any{"qq": "6"}},
			{Type: SegmentReply, Data: map[string]any{"id": "7"}},
			{Type: SegmentMention, Data: map[string]any{"qq": "all"}},
		},
	}
	NormalizeQQMessage(&msg)
	if msg.MessageId != "qq:1" || msg.UserId != "qq:2" || msg.GroupId != "qq:3" ||
		msg.SelfId != "qq:4" || msg.Sender.UserId != "qq:5" {
		t.Fatalf("message QID not normalized: %+v", msg)
	}
	if got := msg.Message[0].Data["qq"]; got != "qq:6" {
		t.Fatalf("mention qq = %v, want qq:6", got)
	}
	if got := msg.Message[1].Data["id"]; got != "qq:7" {
		t.Fatalf("reply id = %v, want qq:7", got)
	}
	if got := msg.Message[2].Data["qq"]; got != "all" {
		t.Fatalf("all mention should stay all, got %v", got)
	}
}
