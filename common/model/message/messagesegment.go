package message

type TextMessage struct {
	Text string
}

type FaceMessage struct {
	Id int
}

type ImageMessage struct {
	File string
	Url  string
}

type MusicMessage struct {
	Title string
}

type MentionMessage struct {
	QQ    uint
	IsAll bool
}

type ReplyMessage struct {
	Id uint
}

type FileMessage struct {
	File string
}

type dataMessage struct {
	URL string
}

type VideoMessage dataMessage

type RecordMessage dataMessage

type ForwardMessage struct {
	Id string
}
