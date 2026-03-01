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
	httpClient *resty.Client
	trigger    adapter.TriggerWrapper
}

func (n *napcatHttpAdapter) Serve(v *viper.Viper) {
	n.httpClient = resty.New()
	n.baseUrl = strings.TrimRight(v.GetString("bot.adapter.http.target_url"), "/")
	http.HandleFunc("/", n.handler)
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
		noticeType := callBack["notice_type"].(string)
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

func (n *napcatHttpAdapter) SendGroupMsg(groupId uint, chain msgchain.GroupChain) (msgId uint, success bool) {
	data := httpGroupPushData{
		GroupId: groupId,
		Message: chain.GetGroupMsg(),
	}

	var resp message.Response[message.Message]
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	if _, err := n.httpClient.R().SetContext(ctx).SetResult(&resp).SetBody(data).Post(n.baseUrl + "/send_group_msg"); err != nil {
		log.Println("HTTP请求失败, 无法发送群聊消息: ", err.Error())
		return 0, false
	}
	if resp.Status == "ok" {
		return resp.Data.MessageId, true
	} else {
		return 0, false
	}
}

func (n *napcatHttpAdapter) SendFriendMsg(UserId uint, chain msgchain.FriendChain) (msgId uint, success bool) {
	data := httpFriendPushData{
		UserId:  UserId,
		Message: chain.GetFriendMsg(),
	}

	var resp message.Response[message.Message]
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	if _, err := n.httpClient.R().SetContext(ctx).SetResult(&resp).SetBody(data).Post(n.baseUrl + "/send_private_msg"); err != nil {
		log.Println("HTTP请求失败, 无法发送私聊消息: ", err.Error())
		return 0, false
	}

	if resp.Status == "ok" {
		return resp.Data.MessageId, true
	} else {
		return 0, false
	}
}

func (n *napcatHttpAdapter) SendGroupAIVoiceMsg(groupId uint, character, msg string) (msgId uint, success bool) {
	data := message.AiVoiceMsg{
		GroupId:   groupId,
		Character: character,
		Text:      msg,
	}
	var resp message.Response[message.Message]
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	if _, err := n.httpClient.R().SetContext(ctx).SetResult(&resp).SetBody(data).Post(n.baseUrl + "/send_group_ai_record"); err != nil {
		log.Println("HTTP请求失败, 无法发送AI语音消息: ", err.Error())
		return 0, false
	}

	if resp.Status == "ok" {
		return resp.Data.MessageId, true
	} else {
		return 0, false
	}
}

func (n *napcatHttpAdapter) SendPokeMsg(userId uint, groupId *uint) {
	data := map[string]uint{}
	data["user_id"] = userId
	if groupId != nil {
		data["group_id"] = *groupId
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	if _, err := n.httpClient.R().SetContext(ctx).SetBody(data).Post(n.baseUrl + "/send_poke"); err != nil {
		log.Println("HTTP请求失败, 无法发送戳一戳消息: ", err.Error())
		return
	}
}

func (n *napcatHttpAdapter) SendGroupForwardMsg(groupId uint, chain msgchain.GroupForwardChain) (msgId uint, success bool) {
	data := message.GroupForwardMessage{}
	data.GroupId = groupId
	data.ForwardMessageSegment = chain.GetForwardMsg()
	var resp message.Response[message.Message]
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	if _, err := n.httpClient.R().SetContext(ctx).SetResult(&resp).SetBody(data).Post(n.baseUrl + "/send_forward_msg"); err != nil {
		log.Println("HTTP请求失败, 无法发送群聊转发消息: ", err.Error())
		return 0, false
	}

	if resp.Status == "ok" {
		return resp.Data.MessageId, true
	} else {
		return 0, false
	}
}

func (n *napcatHttpAdapter) SendFriendForwardMsg(userId uint, chain msgchain.FriendForwardChain) (msgId uint, success bool) {
	data := message.FriendForwardMessage{}
	data.UserId = userId
	data.ForwardMessageSegment = chain.GetForwardMsg()
	var resp message.Response[message.Message]
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	if _, err := n.httpClient.R().SetContext(ctx).SetResult(&resp).SetBody(data).Post(n.baseUrl + "/send_forward_msg"); err != nil {
		log.Println("HTTP请求失败, 无法发送群聊转发消息: ", err.Error())
		return 0, false
	}

	if resp.Status == "ok" {
		return resp.Data.MessageId, true
	} else {
		return 0, false
	}
}

func (n *napcatHttpAdapter) GetMsgDetail(msgId uint) (*message.Message, bool) {
	data := map[string]uint{
		"message_id": msgId,
	}
	result := httpMsgDetail{}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	if _, err := n.httpClient.R().SetResult(&result).SetContext(ctx).SetBody(data).Post(n.baseUrl + "/get_msg"); err != nil {
		log.Println("HTTP请求失败, 无法获取消息详情: ", err.Error())
		return nil, false
	}
	return &result.Data, true
}

func (n *napcatHttpAdapter) GetForwardMsg(msgId string) (msgs *[]message.Message, success bool) {
	data := map[string]string{
		"message_id": msgId,
	}
	result := httpForwardMsgDetail{}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	if _, err := n.httpClient.R().SetResult(&result).SetContext(ctx).SetBody(data).Post(n.baseUrl + "/get_forward_msg"); err != nil {
		log.Println("HTTP请求失败, 无法获取消息详情: ", err.Error())
		return nil, false
	}
	return &result.Data, true
}

func (n *napcatHttpAdapter) GetGroupUserInfo(groupId, userId uint) (*message.GroupUserInfo, bool) {
	data := map[string]any{}
	data["group_id"] = groupId
	data["user_id"] = userId
	data["no_cache"] = true

	resp := message.Response[message.GroupUserInfo]{}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	if _, err := n.httpClient.R().SetResult(&resp).SetContext(ctx).SetBody(data).Post(n.baseUrl + "/get_group_member_info"); err != nil {
		log.Println("HTTP请求失败, 无法获取消息详情: ", err.Error())
		return nil, false
	}
	return &resp.Data, true
}

func (n *napcatHttpAdapter) GetNCrkey() ([]message.NCrkey, bool) {
	resp := message.Response[[]message.NCrkey]{}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	if _, err := n.httpClient.R().SetResult(&resp).SetContext(ctx).Post(n.baseUrl + "/nc_get_rkey"); err != nil {
		log.Println("HTTP请求失败, 无法获取消息详情: ", err.Error())
		return nil, false
	}
	return resp.Data, true
}

func (n *napcatHttpAdapter) GetFriendList() (*[]message.Friend, bool) {
	resp := message.Response[[]message.Friend]{}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	if _, err := n.httpClient.R().SetResult(&resp).SetContext(ctx).Post(n.baseUrl + "/get_friend_list"); err != nil {
		log.Println("HTTP请求失败, 无法获取好友列表: ", err.Error())
		return nil, false
	}
	return &resp.Data, true
}

func (n *napcatHttpAdapter) GetGroupDetail(groupId uint) (*message.GroupInfo, bool) {
	data := map[string]uint{
		"group_id": groupId,
	}
	resp := message.Response[message.GroupInfo]{}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	if _, err := n.httpClient.R().SetResult(&resp).SetContext(ctx).SetBody(data).Post(n.baseUrl + "/get_group_detail_info"); err != nil {
		log.Println("HTTP请求失败, 无法获取群聊详情: ", err.Error())
		return nil, false
	}
	return &resp.Data, true
}

func (n *napcatHttpAdapter) SetMsgEmojiLike(msgId uint, emojiId int, like bool) (success bool) {
	data := message.EmojiLike{
		MessageID: msgId,
		EmojiId:   emojiId,
		Set:       like,
	}
	resp := message.Response[json.RawMessage]{}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	if _, err := n.httpClient.R().SetResult(&resp).SetContext(ctx).SetBody(data).Post(n.baseUrl + "/set_msg_emoji_like"); err != nil {
		log.Println("HTTP请求失败, 无法设置消息表情点赞: ", err.Error())
		return false
	}
	return resp.Status == "ok"
}

func (n *napcatHttpAdapter) SendGroupSign(groupId uint) (success bool) {
	data := map[string]uint{
		"group_id": groupId,
	}
	resp := message.Response[json.RawMessage]{}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	if _, err := n.httpClient.R().SetResult(&resp).SetContext(ctx).SetBody(data).Post(n.baseUrl + "/send_group_sign"); err != nil {
		log.Println("HTTP请求失败, 无法发送群打卡: ", err.Error())
		return false
	}
	return true
}

func (n *napcatHttpAdapter) GetGroupMsgHistory(groupId uint, count int) (*[]message.Message, bool) {
	data := map[string]any{
		"group_id":    groupId,
		"count":       count,
		"message_seq": 0,
	}
	resp := message.Response[struct {
		Messages []message.Message `json:"messages"`
	}]{}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	if _, err := n.httpClient.R().SetResult(&resp).SetContext(ctx).SetBody(data).Post(n.baseUrl + "/get_group_msg_history"); err != nil {
		log.Println("HTTP请求失败, 无法获取群聊消息历史记录: ", err.Error())
		return nil, false
	}
	return &resp.Data.Messages, true
}

func (n *napcatHttpAdapter) GetFriendMsgHistory(userId uint, count int) (*[]message.Message, bool) {
	data := map[string]any{
		"user_id":     userId,
		"count":       count,
		"message_seq": 0,
	}
	resp := message.Response[struct {
		Messages []message.Message `json:"messages"`
	}]{}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	if _, err := n.httpClient.R().SetResult(&resp).SetContext(ctx).SetBody(data).Post(n.baseUrl + "/get_friend_msg_history"); err != nil {
		log.Println("HTTP请求失败, 无法获取好友消息历史记录: ", err.Error())
		return nil, false
	}
	return &resp.Data.Messages, true
}

type httpFriendPushData struct {
	UserId  uint                  `json:"user_id"`
	Message []message.OB11Segment `json:"message"`
}

type httpGroupPushData struct {
	GroupId uint                  `json:"group_id"`
	Message []message.OB11Segment `json:"message"`
}

type httpMsgDetail struct {
	Data message.Message `json:"data"`
}

type httpForwardMsgDetail struct {
	Data []message.Message `json:"data"`
}
