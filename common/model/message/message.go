package message

type Message struct {
	Time        uint          `json:"time"`
	PostType    string        `json:"post_type"`
	MessageType string        `json:"message_type"`
	SubType     string        `json:"sub_type"`
	MessageId   uint          `json:"message_id"`
	UserId      uint          `json:"user_id"`
	GroupId     uint          `json:"group_id"`
	Message     []OB11Segment `json:"message"`
	RawMessage  string        `json:"raw_message"`
	Sender      MessageSender `json:"sender"`
	SelfId      uint          `json:"self_id"`
}

type OB11Segment struct {
	Type string                 `json:"type"`
	Data map[string]interface{} `json:"data"`
}

type MessageSender struct {
	UserId   uint   `json:"user_id"`
	Nickname string `json:"nickname"`
	Sex      string `json:"sex"`
	Card     string `json:"card"`
	Role     string `json:"role"`
}

type Response struct {
	Status  string `json:"status"`
	Retcode int    `json:"retcode"`
	Data    struct {
		MessageId uint `json:"message_id"`
	} `json:"data"`
}

type MessageDetail struct {
	Status  string `json:"status"`
	RetCode int    `json:"retcode"`
	Data    struct {
		SelfID      int64  `json:"self_id"`
		UserID      int64  `json:"user_id"`
		Time        int64  `json:"time"`
		MessageID   int64  `json:"message_id"`
		MessageSeq  int64  `json:"message_seq"`
		RealID      int64  `json:"real_id"`
		RealSeq     string `json:"real_seq"`
		MessageType string `json:"message_type"`
		Sender      struct {
			UserID   int64  `json:"user_id"`
			Nickname string `json:"nickname"`
			Card     string `json:"card"`
			Role     string `json:"role"`
		} `json:"sender"`
		RawMessage    string        `json:"raw_message"`
		Font          int           `json:"font"`
		SubType       string        `json:"sub_type"`
		Message       []OB11Segment `json:"message"`
		MessageFormat string        `json:"message_format"`
		PostType      string        `json:"post_type"`
		GroupID       int64         `json:"group_id"`
	} `json:"data"`
}

type JsonMessage struct {
	App    string `json:"app"`
	Bizsrc string `json:"bizsrc"`
	Config struct {
		Ctime   int64  `json:"ctime"`
		Forward int    `json:"forward"`
		Token   string `json:"token"`
		Type    string `json:"type"`
	} `json:"config"`
	Extra struct {
		AppType int   `json:"app_type"`
		Appid   int64 `json:"appid"`
		MsgSeq  int64 `json:"msg_seq"`
		Uin     int64 `json:"uin"`
	} `json:"extra"`
	Meta struct {
		News struct {
			AppType int    `json:"app_type"`
			Appid   int64  `json:"appid"`
			Ctime   int64  `json:"ctime"`
			Desc    string `json:"desc"`
			JumpUrl string `json:"jumpUrl"`
			Preview string `json:"preview"`
			Tag     string `json:"tag"`
			TagIcon string `json:"tagIcon"`
			Title   string `json:"title"`
			Uin     int64  `json:"uin"`
		} `json:"news"`
	} `json:"meta"`
	Prompt string `json:"prompt"`
	Ver    string `json:"ver"`
	View   string `json:"view"`
}
