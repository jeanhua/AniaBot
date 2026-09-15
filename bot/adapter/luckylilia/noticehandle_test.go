package luckylilia

import (
	"testing"

	"github.com/jeanhua/AniaBot/common/adapter"
	"github.com/jeanhua/AniaBot/common/model/message"
)

// TestHandleNoticeRelilRewrite 验证通知事件解析后 ID 统一改写为 lil: 前缀
// （QID 反序列化对纯数字自动补 qq:，必须在适配器边界替换为本平台前缀）。
func TestHandleNoticeRelilRewrite(t *testing.T) {
	var got message.GroupRecallNotice
	trigger := adapter.TriggerWrapper{
		OnGroupRecall: func(n message.GroupRecallNotice) { got = n },
	}
	raw := []byte(`{
		"post_type": "notice",
		"notice_type": "group_recall",
		"self_id": 333,
		"group_id": 222,
		"user_id": 111,
		"operator_id": 444,
		"message_id": 1001
	}`)
	handleNotice(trigger, "group_recall", raw)

	if got.GroupId != "lil:222" || got.UserId != "lil:111" ||
		got.OperatorId != "lil:444" || got.MessageId != "lil:1001" || got.SelfId != "lil:333" {
		t.Fatalf("通知 ID 应改写为 lil: 前缀, got %+v", got)
	}
}

// TestHandleNoticePokeNilGroup 私聊戳一戳无 group_id，改写不应产生空指针问题。
func TestHandleNoticePokeNilGroup(t *testing.T) {
	var got message.PokeNotice
	trigger := adapter.TriggerWrapper{
		OnPoke: func(n message.PokeNotice) { got = n },
	}
	raw := []byte(`{
		"post_type": "notice",
		"notice_type": "poke",
		"self_id": 333,
		"user_id": 111,
		"target_id": 333
	}`)
	handleNotice(trigger, "poke", raw)

	if got.UserId != "lil:111" || got.TargetId != "lil:333" {
		t.Fatalf("戳一戳 ID 应改写为 lil: 前缀, got %+v", got)
	}
	if got.GroupId != nil {
		t.Fatalf("私聊戳一戳不应有 group_id, got %v", *got.GroupId)
	}
}
