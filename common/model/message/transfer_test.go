package message

import (
	"encoding/json"
	"testing"
)

func TestParseFunctions(t *testing.T) {
	// ParseText
	var tm TextMessage
	ok := ParseText(OB11Segment{Type: SegmentText, Data: map[string]any{"text": "hi"}}, &tm)
	if !ok || tm.Text != "hi" {
		t.Fatalf("ParseText failed: ok=%v text=%s", ok, tm.Text)
	}

	// ParseFace
	var fm FaceMessage
	ok = ParseFace(OB11Segment{Type: SegmentFace, Data: map[string]any{"id": "1"}}, &fm)
	if !ok || fm.Id != 1 {
		t.Fatalf("ParseFace failed: ok=%v id=%d", ok, fm.Id)
	}

	// ParseImage
	var im ImageMessage
	ok = ParseImage(OB11Segment{Type: SegmentImage, Data: map[string]any{"url": "u", "file": "f"}}, &im)
	if !ok || im.Url != "u" || im.File != "f" {
		t.Fatalf("ParseImage failed: ok=%v url=%s file=%s", ok, im.Url, im.File)
	}

	// ParseMention all
	var mm MentionMessage
	ok = ParseMention(OB11Segment{Type: SegmentMention, Data: map[string]any{"qq": "all"}}, &mm)
	if !ok || !mm.IsAll {
		t.Fatalf("ParseMention(all) failed: ok=%v isAll=%v", ok, mm.IsAll)
	}

	// ParseMention id
	ok = ParseMention(OB11Segment{Type: SegmentMention, Data: map[string]any{"qq": "123"}}, &mm)
	if !ok || mm.QQ != "123" {
		t.Fatalf("ParseMention(id) failed: ok=%v qq=%s", ok, mm.QQ)
	}

	// ParseReply
	var rm ReplyMessage
	ok = ParseReply(OB11Segment{Type: SegmentReply, Data: map[string]any{"id": "10"}}, &rm)
	if !ok || rm.Id != "10" {
		t.Fatalf("ParseReply failed: ok=%v id=%s", ok, rm.Id)
	}

	// ParseVideo
	var vm VideoMessage
	ok = ParseVideo(OB11Segment{Type: SegmentVideo, Data: map[string]any{"url": "v"}}, &vm)
	if !ok || vm.URL != "v" {
		t.Fatalf("ParseVideo failed: ok=%v url=%s", ok, vm.URL)
	}

	// ParseRecord
	var rec RecordMessage
	ok = ParseRecord(OB11Segment{Type: SegmentRecord, Data: map[string]any{"url": "r"}}, &rec)
	if !ok || rec.URL != "r" {
		t.Fatalf("ParseRecord failed: ok=%v url=%s", ok, rec.URL)
	}

	// ParseJson
	jm := JsonMessage{View: "x"}
	raw, _ := json.Marshal(jm)
	var jmsg JsonMessage
	ok = ParseJson(OB11Segment{Type: SegmentJson, Data: map[string]any{"data": string(raw)}}, &jmsg)
	if !ok || jmsg.View != "x" {
		t.Fatalf("ParseJson failed: ok=%v view=%s", ok, jmsg.View)
	}

	// ParseMusic
	var mus MusicMessage
	ok = ParseMusic(OB11Segment{Type: SegmentMusic, Data: map[string]any{"title": "t"}}, &mus)
	if !ok || mus.Title != "t" {
		t.Fatalf("ParseMusic failed: ok=%v title=%s", ok, mus.Title)
	}

	// ParseFile
	var fmsg FileMessage
	ok = ParseFile(OB11Segment{Type: SegmentFile, Data: map[string]any{"file": "file1"}}, &fmsg)
	if !ok || fmsg.File != "file1" {
		t.Fatalf("ParseFile failed: ok=%v file=%s", ok, fmsg.File)
	}

	// ParseForward
	var forw ForwardMessage
	ok = ParseForward(OB11Segment{Type: SegmentForward, Data: map[string]any{"id": "123"}}, &forw)
	if !ok || forw.Id != "123" {
		t.Fatalf("ParseForward failed: ok=%v id=%s", ok, forw.Id)
	}
}
