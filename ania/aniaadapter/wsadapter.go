package aniaadapter

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/jeanhua/AniaBot/common/msgchain"
	"github.com/spf13/viper"
)

type napcatWebSocketAdapter struct {
	wsConn        *websocket.Conn
	groupMsgFunc  func(message.Message)
	friendMsgFunc func(message.Message)
	ackMng        *ackManager
}

type ackManager struct {
	pendingAcks sync.Map
	timeout     time.Duration
}

type pendingAck struct {
	ch    chan msgData
	timer *time.Timer
}

type msgData struct {
	result bool
	msgId  uint
}

func (n *napcatWebSocketAdapter) Serve(v *viper.Viper) {
	// initAck
	n.ackMng = &ackManager{
		timeout: time.Second * 5,
	}

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

func (n *napcatWebSocketAdapter) SendGroupMsg(groupId uint, chain msgchain.Chain) (success bool, msgId uint) {
	messageID := generateMessageID()
	raw := wsPushGroupData{
		Action: "send_group_msg",
		Params: struct {
			GroupId uint                  "json:\"group_id\""
			Message []message.OB11Segment "json:\"message\""
		}{
			GroupId: groupId,
			Message: chain.GetMsg(),
		},
		Echo: messageID,
	}
	b, err := json.Marshal(&raw)
	if err != nil {
		log.Println("消息链序列化失败")
		return
	}

	ackChan := make(chan msgData, 1)
	timer := time.NewTimer(n.ackMng.timeout)
	n.ackMng.pendingAcks.Store(messageID, &pendingAck{
		ch:    ackChan,
		timer: timer,
	})
	defer func() {
		timer.Stop()
		n.ackMng.pendingAcks.Delete(messageID)
	}()
	if err := n.wsConn.WriteMessage(websocket.TextMessage, b); err != nil {
		log.Println("消息发送失败:", err)
		return
	}
	select {
	case result := <-ackChan:
		if result.result {
			return true, result.msgId
		} else {
			return false, 0
		}
	case <-timer.C:
		return false, 0
	}
}

func (n *napcatWebSocketAdapter) SendFriendMsg(userId uint, chain msgchain.Chain) (success bool, msgId uint) {
	messageID := generateMessageID()
	raw := wsPushFriendData{
		Action: "send_private_msg",
		Params: struct {
			UserId  uint                  "json:\"user_id\""
			Message []message.OB11Segment "json:\"message\""
		}{
			UserId:  userId,
			Message: chain.GetMsg(),
		},
		Echo: messageID,
	}
	b, err := json.Marshal(&raw)
	if err != nil {
		log.Println("消息链序列化失败")
		return
	}

	ackChan := make(chan msgData, 1)
	timer := time.NewTimer(n.ackMng.timeout)
	n.ackMng.pendingAcks.Store(messageID, &pendingAck{
		ch:    ackChan,
		timer: timer,
	})
	defer func() {
		timer.Stop()
		n.ackMng.pendingAcks.Delete(messageID)
	}()
	if err := n.wsConn.WriteMessage(websocket.TextMessage, b); err != nil {
		log.Println("消息发送失败:", err)
		return
	}
	select {
	case result := <-ackChan:
		if result.result {
			return true, result.msgId
		} else {
			return false, 0
		}
	case <-timer.C:
		return false, 0
	}
}

func (n *napcatWebSocketAdapter) onMsg(data []byte) {
	var callBack msgCallBack
	if err := json.Unmarshal(data, &callBack); err == nil && callBack.Echo != "" {
		if ackInterface, exists := n.ackMng.pendingAcks.Load(callBack.Echo); exists {
			ack := ackInterface.(*pendingAck)
			if callBack.Status == "ok" {
				select {
				case ack.ch <- msgData{
					result: true,
					msgId:  callBack.Data.MessageId,
				}:
				default:
					log.Println("确认通道已满")
				}
			} else {
				select {
				case ack.ch <- msgData{
					result: false,
					msgId:  0,
				}:
				default:
					log.Println("确认通道已满")
				}
			}
		}
		return
	}

	var msg message.Message
	if err := json.Unmarshal(data, &msg); err != nil {
		log.Println("解析WebSocket消息失败:", err)
		return
	}

	if msg.PostType == "message" {
		switch msg.MessageType {
		case "group":
			if n.groupMsgFunc != nil {
				go n.groupMsgFunc(msg)
			}
		case "private":
			if n.friendMsgFunc != nil {
				go n.friendMsgFunc(msg)
			}
		}
	}
}

type wsPushGroupData struct {
	Action string `json:"action"`
	Params struct {
		GroupId uint                  `json:"group_id"`
		Message []message.OB11Segment `json:"message"`
	} `json:"params"`
	Echo string `json:"echo"`
}

type wsPushFriendData struct {
	Action string `json:"action"`
	Params struct {
		UserId  uint                  `json:"user_id"`
		Message []message.OB11Segment `json:"message"`
	} `json:"params"`
	Echo string `json:"echo"`
}

type msgCallBack struct {
	message.Response
	Echo string `json:"echo"`
}
