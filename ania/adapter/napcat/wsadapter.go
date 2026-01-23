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
	Data   *message.Message
}

type groupInfoAck struct {
	ch chan *groupInfo
}

type groupInfo struct {
	result bool
	Data   *message.GroupUserInfo
}

type rkeyAck struct {
	ch chan *rkey
}

type rkey struct {
	result bool
	Data   []message.NCrkey
}

func (n *napcatWebSocketAdapter) Serve(v *viper.Viper) {
	n.ackMng = &ackManager{
		timeout: time.Second * 10,
	}

	url := v.GetString("bot.adapter.ws.address")
	maxRetries := v.GetInt("bot.adapter.ws.max_retries")
	if maxRetries <= 0 {
		maxRetries = 5
	}
	log.Println("已启用 napcat websocket adapter")

	for {
		var conn *websocket.Conn
		var err error
		for i := 0; i < maxRetries; i++ {
			conn, _, err = websocket.DefaultDialer.Dial(url, nil)
			if err == nil {
				break
			}
			waitTime := time.Second * time.Duration(1<<i)
			log.Printf("连接失败 [%d/%d]: %v. %v 后尝试重连...\n", i+1, maxRetries, err, waitTime)
			time.Sleep(waitTime)
		}
		if err != nil {
			log.Printf("无法连接至服务器，已达到最大重试次数 (%d)。程序将彻底退出。\n", maxRetries)
			log.Fatal(err)
		}
		log.Println("WebSocket 连接成功！")
		n.wsConn = conn
		n.readLoop(conn)
		log.Println("连接已断开，准备重新开始重连序列...")
	}
}

func (n *napcatWebSocketAdapter) readLoop(conn *websocket.Conn) {
	defer conn.Close()
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			log.Println("读取数据失败:", err)
			return
		}
		go n.onMsg(msg)
	}
}

func (n *napcatWebSocketAdapter) SetTrigger(trigger adapter.TriggerWrapper) {
	n.trigger = trigger
}

func (n *napcatWebSocketAdapter) SendGroupMsg(groupId uint, chain msgchain.Chain) (msgId uint, success bool) {
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
		return 0, false
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
		return 0, false
	}
	select {
	case result := <-ackChan:
		if result.result {
			return result.msgId, true
		} else {
			return 0, false
		}
	case <-timer.C:
		return 0, false
	}
}

func (n *napcatWebSocketAdapter) SendGroupAIVoiceMsg(groupId uint, character, msg string) (msgId uint, success bool) {
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
		return 0, false
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
		return 0, false
	}
	select {
	case result := <-ackChan:
		if result.result {
			return result.msgId, true
		} else {
			return 0, false
		}
	case <-timer.C:
		return 0, false
	}
}

func (n *napcatWebSocketAdapter) SendFriendMsg(userId uint, chain msgchain.Chain) (msgId uint, success bool) {
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
		return 0, false
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
		return 0, false
	}
	select {
	case result := <-ackChan:
		if result.result {
			return result.msgId, true
		} else {
			return 0, false
		}
	case <-timer.C:
		return 0, false
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

func (n *napcatWebSocketAdapter) SendGroupForwardMsg(groupId uint, chain msgchain.ForwardChain) (msgId uint, success bool) {
	messageID := generateMessageID("ack")
	raw := wsPushData[message.GroupForwardMessage]{
		Action: "send_forward_msg",
		Params: message.GroupForwardMessage{
			GroupId:        groupId,
			ForwardMessage: chain.GetMsg(),
		},
		Echo: messageID,
	}
	b, err := json.Marshal(&raw)
	if err != nil {
		log.Println("消息链序列化失败")
		return 0, false
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
		return 0, false
	}
	select {
	case result := <-ackChan:
		if result.result {
			return result.msgId, true
		} else {
			return 0, false
		}
	case <-timer.C:
		return 0, false
	}
}

func (n *napcatWebSocketAdapter) SendFriendForwardMsg(userId uint, chain msgchain.ForwardChain) (msgId uint, success bool) {
	messageID := generateMessageID("ack")
	raw := wsPushData[message.FriendForwardMessage]{
		Action: "send_forward_msg",
		Params: message.FriendForwardMessage{
			UserId:         userId,
			ForwardMessage: chain.GetMsg(),
		},
		Echo: messageID,
	}
	b, err := json.Marshal(&raw)
	if err != nil {
		log.Println("消息链序列化失败")
		return 0, false
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
		return 0, false
	}
	select {
	case result := <-ackChan:
		if result.result {
			return result.msgId, true
		} else {
			return 0, false
		}
	case <-timer.C:
		return 0, false
	}
}

func (n *napcatWebSocketAdapter) GetMsgDetail(msgId uint) (*message.Message, bool) {
	messageID := generateMessageID("dt")
	raw := wsPushData[map[string]uint]{}
	raw.Action = "get_msg"
	raw.Echo = messageID
	raw.Params = map[string]uint{
		"message_id": msgId,
	}
	b, err := json.Marshal(&raw)
	if err != nil {
		log.Println("消息链序列化失败")
		return nil, false
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
		return nil, false
	}
	select {
	case result := <-ackChan:
		if result.result {
			return result.Data, true
		} else {
			return nil, false
		}
	case <-timer.C:
		return nil, false
	}
}

func (n *napcatWebSocketAdapter) GetGroupUserInfo(groupId, userId uint) (*message.GroupUserInfo, bool) {
	messageID := generateMessageID("ugif")
	raw := wsPushData[map[string]any]{}
	raw.Action = "get_group_member_info"
	raw.Echo = messageID
	raw.Params = map[string]any{
		"group_id": groupId,
		"user_id":  userId,
		"no_cache": true,
	}
	b, err := json.Marshal(&raw)
	if err != nil {
		log.Println("消息链序列化失败")
		return nil, false
	}

	ackChan := make(chan *groupInfo, 1)
	timer := time.NewTimer(n.ackMng.timeout)
	n.ackMng.pendingAcks.Store(messageID, &groupInfoAck{
		ch: ackChan,
	})
	defer func() {
		timer.Stop()
		n.ackMng.pendingAcks.Delete(messageID)
	}()
	if err := n.wsConn.WriteMessage(websocket.TextMessage, b); err != nil {
		log.Println("消息发送失败:", err)
		return nil, false
	}
	select {
	case result := <-ackChan:
		if result.result {
			return result.Data, true
		} else {
			return nil, false
		}
	case <-timer.C:
		return nil, false
	}
}

func (n *napcatWebSocketAdapter) GetNCrkey() ([]message.NCrkey, bool) {
	messageID := generateMessageID("ncrkey")
	raw := wsPushData[struct{}]{}
	raw.Action = "nc_get_rkey"
	raw.Echo = messageID
	b, err := json.Marshal(&raw)
	if err != nil {
		log.Println("消息链序列化失败")
		return nil, false
	}

	ackChan := make(chan *rkey, 1)
	timer := time.NewTimer(n.ackMng.timeout)
	n.ackMng.pendingAcks.Store(messageID, &rkeyAck{
		ch: ackChan,
	})
	defer func() {
		timer.Stop()
		n.ackMng.pendingAcks.Delete(messageID)
	}()
	if err := n.wsConn.WriteMessage(websocket.TextMessage, b); err != nil {
		log.Println("消息发送失败:", err)
		return nil, false
	}
	select {
	case result := <-ackChan:
		if result.result {
			return result.Data, true
		} else {
			return nil, false
		}
	case <-timer.C:
		return nil, false
	}
}

func (n *napcatWebSocketAdapter) onMsg(data []byte) {
	var callBack map[string]any
	if err := json.Unmarshal(data, &callBack); err != nil {
		return
	}
	if _, exist := callBack["echo"]; !exist {
		postType := callBack["post_type"].(string)
		switch postType {
		case "message", "message_sent":
			var msg message.Message
			if err := json.Unmarshal(data, &msg); err != nil {
				log.Println("解析WebSocket消息失败:", err)
				return
			}
			switch msg.MessageType {
			case "group":
				if n.trigger.OnGroupMsg != nil {
					n.trigger.OnGroupMsg(msg)
				}
			case "private":
				if n.trigger.OnFriendMsg != nil {
					n.trigger.OnFriendMsg(msg)
				}
			}
		case "notice":
			noticeType := callBack["notice_type"].(string)
			wsSpreadNotice(n, noticeType, data)
		}
		return
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
	switch px[0] {
	case "ack":
		ack := ackInterface.(*pendingAck)
		var msgCallBack message.Response[message.Message]
		if err := json.Unmarshal(data, &msgCallBack); err != nil {
			log.Println("无法解析ACK", string(data))
			return
		}
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
		var msgCallBack message.Response[message.Message]
		if err := json.Unmarshal(data, &msgCallBack); err != nil {
			log.Println("无法解析消息详情", string(data))
			return
		}
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
	case "ugif":
		ack := ackInterface.(*groupInfoAck)
		var msgCallBack message.Response[message.GroupUserInfo]
		if err := json.Unmarshal(data, &msgCallBack); err != nil {
			log.Println("无法解析群用户信息", string(data))
			return
		}
		if msgCallBack.Status == "ok" {
			select {
			case ack.ch <- &groupInfo{
				Data:   &msgCallBack.Data,
				result: true,
			}:
			default:
				log.Println("确认通道已满, 无法获取消息详情")
			}
		} else {
			select {
			case ack.ch <- &groupInfo{
				Data:   nil,
				result: false,
			}:
			default:
				log.Println("确认通道已满, 无法获取消息详情")
			}
		}
	case "ncrkey":
		ack := ackInterface.(*rkeyAck)
		var msgCallBack message.Response[[]message.NCrkey]
		if err := json.Unmarshal(data, &msgCallBack); err != nil {
			log.Println("无法解析nc get rkey信息", string(data))
			return
		}
		if msgCallBack.Status == "ok" {
			select {
			case ack.ch <- &rkey{
				Data:   msgCallBack.Data,
				result: true,
			}:
			default:
				log.Println("确认通道已满, 无法获取消息详情")
			}
		} else {
			select {
			case ack.ch <- &rkey{
				Data:   nil,
				result: false,
			}:
			default:
				log.Println("确认通道已满, 无法获取消息详情")
			}
		}
	}
}

// wsSpreadNotice 通知事件分发
func wsSpreadNotice(n *napcatWebSocketAdapter, noticeType string, data []byte) {
	switch noticeType {
	case "group_upload":
		var notice message.GroupUploadNotice
		if err := json.Unmarshal(data, &notice); err != nil {
			log.Println("解析消息通知事件[group_upload]错误: ", err.Error())
			return
		}
		if n.trigger.OnGroupUpload != nil {
			n.trigger.OnGroupUpload(notice)
		}

	case "group_admin":
		var notice message.GroupAdminNotice
		if err := json.Unmarshal(data, &notice); err != nil {
			log.Println("解析消息通知事件[group_admin]错误: ", err.Error())
			return
		}
		if n.trigger.OnGroupAdmin != nil {
			n.trigger.OnGroupAdmin(notice)
		}

	case "group_decrease":
		var notice message.GroupDecreaseNotice
		if err := json.Unmarshal(data, &notice); err != nil {
			log.Println("解析消息通知事件[group_decrease]错误: ", err.Error())
			return
		}
		if n.trigger.OnGroupDecrease != nil {
			n.trigger.OnGroupDecrease(notice)
		}

	case "group_increase":
		var notice message.GroupIncreaseNotice
		if err := json.Unmarshal(data, &notice); err != nil {
			log.Println("解析消息通知事件[group_increase]错误: ", err.Error())
			return
		}
		if n.trigger.OnGroupIncrease != nil {
			n.trigger.OnGroupIncrease(notice)
		}

	case "group_ban":
		var notice message.GroupBanNotice
		if err := json.Unmarshal(data, &notice); err != nil {
			log.Println("解析消息通知事件[group_ban]错误: ", err.Error())
			return
		}
		if n.trigger.OnGroupBan != nil {
			n.trigger.OnGroupBan(notice)
		}

	case "friend_add":
		var notice message.FriendAddNotice
		if err := json.Unmarshal(data, &notice); err != nil {
			log.Println("解析消息通知事件[friend_add]错误: ", err.Error())
			return
		}
		if n.trigger.OnFriendAdd != nil {
			n.trigger.OnFriendAdd(notice)
		}

	case "group_recall":
		var notice message.GroupRecallNotice
		if err := json.Unmarshal(data, &notice); err != nil {
			log.Println("解析消息通知事件[group_recall]错误: ", err.Error())
			return
		}
		if n.trigger.OnGroupRecall != nil {
			n.trigger.OnGroupRecall(notice)
		}

	case "friend_recall":
		var notice message.FriendRecallNotice
		if err := json.Unmarshal(data, &notice); err != nil {
			log.Println("解析消息通知事件[friend_recall]错误: ", err.Error())
			return
		}
		if n.trigger.OnFriendRecall != nil {
			n.trigger.OnFriendRecall(notice)
		}

	case "poke":
		var notice message.PokeNotice
		if err := json.Unmarshal(data, &notice); err != nil {
			log.Println("解析消息通知事件[poke]错误: ", err.Error())
			return
		}
		if n.trigger.OnPoke != nil {
			n.trigger.OnPoke(notice)
		}

	case "lucky_king":
		var notice message.LuckyKingNotice
		if err := json.Unmarshal(data, &notice); err != nil {
			log.Println("解析消息通知事件[lucky_king]错误: ", err.Error())
			return
		}
		if n.trigger.OnLuckyKing != nil {
			n.trigger.OnLuckyKing(notice)
		}

	case "honor":
		var notice message.HonorNotice
		if err := json.Unmarshal(data, &notice); err != nil {
			log.Println("解析消息通知事件[honor]错误: ", err.Error())
			return
		}
		if n.trigger.OnHonor != nil {
			n.trigger.OnHonor(notice)
		}

	case "group_msg_emoji_like":
		var notice message.GroupMsgEmojiLikeNotice
		if err := json.Unmarshal(data, &notice); err != nil {
			log.Println("解析消息通知事件[group_msg_emoji_like]错误: ", err.Error())
			return
		}
		if n.trigger.OnGroupMsgEmojiLike != nil {
			n.trigger.OnGroupMsgEmojiLike(notice)
		}

	case "essence":
		var notice message.EssenceNotice
		if err := json.Unmarshal(data, &notice); err != nil {
			log.Println("解析消息通知事件[essence]错误: ", err.Error())
			return
		}
		if n.trigger.OnEssence != nil {
			n.trigger.OnEssence(notice)
		}

	case "group_card":
		var notice message.GroupCardNotice
		if err := json.Unmarshal(data, &notice); err != nil {
			log.Println("解析消息通知事件[group_card]错误: ", err.Error())
			return
		}
		if n.trigger.OnGroupCard != nil {
			n.trigger.OnGroupCard(notice)
		}

	default:
		log.Println("未知的通知类型: ", noticeType)
		return
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
