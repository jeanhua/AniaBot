package message

import "encoding/json"

type TextMessage struct {
	Text string `json:"text"`
}

type FaceMessage struct {
	Id json.Number `json:"id"`
}

type ImageMessage struct {
	File string `json:"file"`
	Url  string `json:"url"`
}

type MusicMessage struct {
	Title string `json:"title"`
}

type MentionMessage struct {
	QQ string `json:"qq"`
}

type ReplyMessage struct {
	Id json.Number `json:"id"`
}

type FileMessage struct {
	File string `json:"file"`
}

type dataMessage struct {
	URL string `json:"url"`
}

type VideoMessage dataMessage

type RecordMessage dataMessage
