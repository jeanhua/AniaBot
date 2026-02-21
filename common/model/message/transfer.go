package message

import (
	"encoding/json"
	"log"
	"strconv"
)

type transferable interface {
	TextMessage | FaceMessage | ImageMessage | MentionMessage |
		ReplyMessage | VideoMessage | RecordMessage | JsonMessage |
		MusicMessage | FileMessage | ForwardMessage
}

func TransferTo[T transferable](s OB11Segment, target *T) bool {
	return transfer(s, target)
}

func transfer(s OB11Segment, target interface{}) (success bool) {
	defer func() {
		if r := recover(); r != nil {
			success = false
		}
	}()
	switch t := target.(type) {
	case *TextMessage:
		if s.Type == "text" {
			t.Text = s.Data["text"].(string)
			return t.Text != ""
		}
		return false
	case *FaceMessage:
		if s.Type == "face" {
			if id, err := strconv.Atoi(s.Data["id"].(string)); err != nil {
				return false
			} else {
				t.Id = id
			}
			return true
		}
		return false
	case *ImageMessage:
		if s.Type == "image" {
			t.File, _ = s.Data["file"].(string)
			t.Url, _ = s.Data["url"].(string)
			return t.Url != ""
		}
		return false
	case *MentionMessage:
		if s.Type == "at" {
			qq, _ := s.Data["qq"].(string)
			if qq == "all" {
				t.IsAll = true
				return true
			}
			qqInt, _ := strconv.Atoi(qq)
			t.QQ = uint(qqInt)
			return qq != ""
		}
		return false
	case *ReplyMessage:
		if s.Type == "reply" {
			id, _ := s.Data["id"].(string)
			idInt, _ := strconv.Atoi(id)
			t.Id = uint(idInt)
			return id != ""
		}
		return false
	case *VideoMessage:
		if s.Type == "video" {
			t.URL, _ = s.Data["url"].(string)
			return t.URL != ""
		}
		return false
	case *RecordMessage:
		if s.Type == "record" {
			t.URL, _ = s.Data["url"].(string)
			return t.URL != ""
		}
		return false
	case *JsonMessage:
		if s.Type == "json" {
			if err := json.Unmarshal([]byte(s.Data["data"].(string)), t); err != nil {
				log.Println("error when json unmarshal: json message", err)
				return false
			}
			return true
		}
		return false
	case *MusicMessage:
		if s.Type == "music" {
			t.Title, _ = s.Data["title"].(string)
			return t.Title != ""
		}
		return false
	case *FileMessage:
		if s.Type == "file" {
			t.File, _ = s.Data["file"].(string)
			return t.File != ""
		}
		return false
	case *ForwardMessage:
		if s.Type == "forward" {
			t.Id = s.Data["id"].(string)
			return t.Id != ""
		}
		return false
	default:
		return false
	}
}
