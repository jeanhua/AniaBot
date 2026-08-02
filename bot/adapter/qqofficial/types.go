package qqofficial

import "encoding/json"

// ---------- 网关通用结构 ----------

// Opcode 网关操作码（见官方文档「通用数据结构」）。
const (
	opDispatch     = 0  // 服务端推送事件
	opHeartbeat    = 1  // 心跳（客户端发送）
	opIdentify     = 2  // 鉴权（客户端发送）
	opResume       = 6  // 恢复连接（客户端发送）
	opReconnect    = 7  // 服务端通知客户端重连
	opInvalidSess  = 9  // identify/resume 参数错误
	opHello        = 10 // 连接建立后网关下发的第一条消息
	opHeartbeatACK = 11 // 心跳 ACK
)

// wsPayload 网关上下行通用结构。
type wsPayload struct {
	ID string          `json:"id,omitempty"` // 事件 ID（下行）
	Op int             `json:"op"`
	D  json.RawMessage `json:"d,omitempty"`
	S  int             `json:"s,omitempty"` // 下行序列号（心跳时需回传最新值）
	T  string          `json:"t,omitempty"` // 事件类型（op=0 时有效）
}

// helloData OpCode 10 Hello 的内容。
type helloData struct {
	HeartbeatInterval int `json:"heartbeat_interval"` // 毫秒
}

// identifyData OpCode 2 Identify 的内容。
type identifyData struct {
	Token      string         `json:"token"` // "QQBot {access_token}"
	Intents    int            `json:"intents"`
	Shard      [2]int         `json:"shard"`
	Properties map[string]any `json:"properties"`
}

// resumeData OpCode 6 Resume 的内容。
type resumeData struct {
	Token     string `json:"token"`
	SessionID string `json:"session_id"`
	Seq       int    `json:"seq"`
}

// readyData READY 事件内容：携带会话 ID 与机器人自身信息。
type readyData struct {
	Version   int    `json:"version"`
	SessionID string `json:"session_id"`
	User      struct {
		ID       string `json:"id"`
		Username string `json:"username"`
		Bot      bool   `json:"bot"`
	} `json:"user"`
	Shard [2]int `json:"shard"`
}

// Intents 事件订阅位：仅订阅 GROUP_AND_C2C_EVENT（1<<25），
// 覆盖 C2C_MESSAGE_CREATE / GROUP_AT_MESSAGE_CREATE / FRIEND_ADD / FRIEND_DEL /
// GROUP_ADD_ROBOT / GROUP_DEL_ROBOT / C2C_MSG_REJECT / C2C_MSG_RECEIVE /
// GROUP_MSG_REJECT / GROUP_MSG_RECEIVE。频道（guild）事件不在本适配器范围。
const intentGroupAndC2C = 1 << 25

// ---------- 事件 ----------

// eventUser 事件中的用户对象。
type eventUser struct {
	ID           string `json:"id"`
	Username     string `json:"username"`
	Bot          bool   `json:"bot"`
	UnionOpenID  string `json:"union_openid,omitempty"`
	UserOpenID   string `json:"user_openid,omitempty"`   // 单聊场景
	MemberOpenID string `json:"member_openid,omitempty"` // 群聊场景
	MemberRole   string `json:"member_role,omitempty"`   // member / admin / owner
}

// eventAttachment 消息附件（图片/语音/视频/文件）。
type eventAttachment struct {
	URL          string `json:"url"`
	Filename     string `json:"filename"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	Size         int    `json:"size"`
	ContentType  string `json:"content_type"` // image/jpeg|image/png|image/gif|video/mp4|voice|file
	VoiceWavURL  string `json:"voice_wav_url,omitempty"`
	AsrReferText string `json:"asr_refer_text,omitempty"`
}

// msgElement 引用/嵌套消息元素（message_type=103 时有值）。
type msgElement struct {
	Content string `json:"content"`
}

// groupMessageEvent GROUP_AT_MESSAGE_CREATE 事件体。
// content 已由平台去除 @机器人 前缀；相同 msg_id 可能重复推送（需去重）。
type groupMessageEvent struct {
	ID          string            `json:"id"`
	Author      eventUser         `json:"author"`
	Content     string            `json:"content"`
	GroupOpenID string            `json:"group_openid"`
	Timestamp   string            `json:"timestamp"` // RFC3339
	MessageType int               `json:"message_type"`
	Attachments []eventAttachment `json:"attachments,omitempty"`
	Mentions    []eventUser       `json:"mentions,omitempty"` // 消息中 @ 的其他用户（不含机器人自身）
	MsgElements []msgElement      `json:"msg_elements,omitempty"`
}

// c2cMessageEvent C2C_MESSAGE_CREATE 事件体。
type c2cMessageEvent struct {
	ID          string            `json:"id"`
	Author      eventUser         `json:"author"`
	Content     string            `json:"content"`
	Timestamp   string            `json:"timestamp"`
	MessageType int               `json:"message_type"`
	Attachments []eventAttachment `json:"attachments,omitempty"`
	MsgElements []msgElement      `json:"msg_elements,omitempty"`
}

// friendEvent FRIEND_ADD / FRIEND_DEL 事件体。
type friendEvent struct {
	OpenID    string `json:"openid"`
	Timestamp int64  `json:"timestamp"` // Unix 秒
}

// groupRobotEvent GROUP_ADD_ROBOT / GROUP_DEL_ROBOT / GROUP_MSG_REJECT /
// GROUP_MSG_RECEIVE 事件体。
type groupRobotEvent struct {
	GroupOpenID    string `json:"group_openid"`
	OpMemberOpenID string `json:"op_member_openid"`
	Timestamp      int64  `json:"timestamp"` // Unix 秒
}

// ---------- OpenAPI ----------

// tokenResponse getAppAccessToken 响应。expires_in 官方示例为字符串，
// 但文档描述为 number，两种形态都兼容（见 UnmarshalJSON 调用方处理）。
type tokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   any    `json:"expires_in"`
}

// gatewayResponse GET /gateway 响应。
type gatewayResponse struct {
	URL string `json:"url"`
}

// apiErrorBody OpenAPI 错误响应体：成功时不含 err_code（或为 0）。
type apiErrorBody struct {
	ErrCode int    `json:"err_code"`
	Message string `json:"message"`
	TraceID string `json:"trace_id"`
}

// sendMessageRequest POST /v2/{groups|users}/{openid}/messages 请求体。
type sendMessageRequest struct {
	MsgType          int               `json:"msg_type,omitempty"`
	Content          string            `json:"content,omitempty"`
	Markdown         *markdownPayload  `json:"markdown,omitempty"`
	MsgID            string            `json:"msg_id,omitempty"`
	MsgSeq           int               `json:"msg_seq,omitempty"`
	Media            *mediaPayload     `json:"media,omitempty"`
	MessageReference *messageReference `json:"message_reference,omitempty"`
}

type markdownPayload struct {
	Content string `json:"content,omitempty"`
}

type mediaPayload struct {
	FileInfo string `json:"file_info,omitempty"`
}

type messageReference struct {
	MessageID string `json:"message_id,omitempty"`
}

// sendMessageResponse 发送消息响应。
type sendMessageResponse struct {
	ID        string `json:"id"`
	Timestamp string `json:"timestamp"`
}

// uploadFileRequest POST /v2/{groups|users}/{openid}/files 请求体。
type uploadFileRequest struct {
	FileType   int    `json:"file_type"`           // 1=图片 2=视频 3=语音 4=文件
	URL        string `json:"url,omitempty"`       // URL 直传（平台下载转存）
	SrvSendMsg bool   `json:"srv_send_msg"`        // true=平台直接发送（占主动消息频次）
	FileName   string `json:"file_name,omitempty"` // 文件名（可选）
	UploadID   string `json:"upload_id,omitempty"` // 分片上传合并路径
}

// uploadFileResponse /files 响应。
type uploadFileResponse struct {
	FileUUID string `json:"file_uuid"`
	FileInfo string `json:"file_info"`
	TTL      int    `json:"ttl"`
	ID       string `json:"id,omitempty"` // 仅 srv_send_msg=true 时返回
}

// uploadPrepareRequest POST /v2/{groups|users}/{openid}/upload_prepare 请求体。
type uploadPrepareRequest struct {
	FileType int    `json:"file_type"`
	FileSize string `json:"file_size"` // 字节数（十进制字符串，官方定义为 string）
	FileName string `json:"file_name,omitempty"`
	Md5      string `json:"md5,omitempty"`
	Sha1     string `json:"sha1,omitempty"`
	Md510m   string `json:"md5_10m,omitempty"` // 前 10002432 字节的 MD5
}

// uploadPrepareResponse upload_prepare 响应。
type uploadPrepareResponse struct {
	UploadID  string `json:"upload_id"`
	BlockSize string `json:"block_size"`
	Parts     []struct {
		Index        int    `json:"index"`
		PresignedURL string `json:"presigned_url"`
		BlockSize    string `json:"block_size"`
	} `json:"parts"`
}

// uploadPartFinishRequest POST /v2/{groups|users}/{openid}/upload_part_finish 请求体。
type uploadPartFinishRequest struct {
	UploadID  string `json:"upload_id"`
	PartIndex int    `json:"part_index"`
	BlockSize string `json:"block_size"` // 该分片的实际字节数
	Md5       string `json:"md5"`        // 该分片的 MD5
}
