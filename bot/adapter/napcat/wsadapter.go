package napcat

import (
	"encoding/json"
	"fmt"
	"log"
	"runtime"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/jeanhua/AniaBot/common/adapter"
	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/jeanhua/AniaBot/common/msgchain"
	"github.com/spf13/viper"
)

type napcatWebSocketAdapter struct {
	wsConn      *websocket.Conn
	trigger     adapter.TriggerWrapper
	ackMng      *ackManager
	mu          sync.Mutex
	msgCh       chan []byte
	workerWg    sync.WaitGroup
	workerCount int
	queueSize   int
}

type ackManager struct {
	pendingAcks sync.Map
	timeout     time.Duration
}

type wsPushData[T any] struct {
	Action string `json:"action"`
	Params T      `json:"params"`
	Echo   string `json:"echo"`
}

func request[T any](n *napcatWebSocketAdapter, action string, params any, prefix string) (*T, bool) {

	echo := fmt.Sprintf("%s:%d", prefix, time.Now().UnixNano())

	req := wsPushData[any]{
		Action: action,
		Params: params,
		Echo:   echo,
	}
	b, err := json.Marshal(req)
	if err != nil {
		log.Printf("[%s] 序列化失败: %v", action, err)
		return nil, false
	}

	resCh := make(chan []byte, 1)
	n.ackMng.pendingAcks.Store(echo, resCh)
	defer n.ackMng.pendingAcks.Delete(echo)

	n.mu.Lock()
	err = n.wsConn.WriteMessage(websocket.TextMessage, b)
	n.mu.Unlock()
	if err != nil {
		log.Printf("[%s] 发送失败: %v", action, err)
		return nil, false
	}

	timer := time.NewTimer(n.ackMng.timeout)
	defer timer.Stop()

	select {
	case data := <-resCh:
		var resp message.Response[T]
		if err := json.Unmarshal(data, &resp); err != nil {
			log.Printf("[%s] 解析响应失败: %v", action, err)
			return nil, false
		}
		if resp.Status != "ok" {
			log.Println("消息发送失败", string(data))
			return nil, false
		}
		return &resp.Data, true
	case <-timer.C:
		log.Printf("[%s] 请求超时, echo: %s", action, echo)
		return nil, false
	}
}

func (n *napcatWebSocketAdapter) SendGroupMsg(groupId uint, chain msgchain.GroupChain) (uint, bool) {
	params := map[string]any{"group_id": groupId, "message": chain.GetGroupMsg()}
	res, ok := request[message.Message](n, "send_group_msg", params, "ack")
	if !ok || res == nil {
		return 0, false
	}
	return res.MessageId, true
}

func (n *napcatWebSocketAdapter) SendGroupAIVoiceMsg(groupId uint, character, msg string) (uint, bool) {
	params := message.AiVoiceMsg{GroupId: groupId, Character: character, Text: msg}
	res, ok := request[message.Message](n, "send_group_ai_record", params, "ack")
	if !ok || res == nil {
		return 0, false
	}
	return res.MessageId, true
}

func (n *napcatWebSocketAdapter) SendFriendMsg(userId uint, chain msgchain.FriendChain) (uint, bool) {
	params := map[string]any{"user_id": userId, "message": chain.GetFriendMsg()}
	res, ok := request[message.Message](n, "send_private_msg", params, "ack")
	if !ok || res == nil {
		return 0, false
	}
	return res.MessageId, true
}

func (n *napcatWebSocketAdapter) SendGroupForwardMsg(groupId uint, chain msgchain.GroupForwardChain) (uint, bool) {
	params := message.GroupForwardMessage{GroupId: groupId, ForwardMessageSegment: chain.GetForwardMsg()}
	res, ok := request[message.Message](n, "send_forward_msg", params, "ack")
	if !ok || res == nil {
		return 0, false
	}
	return res.MessageId, true
}

func (n *napcatWebSocketAdapter) SendFriendForwardMsg(userId uint, chain msgchain.FriendForwardChain) (uint, bool) {
	params := message.FriendForwardMessage{UserId: userId, ForwardMessageSegment: chain.GetForwardMsg()}
	res, ok := request[message.Message](n, "send_forward_msg", params, "ack")
	if !ok || res == nil {
		return 0, false
	}
	return res.MessageId, true
}

func (n *napcatWebSocketAdapter) GetMsgDetail(msgId uint) (*message.Message, bool) {
	params := map[string]uint{"message_id": msgId}
	return request[message.Message](n, "get_msg", params, "dt")
}

func (n *napcatWebSocketAdapter) GetForwardMsg(msgId string) (*[]message.Message, bool) {
	params := map[string]string{"message_id": msgId}
	type fwData struct {
		Messages []message.Message `json:"messages"`
	}
	res, ok := request[fwData](n, "get_forward_msg", params, "fw")
	if !ok || res == nil {
		return nil, false
	}
	return &res.Messages, true
}

func (n *napcatWebSocketAdapter) GetGroupUserInfo(groupId, userId uint) (*message.GroupUserInfo, bool) {
	params := map[string]any{"group_id": groupId, "user_id": userId, "no_cache": true}
	return request[message.GroupUserInfo](n, "get_group_member_info", params, "ugif")
}

func (n *napcatWebSocketAdapter) GetNCrkey() ([]message.NCrkey, bool) {
	res, ok := request[[]message.NCrkey](n, "nc_get_rkey", struct{}{}, "ncrkey")
	if !ok || res == nil {
		return nil, false
	}
	return *res, true
}

func (n *napcatWebSocketAdapter) SendPokeMsg(userId uint, groupId *uint) {
	params := map[string]uint{"user_id": userId}
	if groupId != nil {
		params["group_id"] = *groupId
	}
	req := wsPushData[any]{Action: "send_poke", Params: params}
	if b, err := json.Marshal(req); err == nil {
		n.mu.Lock()
		n.wsConn.WriteMessage(websocket.TextMessage, b)
		n.mu.Unlock()
	}
}

func (n *napcatWebSocketAdapter) GetFriendList() (*[]message.Friend, bool) {
	res, ok := request[[]message.Friend](n, "get_friend_list", struct{}{}, "friend_list")
	if !ok || res == nil {
		return nil, false
	}
	return res, true
}

func (n *napcatWebSocketAdapter) GetGroupDetail(groupId uint) (*message.GroupInfo, bool) {
	params := map[string]uint{"group_id": groupId}
	return request[message.GroupInfo](n, "get_group_detail_info", params, "group_info")
}

func (n *napcatWebSocketAdapter) SetMsgEmojiLike(msgId uint, emojiId int, like bool) (success bool) {
	params := message.EmojiLike{MessageID: msgId, EmojiId: emojiId, Set: like}
	res, ok := request[json.RawMessage](n, "set_msg_emoji_like", params, "ack")
	if !ok || res == nil {
		return false
	}
	return true
}

func (n *napcatWebSocketAdapter) Serve(v *viper.Viper) {
	n.ackMng = &ackManager{timeout: time.Second * 10}
	url := v.GetString("bot.adapter.ws.address")
	maxRetries := v.GetInt("bot.adapter.ws.max_retries")
	if maxRetries <= 0 {
		maxRetries = 5
	}

	// worker pool configuration
	n.workerCount = v.GetInt("bot.adapter.ws.worker_count")
	if n.workerCount <= 0 {
		n.workerCount = runtime.NumCPU() * 2
	}
	n.queueSize = v.GetInt("bot.adapter.ws.worker_queue_size")
	if n.queueSize <= 0 {
		n.queueSize = 256
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
			log.Printf("连接失败 [%d/%d]: %v. %v 后重连...", i+1, maxRetries, err, waitTime)
			time.Sleep(waitTime)
		}
		if err != nil {
			log.Println("无法连接至服务器，程序退出")
			break
		}
		log.Println("WebSocket 连接成功！")
		n.wsConn = conn
		n.startWorkerPool(n.workerCount, n.queueSize)
		n.readLoop(conn)
		n.stopWorkerPool()
		n.wsConn = nil
		log.Println("连接断开，准备重连...")
	}

	log.Println("Websocket正在退出")
}

func (n *napcatWebSocketAdapter) readLoop(conn *websocket.Conn) {
	defer conn.Close()
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			log.Println("读取数据失败:", err)
			return
		}
		if n.msgCh != nil {
			select {
			case n.msgCh <- msg:
			default:
				log.Println("消息队列已满，丢弃消息")
			}
		} else {
			go n.onMsg(msg)
		}
	}
}

func (n *napcatWebSocketAdapter) SetTrigger(trigger adapter.TriggerWrapper) {
	n.trigger = trigger
}

func (n *napcatWebSocketAdapter) startWorkerPool(count, qsize int) {
	if count <= 0 {
		count = 4
	}
	if qsize <= 0 {
		qsize = 256
	}
	n.msgCh = make(chan []byte, qsize)
	for i := 0; i < count; i++ {
		n.workerWg.Add(1)
		go func() {
			defer n.workerWg.Done()
			for data := range n.msgCh {
				n.onMsg(data)
			}
		}()
	}
	log.Printf("启动 napcat websocket worker pool: workers=%d queue=%d", count, qsize)
}

func (n *napcatWebSocketAdapter) stopWorkerPool() {
	if n.msgCh == nil {
		return
	}
	close(n.msgCh)
	n.workerWg.Wait()
	n.msgCh = nil
	log.Println("已停止 napcat websocket worker pool")
}

func (n *napcatWebSocketAdapter) onMsg(data []byte) {
	var callBack map[string]any
	if err := json.Unmarshal(data, &callBack); err != nil {
		return
	}

	if echo, ok := callBack["echo"].(string); ok && echo != "" {
		if chIf, exists := n.ackMng.pendingAcks.Load(echo); exists {
			select {
			case chIf.(chan []byte) <- data:
			default:
			}
		}
		return
	}

	postType, _ := callBack["post_type"].(string)
	switch postType {
	case "message", "message_sent":
		var msg message.Message
		if err := json.Unmarshal(data, &msg); err == nil {
			if msg.MessageType == "group" && n.trigger.OnGroupMsg != nil {
				if msg.RawMessage != "" {
					n.trigger.OnGroupMsg(msg)
				}
			} else if msg.MessageType == "private" && msg.SubType == "friend" && n.trigger.OnFriendMsg != nil {
				if msg.RawMessage != "" {
					n.trigger.OnFriendMsg(msg)
				}
			}
		}
	case "notice":
		noticeType, _ := callBack["notice_type"].(string)
		wsSpreadNotice(n, noticeType, data)
	}
}

func wsSpreadNotice(n *napcatWebSocketAdapter, noticeType string, data []byte) {
	handleNotice(n.trigger, noticeType, data)
}
