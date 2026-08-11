package napcat

import (
	"testing"

	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/jeanhua/AniaBot/common/msgchain"
)

func TestStripQQSegments(t *testing.T) {
	chain := msgchain.Builder().Group().
		Mention(message.FromUint64(123)).
		Reply(message.FromUint64(456)).
		Build()
	got := stripQQSegments(chain.GetGroupMsg())
	if got[0].Data["qq"] != "123" {
		t.Fatalf("mention = %v, want 123", got[0].Data["qq"])
	}
	if got[1].Data["id"] != "456" {
		t.Fatalf("reply = %v, want 456", got[1].Data["id"])
	}
}

func TestStripQQForward(t *testing.T) {
	inner := msgchain.Builder().Group().
		Mention(message.FromUint64(789)).
		Build()
	forwardBuilder := msgchain.Builder().GroupForward()
	forwardBuilder.Message(message.FromUint64(321), "nick", inner)
	forward := forwardBuilder.Build()
	got := stripQQForward(forward.GetForwardMsg())
	if got.Messages[0].Data.UserId != "321" {
		t.Fatalf("node user = %q, want 321", got.Messages[0].Data.UserId)
	}
	if got.Messages[0].Data.Content[0].Data["qq"] != "789" {
		t.Fatalf("nested mention = %v, want 789", got.Messages[0].Data.Content[0].Data["qq"])
	}
}
