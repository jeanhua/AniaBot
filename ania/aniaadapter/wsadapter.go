package aniaadapter

import (
	"encoding/json"
	"log"

	"github.com/gorilla/websocket"
	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/jeanhua/AniaBot/common/msgchain"
	"github.com/spf13/viper"
)

type napcatWebSocketAdapter struct {
	wsConn        *websocket.Conn
	groupMsgFunc  func(message.Message)
	friendMsgFunc func(message.Message)
}

func (n *napcatWebSocketAdapter) Serve(v *viper.Viper) {
	url := v.GetString("bot.adapter.ws.address")
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		log.Fatal("Failed to connect:", err)
	}
	n.wsConn = conn
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			log.Println("Failed to receive message:", err)
			break
		}
		n.onMsg(msg)
	}
	conn.Close()
}

func (n *napcatWebSocketAdapter) SetGroupMsgEvent(f func(message.Message)) {
	n.groupMsgFunc = f
}

func (n *napcatWebSocketAdapter) SetFriendMsgEvent(f func(message.Message)) {
	n.friendMsgFunc = f
}

func (n *napcatWebSocketAdapter) SendGroupMsg(groupId uint, chain msgchain.Chain) {
	raw := wsPushGroupData{
		Action: "send_group_msg",
		Params: struct {
			GroupId uint                  "json:\"group_id\""
			Message []message.OB11Segment "json:\"message\""
		}{
			GroupId: groupId,
			Message: chain.GetMsg(),
		},
	}
	b, err := json.Marshal(&raw)
	if err != nil {
		log.Println("消息链序列化失败")
		return
	}
	if err := n.wsConn.WriteMessage(websocket.TextMessage, b); err != nil {
		log.Fatal("消息发送失败:", err)
	}
}

func (n *napcatWebSocketAdapter) SendFriendMsg(friendId uint, chain msgchain.Chain) {
	raw := wsPushFriendData{
		Action: "/send_private_msg",
		Params: struct {
			Friend  uint                  "json:\"friend_id\""
			Message []message.OB11Segment "json:\"message\""
		}{
			Friend:  friendId,
			Message: chain.GetMsg(),
		},
	}
	b, err := json.Marshal(&raw)
	if err != nil {
		log.Println("消息链序列化失败")
		return
	}
	if err := n.wsConn.WriteMessage(websocket.TextMessage, b); err != nil {
		log.Fatal("消息发送失败:", err)
	}
}

func (n *napcatWebSocketAdapter) onMsg(data []byte) {
	var msg message.Message
	if err := json.Unmarshal(data, &msg); err != nil {
		log.Println("解析WebSocket消息失败")
	}
	if msg.PostType == "message" {
		switch msg.MessageType {
		case "group":
			go n.groupMsgFunc(msg)
		case "private":
			go n.friendMsgFunc(msg)
		}
	}
}

type wsPushGroupData struct {
	Action string `json:"action"`
	Params struct {
		GroupId uint                  `json:"group_id"`
		Message []message.OB11Segment `json:"message"`
	} `json:"params"`
}

type wsPushFriendData struct {
	Action string `json:"action"`
	Params struct {
		Friend  uint                  `json:"friend_id"`
		Message []message.OB11Segment `json:"message"`
	} `json:"params"`
}
