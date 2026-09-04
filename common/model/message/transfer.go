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

	// 不再限定数字：多平台下 at 目标可能是带平台前缀的 ID（如 fs:ou_xxx）
	if qq == "" {
		return false
	}

	m.QQ = FromString(qq)
	return true
}

func ParseReply(s OB11Segment, r *ReplyMessage) bool {
	if s.Type != "reply" {
		return false
	}

	idStr, ok := s.Data["id"].(string)
	if !ok || idStr == "" {
		return false
	}

	r.Id = FromString(idStr)
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

	url, ok := s.Data["url"].(string)
	if ok && url != "" {
		f.URL = url
	}

	fileId, ok := s.Data["file_id"].(string)
	if ok {
		f.FileId = fileId
	}

	if name, ok := s.Data["name"].(string); ok {
		f.Name = name
	}

	f.File = file
	f.FileId = fileId
	f.URL = url
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

	qid := FromString(id)
	f.Id = qid
	return true
}

// ParseForwardContent 解析合并转发段内联携带的消息内容（OB11Message 数组）。
// NapCat 解析转发内容时会把（含嵌套的）合并转发消息一并放进 forward 段的
// content 字段；内层转发 id 仅供查看、无法再通过 get_forward_msg 拉取，
// 因此有内联内容时应直接解析而不是按 id 请求。
func ParseForwardContent(s OB11Segment) ([]Message, bool) {
	if s.Type != SegmentForward || s.Data == nil {
		return nil, false
	}
	raw, ok := s.Data["content"]
	if !ok {
		return nil, false
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return nil, false
	}
	var msgs []Message
	if err := json.Unmarshal(b, &msgs); err != nil || len(msgs) == 0 {
		return nil, false
	}
	return msgs, true
}
