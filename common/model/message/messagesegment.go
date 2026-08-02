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
	File    string
	Url     string
	Summary string
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
	File   string
	FileId string
	Name   string
	dataMessage
}

type dataMessage struct {
	URL string
}

type VideoMessage dataMessage

type RecordMessage dataMessage

type ForwardMessage struct {
	Id QID
}

// Marshal 系列：typed 消息 → OB11Segment.Data（键与 ParseXxx 读取完全一致，
// 是段数据构造的单一事实来源，适配器与 msgchain 构造段时统一使用）。

func (t TextMessage) Marshal() map[string]any { return map[string]any{"text": t.Text} }
func (f FaceMessage) Marshal() map[string]any { return map[string]any{"id": f.Id} }
func (i ImageMessage) Marshal() map[string]any {
	return map[string]any{"file": i.File, "url": i.Url, "summary": i.Summary}
}
func (m MusicMessage) Marshal() map[string]any { return map[string]any{"title": m.Title} }
func (r ReplyMessage) Marshal() map[string]any { return map[string]any{"id": r.Id.String()} }
func (f FileMessage) Marshal() map[string]any {
	return map[string]any{"file": f.File, "file_id": f.FileId, "name": f.Name, "url": f.URL}
}
func (v VideoMessage) Marshal() map[string]any  { return map[string]any{"file": v.URL, "url": v.URL} }
func (r RecordMessage) Marshal() map[string]any { return map[string]any{"file": r.URL} }

// MentionMessage.Marshal 全员 @ 时写 "all"（与 ParseMention 读取一致）。
func (m MentionMessage) Marshal() map[string]any {
	if m.IsAll {
		return map[string]any{"qq": "all"}
	}
	return map[string]any{"qq": m.QQ.String()}
}
