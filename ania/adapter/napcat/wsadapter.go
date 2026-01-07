package napcat

import (
	"encoding/json"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/jeanhua/AniaBot/common/adapter"
	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/jeanhua/AniaBot/common/msgchain"
	"github.com/spf13/viper"
)

type napcatWebSocketAdapter struct {
	wsConn  *websocket.Conn
	trigger adapter.TriggerWrapper
	ackMng  *ackManager
}

type ackManager struct {
	pendingAcks sync.Map
	timeout     time.Duration
}

type pendingAck struct {
	ch chan *msgData
}

type msgData struct {
	result bool
	msgId  uint
}

type detailAck struct {
	ch chan *detail
}

type detail struct {
	result bool
	Data   *message.Message `json:"data"`
}

func (n *napcatWebSocketAdapter) Serve(v *viper.Viper) {
	// initAck
	n.ackMng = &ackManager{
		timeout: time.Second * 5,
	}

	log.Println("已启用napcat websocket adapter")
	url := v.GetString("bot.adapter.ws.address")
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		log.Fatal("无法连接到napcat websocket服务器:", err)
	}
	n.wsConn = conn
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			log.Println("读取数据失败:", err)
			log.Println("正在结束进程")
			break
		}
		n.onMsg(msg)
	}
	if err := conn.Close(); err != nil {
		log.Println("关闭连接出现问题: ", err.Error())
		return
	}
}

func (n *napcatWebSocketAdapter) SetTrigger(trigger adapter.TriggerWrapper) {
	n.trigger = trigger
}

func (n *napcatWebSocketAdapter) SendGroupMsg(groupId uint, chain msgchain.Chain) (success bool, msgId uint) {
	messageID := generateMessageID("ack")
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
		return false, 0
	}

	ackChan := make(chan *msgData, 1)
	timer := time.NewTimer(n.ackMng.timeout)
	n.ackMng.pendingAcks.Store(messageID, &pendingAck{
		ch: ackChan,
	})
	defer func() {
		timer.Stop()
		n.ackMng.pendingAcks.Delete(messageID)
	}()
	if err := n.wsConn.WriteMessage(websocket.TextMessage, b); err != nil {
		log.Println("消息发送失败:", err)
		return false, 0
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

func (n *napcatWebSocketAdapter) SendGroupAIVoiceMsg(groupId uint, character, msg string) (success bool, msgId uint) {
	messageID := generateMessageID("ack")
	raw := wsPushGroupAIMsgData{
		Action: "send_group_ai_record",
		Params: message.AiVoiceMsg{
			GroupId:   groupId,
			Character: character,
			Text:      msg,
		},
		Echo: messageID,
	}
	b, err := json.Marshal(&raw)
	if err != nil {
		log.Println("消息链序列化失败")
		return false, 0
	}

	ackChan := make(chan *msgData, 1)
	timer := time.NewTimer(n.ackMng.timeout)
	n.ackMng.pendingAcks.Store(messageID, &pendingAck{
		ch: ackChan,
	})
	defer func() {
		timer.Stop()
		n.ackMng.pendingAcks.Delete(messageID)
	}()
	if err := n.wsConn.WriteMessage(websocket.TextMessage, b); err != nil {
		log.Println("消息发送失败:", err)
		return false, 0
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
	messageID := generateMessageID("ack")
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
		return false, 0
	}

	ackChan := make(chan *msgData, 1)
	timer := time.NewTimer(n.ackMng.timeout)
	n.ackMng.pendingAcks.Store(messageID, &pendingAck{
		ch: ackChan,
	})
	defer func() {
		timer.Stop()
		n.ackMng.pendingAcks.Delete(messageID)
	}()
	if err := n.wsConn.WriteMessage(websocket.TextMessage, b); err != nil {
		log.Println("消息发送失败:", err)
		return false, 0
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

func (n *napcatWebSocketAdapter) SendPokeMsg(userId uint, groupId *uint) {
	data := wsPushData[map[string]uint]{}
	data.Action = "send_poke"
	data.Params["user_id"] = userId
	if groupId != nil {
		data.Params["group_id"] = *groupId
	}
	b, err := json.Marshal(&data)
	if err != nil {
		log.Println("戳一戳消息序列化失败")
		return
	}
	if err := n.wsConn.WriteMessage(websocket.TextMessage, b); err != nil {
		log.Println("消息发送失败:", err)
		return
	}
}

func (n *napcatWebSocketAdapter) GetMsgDetail(msgId uint) (bool, *message.Message) {
	messageID := generateMessageID("dt")
	raw := wsPushData[map[string]uint]{}
	raw.Action = "get_msg"
	raw.Params = map[string]uint{
		"message_id": msgId,
	}
	b, err := json.Marshal(&raw)
	if err != nil {
		log.Println("消息链序列化失败")
		return false, nil
	}

	ackChan := make(chan *detail, 1)
	timer := time.NewTimer(n.ackMng.timeout)
	n.ackMng.pendingAcks.Store(messageID, &detailAck{
		ch: ackChan,
	})
	defer func() {
		timer.Stop()
		n.ackMng.pendingAcks.Delete(messageID)
	}()
	if err := n.wsConn.WriteMessage(websocket.TextMessage, b); err != nil {
		log.Println("消息发送失败:", err)
		return false, nil
	}
	select {
	case result := <-ackChan:
		if result.result {
			return true, result.Data
		} else {
			return false, nil
		}
	case <-timer.C:
		return false, nil
	}
}

func (n *napcatWebSocketAdapter) onMsg(data []byte) {
	var callBack map[string]any
	if err := json.Unmarshal(data, &callBack); err != nil {
		return
	}
	if _, exist := callBack["echo"]; !exist {
		var msg message.Message
		if err := json.Unmarshal(data, &msg); err != nil {
			log.Println("解析WebSocket消息失败:", err)
			return
		}

		if msg.PostType == "message" {
			switch msg.MessageType {
			case "group":
				if n.trigger.OnGroupMsg != nil {
					go n.trigger.OnGroupMsg(msg)
				}
			case "private":
				if n.trigger.OnFriendMsg != nil {
					go n.trigger.OnFriendMsg(msg)
				}
			}
		}
	}

	echo, ok := callBack["echo"].(string)
	if !ok {
		return
	}
	ackInterface, exists := n.ackMng.pendingAcks.Load(echo)
	if !exists {
		return
	}
	px := strings.SplitN(echo, ":", 2)
	var msgCallBack message.Response
	if err := json.Unmarshal(data, &msgCallBack); err != nil {
		return
	}
	switch px[0] {
	case "ack":
		ack := ackInterface.(*pendingAck)
		if msgCallBack.Status == "ok" {
			select {
			case ack.ch <- &msgData{
				result: true,
				msgId:  msgCallBack.Data.MessageId,
			}:
			default:
				log.Println("确认通道已满, 无法获取消息发送情况")
			}
		} else {
			select {
			case ack.ch <- &msgData{
				result: false,
				msgId:  0,
			}:
			default:
				log.Println("确认通道已满, 无法获取消息发送情况")
			}
		}
	case "dt":
		ack := ackInterface.(*detailAck)
		if msgCallBack.Status == "ok" {
			select {
			case ack.ch <- &detail{
				Data:   &msgCallBack.Data,
				result: true,
			}:
			default:
				log.Println("确认通道已满, 无法获取消息详情")
			}
		} else {
			select {
			case ack.ch <- &detail{
				Data:   nil,
				result: false,
			}:
			default:
				log.Println("确认通道已满, 无法获取消息详情")
			}
		}
	}
}

type wsPushData[T any] struct {
	Action string `json:"action"`
	Params T      `json:"params"`
	Echo   string `json:"echo"`
}

type wsPushGroupData wsPushData[struct {
	GroupId uint                  `json:"group_id"`
	Message []message.OB11Segment `json:"message"`
}]

type wsPushGroupAIMsgData wsPushData[message.AiVoiceMsg]

type wsPushFriendData wsPushData[struct {
	UserId  uint                  `json:"user_id"`
	Message []message.OB11Segment `json:"message"`
}]
