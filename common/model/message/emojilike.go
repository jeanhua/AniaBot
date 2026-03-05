package message

type EmojiLike struct {
	MessageID QID  `json:"message_id"`
	EmojiId   int  `json:"emoji_id"`
	Set       bool `json:"set"`
}
