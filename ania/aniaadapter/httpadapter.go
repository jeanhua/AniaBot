package aniaadapter

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

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
	log.Printf("Server starting on http://localhost:%d...\n", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
}

func (n *napcatHttpAdapter) handler(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error reading request body: %v", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()
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
	if _, err := n.httpClient.R().SetResult(&resp).SetBody(data).Post(n.baseUrl + "/send_group_msg"); err != nil {
		log.Println("HTTP请求失败: ", err.Error())
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
	if _, err := n.httpClient.R().SetResult(&resp).SetBody(data).Post(n.baseUrl + "/send_private_msg"); err != nil {
		log.Println("HTTP请求失败: ", err.Error())
	}

	if resp.Status == "ok" {
		return true, resp.Data.MessageId
	} else {
		return false, 0
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
