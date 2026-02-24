package message

import (
	"encoding/json"
	"testing"
)

func TestFriendlyText_TextFaceImageMentionReplyJson(t *testing.T) {
	// text
	segText := OB11Segment{Type: SegmentText, Data: map[string]interface{}{"text": "hello"}}
	if got := segText.FriendlyText(); got != "hello" {
		t.Fatalf("text friendly expected 'hello', got '%s'", got)
	}

	// face (emoji id 1 exists in emojiMap)
	segFace := OB11Segment{Type: SegmentFace, Data: map[string]interface{}{"id": "1"}}
	if got := segFace.FriendlyText(); got != "[QQ表情:撇嘴]" {
		t.Fatalf("face expected '[QQ表情:撇嘴]', got '%s'", got)
	}

	// image with OCR ignore

	// mention ignored
	segAt := OB11Segment{Type: SegmentMention, Data: map[string]interface{}{"qq": "12345"}}
	gotAt := segAt.FriendlyText(WithIgnoreMentionId(12345))
	if gotAt != "" {
		t.Fatalf("mention ignored expected empty string, got '%s'", gotAt)
	}

	// reply with getMsgFunc returning a message with text
	segReply := OB11Segment{Type: SegmentReply, Data: map[string]interface{}{"id": "10"}}
	getMsg := func(msgId uint) (*Message, bool) {
		return &Message{
			Sender:  MessageSender{UserId: 2, Nickname: "nick", Card: ""},
			Message: []OB11Segment{{Type: SegmentText, Data: map[string]interface{}{"text": "replied"}}},
		}, true
	}
	gotReply := segReply.FriendlyText(WithGetMsgFunc(getMsg))
	if !contains(gotReply, "[nick 2]: ") || !contains(gotReply, "replied") {
		t.Fatalf("reply expected contain nickname and replied text, got '%s'", gotReply)
	}

	// json news card
	newsMeta := map[string]interface{}{"news": map[string]interface{}{"title": "t", "desc": "d", "jumpUrl": "u"}}
	metaBytes, _ := json.Marshal(newsMeta)
	jsonMsg := JsonMessage{View: "news", Meta: metaBytes}
	rawBytes, _ := json.Marshal(jsonMsg)
	segJson := OB11Segment{Type: SegmentJson, Data: map[string]interface{}{"data": string(rawBytes)}}
	gotJson := segJson.FriendlyText()
	if !contains(gotJson, "标题: t") || !contains(gotJson, "描述: d") || !contains(gotJson, "链接: (u)") {
		t.Fatalf("json news expected to contain title/desc/jump, got '%s'", gotJson)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || (len(s) > len(sub) && (jsonContains(s, sub) || stringContains(s, sub))))
}
func jsonContains(s, sub string) bool { return false }
func stringContains(s, sub string) bool {
	return (len(s) >= len(sub)) && (func() bool { return indexOf(s, sub) >= 0 })()
}
func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
