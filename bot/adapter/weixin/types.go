package weixin

// 微信 iLink Bot 协议类型（JSON over HTTP，字节字段为 base64/hex 字符串）。
// 协议参考开源实现 github.com/Tencent/openclaw-weixin（MIT）。

// BaseInfo 附带在所有 CGI 请求体中的公共元信息。
type BaseInfo struct {
	ChannelVersion string `json:"channel_version,omitempty"`
	// BotAgent 上游应用自我声明（类似 HTTP User-Agent），仅用于后台日志归因
	BotAgent string `json:"bot_agent,omitempty"`
}

// UploadMediaType CDN 上传媒体类型。
const (
	UploadMediaImage = 1
	UploadMediaVideo = 2
	UploadMediaFile  = 3
	UploadMediaVoice = 4
)

// GetUploadUrlReq 获取 CDN 预签名上传 URL 请求。
type GetUploadUrlReq struct {
	Filekey         string   `json:"filekey,omitempty"`
	MediaType       int      `json:"media_type,omitempty"`
	ToUserID        string   `json:"to_user_id,omitempty"`
	Rawsize         int64    `json:"rawsize,omitempty"`       // 原文件明文大小
	Rawfilemd5      string   `json:"rawfilemd5,omitempty"`    // 原文件明文 MD5
	Filesize        int64    `json:"filesize,omitempty"`      // 密文大小（AES-128-ECB PKCS7 后）
	ThumbRawsize    int64    `json:"thumb_rawsize,omitempty"` // 缩略图明文大小（IMAGE/VIDEO 必填）
	ThumbRawfilemd5 string   `json:"thumb_rawfilemd5,omitempty"`
	ThumbFilesize   int64    `json:"thumb_filesize,omitempty"`
	NoNeedThumb     bool     `json:"no_need_thumb,omitempty"`
	Aeskey          string   `json:"aeskey,omitempty"` // 加密 key（hex）
	BaseInfo        BaseInfo `json:"base_info"`
}

// GetUploadUrlResp 获取 CDN 上传 URL 响应。
type GetUploadUrlResp struct {
	UploadParam      string `json:"upload_param"`       // 原图上传加密参数
	ThumbUploadParam string `json:"thumb_upload_param"` // 缩略图上传加密参数
	UploadFullURL    string `json:"upload_full_url"`    // 完整上传 URL（服务端直接返回）
}

// MessageType 消息归属：1=用户 2=机器人。
const (
	MsgTypeUser = 1
	MsgTypeBot  = 2
)

// MessageItemType 消息条目类型。
const (
	ItemText           = 1
	ItemImage          = 2
	ItemVoice          = 3
	ItemFile           = 4
	ItemVideo          = 5
	ItemToolCallStart  = 11
	ItemToolCallResult = 12
)

// MessageState 消息状态：0=NEW 1=GENERATING 2=FINISH。
const (
	MsgStateNew        = 0
	MsgStateGenerating = 1
	MsgStateFinish     = 2
)

// TextItem 文本条目。
type TextItem struct {
	Text string `json:"text,omitempty"`
}

// CDNMedia CDN 媒体引用（aes_key 为 base64 字符串）。
type CDNMedia struct {
	EncryptQueryParam string `json:"encrypt_query_param,omitempty"`
	AesKey            string `json:"aes_key,omitempty"`
	EncryptType       int    `json:"encrypt_type,omitempty"` // 0=只加密fileid 1=打包缩略图等信息
	FullURL           string `json:"full_url,omitempty"`     // 完整下载 URL（服务端直接返回）
}

// ImageItem 图片条目。
type ImageItem struct {
	Media       *CDNMedia `json:"media,omitempty"`
	ThumbMedia  *CDNMedia `json:"thumb_media,omitempty"`
	Aeskey      string    `json:"aeskey,omitempty"` // 原始 AES key 的 hex 串（入站解密优先于 media.aes_key）
	URL         string    `json:"url,omitempty"`
	MidSize     int64     `json:"mid_size,omitempty"`
	ThumbSize   int64     `json:"thumb_size,omitempty"`
	ThumbHeight int       `json:"thumb_height,omitempty"`
	ThumbWidth  int       `json:"thumb_width,omitempty"`
	HdSize      int64     `json:"hd_size,omitempty"`
}

// VoiceItem 语音条目（encode_type: 6=silk）。
type VoiceItem struct {
	Media         *CDNMedia `json:"media,omitempty"`
	EncodeType    int       `json:"encode_type,omitempty"`
	BitsPerSample int       `json:"bits_per_sample,omitempty"`
	SampleRate    int       `json:"sample_rate,omitempty"`
	Playtime      int64     `json:"playtime,omitempty"` // 毫秒
	Text          string    `json:"text,omitempty"`     // 语音转文字内容
}

// FileItem 文件条目。
type FileItem struct {
	Media    *CDNMedia `json:"media,omitempty"`
	FileName string    `json:"file_name,omitempty"`
	Md5      string    `json:"md5,omitempty"`
	Len      string    `json:"len,omitempty"`
}

// VideoItem 视频条目。
type VideoItem struct {
	Media      *CDNMedia `json:"media,omitempty"`
	VideoSize  int64     `json:"video_size,omitempty"`
	PlayLength int64     `json:"play_length,omitempty"`
	VideoMd5   string    `json:"video_md5,omitempty"`
	ThumbMedia *CDNMedia `json:"thumb_media,omitempty"`
}

// RefMessage 引用消息。
type RefMessage struct {
	MessageItem *MessageItem `json:"message_item,omitempty"`
	Title       string       `json:"title,omitempty"` // 摘要
}

// MessageItem 消息条目（type 决定哪个 item 字段有效）。
type MessageItem struct {
	Type               int         `json:"type,omitempty"`
	CreateTimeMs       int64       `json:"create_time_ms,omitempty"`
	UpdateTimeMs       int64       `json:"update_time_ms,omitempty"`
	IsCompleted        bool        `json:"is_completed,omitempty"`
	MsgID              string      `json:"msg_id,omitempty"`
	RefMsg             *RefMessage `json:"ref_msg,omitempty"`
	TextItem           *TextItem   `json:"text_item,omitempty"`
	ImageItem          *ImageItem  `json:"image_item,omitempty"`
	VoiceItem          *VoiceItem  `json:"voice_item,omitempty"`
	FileItem           *FileItem   `json:"file_item,omitempty"`
	VideoItem          *VideoItem  `json:"video_item,omitempty"`
	ToolCallStartItem  any         `json:"tool_call_start_item,omitempty"`
	ToolCallResultItem any         `json:"tool_call_result_item,omitempty"`
}

// WeixinMessage 统一消息（proto: WeixinMessage）。
type WeixinMessage struct {
	Seq          int64          `json:"seq,omitempty"`
	MessageID    int64          `json:"message_id,omitempty"`
	FromUserID   string         `json:"from_user_id,omitempty"`
	ToUserID     string         `json:"to_user_id,omitempty"`
	ClientID     string         `json:"client_id,omitempty"`
	CreateTimeMs int64          `json:"create_time_ms,omitempty"`
	UpdateTimeMs int64          `json:"update_time_ms,omitempty"`
	DeleteTimeMs int64          `json:"delete_time_ms,omitempty"`
	SessionID    string         `json:"session_id,omitempty"`
	GroupID      string         `json:"group_id,omitempty"`
	MessageType  int            `json:"message_type,omitempty"`
	MessageState int            `json:"message_state,omitempty"`
	ItemList     []*MessageItem `json:"item_list,omitempty"`
	// ContextToken 会话上下文票据：服务端随每条入站消息下发，回复时须原样带回，
	// 否则服务端可能拒发；适配器按用户缓存最近一次收到的票据
	ContextToken string `json:"context_token,omitempty"`
	RunID        string `json:"run_id,omitempty"`
}

// GetUpdatesReq 长轮询拉取请求。
type GetUpdatesReq struct {
	GetUpdatesBuf string   `json:"get_updates_buf"` // 本地缓存的游标；首次或重置后传 ""
	BaseInfo      BaseInfo `json:"base_info"`
}

// GetUpdatesResp 长轮询拉取响应。
type GetUpdatesResp struct {
	Ret                  int              `json:"ret"`
	Errcode              int              `json:"errcode"` // 如 -14 = 凭证失效
	Errmsg               string           `json:"errmsg"`
	Msgs                 []*WeixinMessage `json:"msgs"`
	GetUpdatesBuf        string           `json:"get_updates_buf"`        // 新游标，本地缓存后下次携带
	LongpollingTimeoutMs int              `json:"longpolling_timeout_ms"` // 服务端建议的下轮长轮询超时
}

// SendMessageReq 发送消息请求（包装单条 WeixinMessage）。
type SendMessageReq struct {
	Msg      *WeixinMessage `json:"msg"`
	BaseInfo BaseInfo       `json:"base_info"`
}

// SendMessageResp 发送消息响应。
type SendMessageResp struct {
	Ret    int    `json:"ret"`
	Errmsg string `json:"errmsg"`
}

// TypingStatus 输入状态：1=输入中 2=取消。
const (
	TypingStatusTyping = 1
	TypingStatusCancel = 2
)

// SendTypingReq 发送输入状态请求。
type SendTypingReq struct {
	IlinkUserID  string   `json:"ilink_user_id,omitempty"`
	TypingTicket string   `json:"typing_ticket,omitempty"`
	Status       int      `json:"status,omitempty"`
	BaseInfo     BaseInfo `json:"base_info"`
}

// GetConfigResp 获取机器人配置响应（含 typing_ticket）。
type GetConfigResp struct {
	Ret          int    `json:"ret"`
	Errmsg       string `json:"errmsg"`
	TypingTicket string `json:"typing_ticket"`
}

// NotifyStartResp / NotifyStopResp 生命周期通知响应。
type NotifyStartResp struct {
	Ret    int    `json:"ret"`
	Errmsg string `json:"errmsg"`
}

type NotifyStopResp = NotifyStartResp

// QRCodeResponse 获取登录二维码响应。
type QRCodeResponse struct {
	Qrcode           string `json:"qrcode"`             // 二维码票据（轮询状态用）
	QrcodeImgContent string `json:"qrcode_img_content"` // 二维码内容 URL（渲染二维码用）
}

// QRCodeStatusResponse 二维码扫码状态响应。
type QRCodeStatusResponse struct {
	Status       string `json:"status"` // wait|scaned|confirmed|expired|scaned_but_redirect|need_verifycode|verify_code_blocked|binded_redirect
	BotToken     string `json:"bot_token"`
	IlinkBotID   string `json:"ilink_bot_id"`
	Baseurl      string `json:"baseurl"`
	IlinkUserID  string `json:"ilink_user_id"` // 扫码者的用户 ID
	RedirectHost string `json:"redirect_host"` // scaned_but_redirect 时的新轮询主机
}

// 二维码状态常量。
const (
	QRStatusWait              = "wait"
	QRStatusScaned            = "scaned"
	QRStatusConfirmed         = "confirmed"
	QRStatusExpired           = "expired"
	QRStatusScanedButRedir    = "scaned_but_redirect"
	QRStatusNeedVerifyCode    = "need_verifycode"
	QRStatusVerifyCodeBlocked = "verify_code_blocked"
	QRStatusBindedRedirect    = "binded_redirect"
)
