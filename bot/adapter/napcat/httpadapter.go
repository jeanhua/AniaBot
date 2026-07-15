package napcat

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/jeanhua/AniaBot/common/adapter"
	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/jeanhua/AniaBot/common/msgchain"
	"github.com/spf13/viper"
)

type napcatHttpAdapter struct {
	baseUrl    string
	token      *string
	httpClient *resty.Client
	trigger    adapter.TriggerWrapper
}

const defaultTimeout = time.Second * 5

func (n *napcatHttpAdapter) createContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), defaultTimeout)
}

func (n *napcatHttpAdapter) postAndCheck(url string, body any, result any) bool {
	ctx, cancel := n.createContext()
	defer cancel()

	req := n.httpClient.R().SetContext(ctx)
	if body != nil {
		req = req.SetBody(body)
	}
	if result != nil {
		req = req.SetResult(result)
	}
	if n.token != nil {
		req = req.SetQueryParam("access_token", *n.token)
	}
	resp, err := req.Post(url)
	if err != nil {
		log.Printf("HTTP请求失败: %v", err)
		return false
	}
	if resp.StatusCode() != http.StatusOK {
		log.Printf("HTTP响应异常: %d", resp.StatusCode())
		return false
	}
	return true
}

func checkResponseStatus[T any](resp *message.Response[T]) bool {
	return resp != nil && resp.Status == "ok"
}

func (n *napcatHttpAdapter) Serve(v *viper.Viper) {
	n.httpClient = resty.New()
	n.baseUrl = strings.TrimRight(v.GetString("bot.adapter.http.target_url"), "/")
	http.HandleFunc("/", n.handler)
	if v.IsSet("bot.adapter.token") {
		token := v.GetString("bot.adapter.token")
		n.token = &token
	}
	port := v.GetInt("bot.adapter.http.listen_port")
	log.Println("已启用napcat http adapter")
	log.Printf("本地HTTP服务器已启动 http://localhost:%d...\n", port)
	http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
	log.Println("HTTP服务器已停止，正在退出")
}

func (n *napcatHttpAdapter) handler(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("无法读取HTTP请求内容: %v", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	defer func() {
		if err := r.Body.Close(); err != nil {
			log.Printf("关闭HTTP请求体出错: %v", err.Error())
		}
	}()
	n.onMsg(body)
}

func (n *napcatHttpAdapter) onMsg(data []byte) {
	var callBack map[string]any
	if err := json.Unmarshal(data, &callBack); err != nil {
		return
	}
	postType, _ := callBack["post_type"].(string)
	switch postType {
	case "message", "message_sent":
		var msg message.Message
		if err := json.Unmarshal(data, &msg); err != nil {
			log.Println("解析HTTP消息失败:", err)
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
		noticeType, _ := callBack["notice_type"].(string)
		httpSpreadNotice(n, noticeType, data)
	}
}

// httpSpreadNotice 通知事件分发
func httpSpreadNotice(n *napcatHttpAdapter, noticeType string, data []byte) {
	handleNotice(n.trigger, noticeType, data)
}

func (n *napcatHttpAdapter) SetTrigger(trigger adapter.TriggerWrapper) {
	n.trigger = trigger
}

func (n *napcatHttpAdapter) SendGroupMsg(groupId message.QID, chain msgchain.GroupChain) (msgId message.QID, success bool) {
	data := httpGroupPushData{
		GroupId: groupId,
		Message: chain.GetGroupMsg(),
	}

	var resp message.Response[message.Message]
	if !n.postAndCheck(n.baseUrl+"/send_group_msg", data, &resp) {
		return 0, false
	}
	return resp.Data.MessageId, checkResponseStatus(&resp)
}

func (n *napcatHttpAdapter) SendFriendMsg(userId message.QID, chain msgchain.FriendChain) (msgId message.QID, success bool) {
	data := httpFriendPushData{
		UserId:  userId,
		Message: chain.GetFriendMsg(),
	}

	var resp message.Response[message.Message]
	if !n.postAndCheck(n.baseUrl+"/send_private_msg", data, &resp) {
		return 0, false
	}
	return resp.Data.MessageId, checkResponseStatus(&resp)
}

func (n *napcatHttpAdapter) SendGroupAIVoiceMsg(groupId message.QID, character, msg string) (msgId message.QID, success bool) {
	data := message.AiVoiceMsg{
		GroupId:   groupId,
		Character: character,
		Text:      msg,
	}
	var resp message.Response[message.Message]
	if !n.postAndCheck(n.baseUrl+"/send_group_ai_record", data, &resp) {
		return 0, false
	}
	return resp.Data.MessageId, checkResponseStatus(&resp)
}

func (n *napcatHttpAdapter) SendPokeMsg(userId message.QID, groupId *message.QID) (success bool) {
	data := map[string]message.QID{}
	data["user_id"] = userId
	if groupId != nil {
		data["group_id"] = *groupId
	}
	return n.postAndCheck(n.baseUrl+"/send_poke", data, nil)
}

func (n *napcatHttpAdapter) SendGroupForwardMsg(groupId message.QID, chain msgchain.GroupForwardChain) (msgId message.QID, success bool) {
	data := message.GroupForwardMessage{
		GroupId:               groupId,
		ForwardMessageSegment: chain.GetForwardMsg(),
	}
	var resp message.Response[message.Message]
	if !n.postAndCheck(n.baseUrl+"/send_forward_msg", data, &resp) {
		return 0, false
	}
	return resp.Data.MessageId, checkResponseStatus(&resp)
}

func (n *napcatHttpAdapter) SendFriendForwardMsg(userId message.QID, chain msgchain.FriendForwardChain) (msgId message.QID, success bool) {
	data := message.FriendForwardMessage{
		UserId:                userId,
		ForwardMessageSegment: chain.GetForwardMsg(),
	}
	var resp message.Response[message.Message]
	if !n.postAndCheck(n.baseUrl+"/send_forward_msg", data, &resp) {
		return 0, false
	}
	return resp.Data.MessageId, checkResponseStatus(&resp)
}

func (n *napcatHttpAdapter) GetMsgDetail(msgId message.QID) (*message.Message, bool) {
	data := map[string]message.QID{"message_id": msgId}
	var resp message.Response[message.Message]
	if !n.postAndCheck(n.baseUrl+"/get_msg", data, &resp) {
		return nil, false
	}
	if !checkResponseStatus(&resp) {
		return nil, false
	}
	return &resp.Data, true
}

func (n *napcatHttpAdapter) GetForwardMsg(msgId message.QID) (msgs *[]message.Message, success bool) {
	data := map[string]message.QID{"message_id": msgId}
	var resp message.Response[httpForwardData]
	if !n.postAndCheck(n.baseUrl+"/get_forward_msg", data, &resp) {
		return nil, false
	}
	if !checkResponseStatus(&resp) {
		return nil, false
	}
	return &resp.Data.Messages, true
}

func (n *napcatHttpAdapter) GetGroupUserInfo(groupId, userId message.QID) (*message.GroupUserInfo, bool) {
	data := map[string]any{
		"group_id": groupId,
		"user_id":  userId,
		"no_cache": true,
	}
	resp := message.Response[message.GroupUserInfo]{}
	if !n.postAndCheck(n.baseUrl+"/get_group_member_info", data, &resp) {
		return nil, false
	}
	return &resp.Data, true
}

func (n *napcatHttpAdapter) GetNCrkey() ([]message.NCrkey, bool) {
	resp := message.Response[[]message.NCrkey]{}
	if !n.postAndCheck(n.baseUrl+"/nc_get_rkey", nil, &resp) {
		return nil, false
	}
	return resp.Data, true
}

func (n *napcatHttpAdapter) GetFriendList() (*[]message.Friend, bool) {
	resp := message.Response[[]message.Friend]{}
	if !n.postAndCheck(n.baseUrl+"/get_friend_list", nil, &resp) {
		return nil, false
	}
	return &resp.Data, true
}

func (n *napcatHttpAdapter) GetGroupDetail(groupId message.QID) (*message.GroupInfo, bool) {
	data := map[string]message.QID{"group_id": groupId}
	resp := message.Response[message.GroupInfo]{}
	if !n.postAndCheck(n.baseUrl+"/get_group_detail_info", data, &resp) {
		return nil, false
	}
	return &resp.Data, true
}

func (n *napcatHttpAdapter) SetMsgEmojiLike(msgId message.QID, emojiId int, like bool) bool {
	data := message.EmojiLike{
		MessageID: msgId,
		EmojiId:   emojiId,
		Set:       like,
	}
	resp := message.Response[json.RawMessage]{}
	if !n.postAndCheck(n.baseUrl+"/set_msg_emoji_like", data, &resp) {
		return false
	}
	return checkResponseStatus(&resp)
}

func (n *napcatHttpAdapter) SendGroupSign(groupId message.QID) bool {
	data := map[string]message.QID{"group_id": groupId}
	resp := message.Response[json.RawMessage]{}
	return n.postAndCheck(n.baseUrl+"/send_group_sign", data, &resp)
}

func (n *napcatHttpAdapter) GetGroupMsgHistory(groupId message.QID, count int, message_seq int) (*[]message.Message, bool) {
	data := map[string]any{
		"group_id":    groupId,
		"count":       count,
		"message_seq": message_seq,
	}
	var resp message.Response[httpForwardData]
	if !n.postAndCheck(n.baseUrl+"/get_group_msg_history", data, &resp) {
		return nil, false
	}
	if !checkResponseStatus(&resp) {
		return nil, false
	}
	return &resp.Data.Messages, true
}

func (n *napcatHttpAdapter) GetFriendMsgHistory(userId message.QID, count int, message_seq int) (*[]message.Message, bool) {
	data := map[string]any{
		"user_id":     userId,
		"count":       count,
		"message_seq": message_seq,
	}
	var resp message.Response[httpForwardData]
	if !n.postAndCheck(n.baseUrl+"/get_friend_msg_history", data, &resp) {
		return nil, false
	}
	if !checkResponseStatus(&resp) {
		return nil, false
	}
	return &resp.Data.Messages, true
}

func (n *napcatHttpAdapter) GetAIChatacter() (*[]message.AIChatacter, bool) {
	resp := message.Response[message.AIChatacterResp]{}
	if !n.postAndCheck(n.baseUrl+"/get_ai_chatacter", nil, &resp) {
		return nil, false
	}
	return &resp.Data.Characters, true
}

func (n *napcatHttpAdapter) GetPrivateFileURL(userId message.QID, fileId string) (string, bool) {
	data := map[string]any{
		"user_id": userId,
		"file_id": fileId,
	}
	type privateFileData struct {
		URL string `json:"url"`
	}
	var resp message.Response[privateFileData]
	if !n.postAndCheck(n.baseUrl+"/get_private_file_url", data, &resp) {
		return "", false
	}
	return resp.Data.URL, true
}

func (n *napcatHttpAdapter) GetGroupList() (*[]message.GroupInfo, bool) {
	resp := message.Response[[]message.GroupInfo]{}
	if !n.postAndCheck(n.baseUrl+"/get_group_list", nil, &resp) {
		return nil, false
	}
	return &resp.Data, true
}

type httpFriendPushData struct {
	UserId  message.QID           `json:"user_id"`
	Message []message.OB11Segment `json:"message"`
}

type httpGroupPushData struct {
	GroupId message.QID           `json:"group_id"`
	Message []message.OB11Segment `json:"message"`
}

// httpForwardData 对应 OneBot v11 get_forward_msg / *_msg_history 响应的 data 字段
type httpForwardData struct {
	Messages []message.Message `json:"messages"`
}
