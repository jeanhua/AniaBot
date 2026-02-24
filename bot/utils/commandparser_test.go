package utils

import (
	"testing"

	"github.com/jeanhua/AniaBot/common/model/message"
)

func TestParseCommand(t *testing.T) {
	// no message
	msg := message.Message{Message: []message.OB11Segment{}}
	cmd := ParseCommand(msg)
	if cmd.Name != "" || cmd.Mention {
		t.Fatalf("expected empty command, got %+v", cmd)
	}

	// not a command
	msg = message.Message{Message: []message.OB11Segment{{Type: "text", Data: map[string]interface{}{"text": "hello"}}}}
	cmd = ParseCommand(msg)
	if cmd.Name != "" || len(cmd.Args) != 0 {
		t.Fatalf("expected no command, got %+v", cmd)
	}

	// command with args
	msg = message.Message{Message: []message.OB11Segment{{Type: "text", Data: map[string]interface{}{"text": "/say hi there"}}}}
	cmd = ParseCommand(msg)
	if cmd.Name != "say" || len(cmd.Args) != 2 || cmd.Args[0] != "hi" {
		t.Fatalf("unexpected parsed command %+v", cmd)
	}

	// mention detection
	self := uint(999)
	msg = message.Message{SelfId: self, Message: []message.OB11Segment{{Type: "at", Data: map[string]interface{}{"qq": "999"}}}}
	cmd = ParseCommand(msg)
	if !cmd.Mention {
		t.Fatalf("expected mention true, got %+v", cmd)
	}
}
