package message

type BasicNotice struct {
	Time       uint   `json:"time"`
	PostType   string `json:"post_type"`
	SelfId     QID    `json:"self_id"`
	NoticeType string `json:"notice_type"`
}

// GroupUploadNotice 群文件上传
type GroupUploadNotice struct {
	BasicNotice
	GroupId QID `json:"group_id"`
	UserId  QID `json:"user_id"`
	File    struct {
		Id    QID    `json:"id"`
		Name  string `json:"name"`
		Size  uint   `json:"size"`
		Busid uint   `json:"busid"`
	} `json:"file"`
}

// GroupAdminNotice 群管理员变动
type GroupAdminNotice struct {
	BasicNotice
	SubType string `json:"sub_type"` // "set" / "unset"
	GroupId QID    `json:"group_id"`
	UserId  QID    `json:"user_id"`
}

// GroupDecreaseNotice 群成员减少
type GroupDecreaseNotice struct {
	BasicNotice
	SubType    string `json:"sub_type"` // "leave" / "kick" / "kick_me"
	GroupId    QID    `json:"group_id"`
	OperatorId QID    `json:"operator_id"`
	UserId     QID    `json:"user_id"`
}

// GroupIncreaseNotice 群成员增加
type GroupIncreaseNotice struct {
	BasicNotice
	SubType    string `json:"sub_type"` // "approve" / "invite"
	GroupId    QID    `json:"group_id"`
	OperatorId QID    `json:"operator_id"`
	UserId     QID    `json:"user_id"`
}

// GroupBanNotice 群禁言
type GroupBanNotice struct {
	BasicNotice
	SubType    string `json:"sub_type"` // "ban" / "lift_ban"
	GroupId    QID    `json:"group_id"`
	OperatorId QID    `json:"operator_id"`
	UserId     QID    `json:"user_id"`
	Duration   uint   `json:"duration"`
}

// FriendAddNotice 新添加好友
type FriendAddNotice struct {
	BasicNotice
	UserId QID `json:"user_id"`
}

// GroupRecallNotice 群消息撤回
type GroupRecallNotice struct {
	BasicNotice
	GroupId    QID  `json:"group_id"`
	UserId     QID  `json:"user_id"`
	OperatorId QID  `json:"operator_id"`
	MessageId  uint `json:"message_id"`
}

// FriendRecallNotice 好友消息撤回
type FriendRecallNotice struct {
	BasicNotice
	UserId    QID  `json:"user_id"`
	MessageId uint `json:"message_id"`
}

// PokeNotice 戳一戳
type PokeNotice struct {
	BasicNotice
	SubType  string `json:"sub_type"`           // "poke"
	GroupId  *QID   `json:"group_id,omitempty"` // 私聊戳一戳无 group_id
	UserId   QID    `json:"user_id"`
	TargetId QID    `json:"target_id"`
}

// LuckyKingNotice 运气王
type LuckyKingNotice struct {
	BasicNotice
	SubType  string `json:"sub_type"` // "lucky_king"
	GroupId  QID    `json:"group_id"`
	UserId   QID    `json:"user_id"`
	TargetId QID    `json:"target_id"`
}

// HonorNotice 荣誉变更
type HonorNotice struct {
	BasicNotice
	SubType   string `json:"sub_type"` // "honor"
	GroupId   QID    `json:"group_id"`
	HonorType string `json:"honor_type"`
	UserId    QID    `json:"user_id"`
}

// GroupMsgEmojiLikeNotice 群表情回应
type GroupMsgEmojiLikeNotice struct {
	BasicNotice
	GroupId   QID `json:"group_id"`
	UserId    QID `json:"user_id"` // 贴/取消贴表情的操作者
	MessageId QID `json:"message_id"`
	Likes     []struct {
		EmojiId string `json:"emoji_id"`
		Count   int    `json:"count"`
	} `json:"likes,omitempty"`
}

// EssenceNotice 群精华
type EssenceNotice struct {
	BasicNotice
	SubType    string `json:"sub_type"` // "add" / "delete"
	GroupId    QID    `json:"group_id"`
	MessageId  QID    `json:"message_id"`
	SenderId   QID    `json:"sender_id"`
	OperatorId QID    `json:"operator_id"`
}

// GroupCardNotice 群名片变更
type GroupCardNotice struct {
	BasicNotice
	GroupId QID    `json:"group_id"`
	UserId  QID    `json:"user_id"`
	CardNew string `json:"card_new"`
	CardOld string `json:"card_old"`
}
