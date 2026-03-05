package message

const (
	SegmentText    = "text"
	SegmentFace    = "face"
	SegmentImage   = "image"
	SegmentMention = "at"
	SegmentReply   = "reply"
	SegmentVideo   = "video"
	SegmentRecord  = "record"
	SegmentJson    = "json"
	SegmentMusic   = "music"
	SegmentFile    = "file"
	SegmentForward = "forward"
)

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
	QQ    QID
	IsAll bool
}

type ReplyMessage struct {
	Id QID
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
	Id QID
}
