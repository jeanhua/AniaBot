package napcat

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/jeanhua/AniaBot/common/msgchain"
	"github.com/spf13/viper"
)

type napcatHttpAdapter struct {
	baseUrl       string
	httpClient    *resty.Client
	groupMsgFunc  func(message.Message)
	friendMsgFunc func(message.Message)
}

func (n *napcatHttpAdapter) Serve(v *viper.Viper) {
	n.httpClient = resty.New()
	n.baseUrl = v.GetString("bot.adapter.http.target_url")
	http.HandleFunc("/", n.handler)
	port := v.GetInt("bot.adapter.http.listen_port")
	log.Println("已启用napcat http adapter")
	log.Printf("本地HTTP服务器已启动 http://localhost:%d...\n", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
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
	var msg message.Message
	if err := json.Unmarshal(data, &msg); err != nil {
		log.Println("解析HTTP消息失败")
	}
	if msg.PostType == "message" {
		switch msg.MessageType {
		case "group":
			n.groupMsgFunc(msg)
		case "private":
			n.friendMsgFunc(msg)
		}
	}
}

func (n *napcatHttpAdapter) SetGroupMsgEvent(f func(message.Message)) {
	n.groupMsgFunc = f
}

func (n *napcatHttpAdapter) SetFriendMsgEvent(f func(message.Message)) {
	n.friendMsgFunc = f
}

func (n *napcatHttpAdapter) SendGroupMsg(groupId uint, chain msgchain.Chain) (success bool, msgId uint) {
	data := httpGroupPushData{
		GroupId: groupId,
		Message: chain.GetMsg(),
	}

	var resp message.Response
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	if _, err := n.httpClient.R().SetContext(ctx).SetResult(&resp).SetBody(data).Post(n.baseUrl + "/send_group_msg"); err != nil {
		log.Println("HTTP请求失败, 无法发送群聊消息: ", err.Error())
		return false, 0
	}
	if resp.Status == "ok" {
		return true, resp.Data.MessageId
	} else {
		return false, 0
	}
}

func (n *napcatHttpAdapter) SendFriendMsg(UserId uint, chain msgchain.Chain) (success bool, msgId uint) {
	data := httpFriendPushData{
		UserId:  UserId,
		Message: chain.GetMsg(),
	}

	var resp message.Response
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	if _, err := n.httpClient.R().SetContext(ctx).SetResult(&resp).SetBody(data).Post(n.baseUrl + "/send_private_msg"); err != nil {
		log.Println("HTTP请求失败, 无法发送私聊消息: ", err.Error())
		return false, 0
	}

	if resp.Status == "ok" {
		return true, resp.Data.MessageId
	} else {
		return false, 0
	}
}

func (n *napcatHttpAdapter) SendGroupAIVoiceMsg(groupId uint, character, msg string) (success bool, msgId uint) {
	data := message.AiVoiceMsg{
		GroupId:   groupId,
		Character: character,
		Text:      msg,
	}
	var resp message.Response
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	if _, err := n.httpClient.R().SetContext(ctx).SetResult(&resp).SetBody(data).Post(n.baseUrl + "/send_group_ai_record"); err != nil {
		log.Println("HTTP请求失败, 无法发送私聊消息: ", err.Error())
		return false, 0
	}

	if resp.Status == "ok" {
		return true, resp.Data.MessageId
	} else {
		return false, 0
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

type httpFriendPushData struct {
	UserId  uint                  `json:"user_id"`
	Message []message.OB11Segment `json:"message"`
}

type httpGroupPushData struct {
	GroupId uint                  `json:"group_id"`
	Message []message.OB11Segment `json:"message"`
}
