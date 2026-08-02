package telegram

import "encoding/json"

// 本文件声明 Telegram Bot API 的 JSON 类型（只声明用到的字段，
// 指针区分缺省值）。字段名与官方文档一致：
// https://core.telegram.org/bots/api

// apiResponse 所有 Bot API 方法响应的统一外层。
type apiResponse struct {
	OK          bool            `json:"ok"`
	Result      json.RawMessage `json:"result"`
	ErrorCode   int             `json:"error_code"`
	Description string          `json:"description"`
	Parameters  *struct {
		RetryAfter int `json:"retry_after"` // 429 限流时建议等待的秒数
	} `json:"parameters"`
}

// Update 长轮询返回的一个更新。
type Update struct {
	UpdateID        int                     `json:"update_id"`
	Message         *Message                `json:"message"`
	ChannelPost     *Message                `json:"channel_post"`
	MyChatMember    *ChatMemberUpdated      `json:"my_chat_member"`
	MessageReaction *MessageReactionUpdated `json:"message_reaction"`
	// edited_message / callback_query / chat_member 等其余更新类型未使用，不声明
}

// Message 一条消息（message 或 channel_post）。
type Message struct {
	MessageID       int             `json:"message_id"`
	Date            int64           `json:"date"`
	Chat            Chat            `json:"chat"`
	From            *User           `json:"from"` // 频道消息可能为空
	Text            string          `json:"text"`
	Caption         string          `json:"caption"` // 媒体消息的说明文字
	Entities        []MessageEntity `json:"entities"`
	CaptionEntities []MessageEntity `json:"caption_entities"`
	ReplyToMessage  *Message        `json:"reply_to_message"`
	Photo           []PhotoSize     `json:"photo"`
	Document        *Document       `json:"document"`
	Voice           *Voice          `json:"voice"`
	Audio           *Audio          `json:"audio"`
	Video           *Video          `json:"video"`
	Sticker         *Sticker        `json:"sticker"`
	Animation       *Animation      `json:"animation"`
	VideoNote       *VideoNote      `json:"video_note"`
	NewChatMembers  []User          `json:"new_chat_members"` // 服务消息：加入群
	LeftChatMember  *User           `json:"left_chat_member"` // 服务消息：离开/被踢
}

// Chat 会话（私聊/群组/超群/频道）。
type Chat struct {
	ID          int64  `json:"id"`
	Type        string `json:"type"` // private | group | supergroup | channel
	Title       string `json:"title"`
	MemberCount int    `json:"member_count"`
}

// User 用户或机器人。
type User struct {
	ID        int64  `json:"id"`
	IsBot     bool   `json:"is_bot"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Username  string `json:"username"` // 可能为空
}

// MessageEntity 消息文本特殊实体。
type MessageEntity struct {
	Type   string `json:"type"` // mention | text_mention | url | bold ...
	Offset int    `json:"offset"`
	Length int    `json:"length"`
	User   *User  `json:"user"` // 仅 text_mention 携带
}

// PhotoSize 图片尺寸（同 file_id 的多个尺寸取最大者）。
type PhotoSize struct {
	FileID   string `json:"file_id"`
	FileSize int    `json:"file_size"`
}

// Document 文件消息。
type Document struct {
	FileID   string `json:"file_id"`
	FileName string `json:"file_name"`
}

// Voice 语音消息（ogg/opus）。
type Voice struct {
	FileID string `json:"file_id"`
}

// Audio 音频文件（mp3/m4a 等）。
type Audio struct {
	FileID   string `json:"file_id"`
	FileName string `json:"file_name"`
	Title    string `json:"title"`
}

// Video 视频消息。
type Video struct {
	FileID string `json:"file_id"`
}

// Sticker 贴纸。
type Sticker struct {
	FileID string `json:"file_id"`
}

// Animation 动画（GIF）。
type Animation struct {
	FileID string `json:"file_id"`
}

// VideoNote 圆形视频消息。
type VideoNote struct {
	FileID string `json:"file_id"`
}

// ChatMemberUpdated 成员状态更新（my_chat_member / chat_member 更新载体）。
type ChatMemberUpdated struct {
	Chat          Chat `json:"chat"`
	From          User `json:"from"`
	NewChatMember struct {
		Status string `json:"status"` // member | administrator | restricted | kicked | left ...
	} `json:"new_chat_member"`
}

// MessageReactionUpdated 消息表情回应更新（仅 bot 自己消息或管理员可见）。
type MessageReactionUpdated struct {
	Chat        Chat       `json:"chat"`
	MessageID   int        `json:"message_id"`
	User        *User      `json:"user"`
	NewReaction []Reaction `json:"new_reaction"`
}

// Reaction 一个表情回应。
type Reaction struct {
	Type          string `json:"type"` // emoji | custom_emoji
	Emoji         string `json:"emoji"`
	CustomEmojiID string `json:"custom_emoji_id"`
}

// File getFile 返回值。
type File struct {
	FileID   string `json:"file_id"`
	FilePath string `json:"file_path"`
}

// ChatMember 群成员信息（getChatMember 返回，出站 @ 解析用）。
type ChatMember struct {
	User User `json:"user"`
}

// messageSendResult sendMessage 等发送方法的返回值（取 message_id）。
type messageSendResult struct {
	MessageID int `json:"message_id"`
}

// userResult getMe 返回值。
type userResult struct {
	User
}
