package message

import (
	"encoding/json"
	"log"
	"strconv"
)

// ParseXXX函数，将OB11Segment转换为对应消息

func ParseText(s OB11Segment, t *TextMessage) bool {
	if s.Type != "text" {
		return false
	}

	text, ok := s.Data["text"].(string)
	if !ok || text == "" {
		return false
	}

	t.Text = text
	return true
}

func ParseFace(s OB11Segment, f *FaceMessage) bool {
	if s.Type != "face" {
		return false
	}

	idStr, ok := s.Data["id"].(string)
	if !ok {
		return false
	}

	id, err := strconv.Atoi(idStr)
	if err != nil {
		return false
	}

	f.Id = id
	return true
}

func ParseImage(s OB11Segment, i *ImageMessage) bool {
	if s.Type != "image" {
		return false
	}

	url, ok := s.Data["url"].(string)
	if !ok || url == "" {
		return false
	}

	file, _ := s.Data["file"].(string)

	i.Url = url
	i.File = file
	return true
}

func ParseMention(s OB11Segment, m *MentionMessage) bool {
	if s.Type != "at" {
		return false
	}

	qq, ok := s.Data["qq"].(string)
	if !ok {
		return false
	}

	if qq == "all" {
		m.IsAll = true
		return true
	}

	id, err := strconv.Atoi(qq)
	if err != nil {
		return false
	}

	m.QQ = uint(id)
	return true
}

func ParseReply(s OB11Segment, r *ReplyMessage) bool {
	if s.Type != "reply" {
		return false
	}

	idStr, ok := s.Data["id"].(string)
	if !ok {
		return false
	}

	id, err := strconv.Atoi(idStr)
	if err != nil {
		return false
	}

	r.Id = uint(id)
	return true
}

func ParseVideo(s OB11Segment, v *VideoMessage) bool {
	if s.Type != "video" {
		return false
	}

	url, ok := s.Data["url"].(string)
	if !ok || url == "" {
		return false
	}

	v.URL = url
	return true
}

func ParseRecord(s OB11Segment, r *RecordMessage) bool {
	if s.Type != "record" {
		return false
	}

	url, ok := s.Data["url"].(string)
	if !ok || url == "" {
		return false
	}

	r.URL = url
	return true
}

func ParseJson(s OB11Segment, j *JsonMessage) bool {
	if s.Type != "json" {
		return false
	}

	raw, ok := s.Data["data"].(string)
	if !ok {
		return false
	}

	if err := json.Unmarshal([]byte(raw), j); err != nil {
		log.Println("json unmarshal error:", err)
		return false
	}
	return true
}

func ParseMusic(s OB11Segment, m *MusicMessage) bool {
	if s.Type != "music" {
		return false
	}
	title, ok := s.Data["title"].(string)
	if !ok || title == "" {
		return false
	}
	m.Title = title
	return true
}

func ParseFile(s OB11Segment, f *FileMessage) bool {
	if s.Type != "file" {
		return false
	}

	file, ok := s.Data["file"].(string)
	if !ok || file == "" {
		return false
	}

	f.File = file
	return true
}

func ParseForward(s OB11Segment, f *ForwardMessage) bool {
	if s.Type != "forward" {
		return false
	}

	id, ok := s.Data["id"].(string)
	if !ok || id == "" {
		return false
	}

	f.Id = id
	return true
}
