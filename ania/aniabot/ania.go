package aniabot

import (
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/jeanhua/AniaBot/ania/utils"
	"github.com/jeanhua/AniaBot/common/adapter"
	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/jeanhua/AniaBot/common/msgchain"
	"github.com/jeanhua/AniaBot/common/plugin"
	"github.com/spf13/viper"
)

type AniaBot struct {
	adapter adapter.Adapter
	plugins []plugin.Plugin
	admin   uint
	cfg     *viper.Viper
}

func NewAniaBot(adapter adapter.Adapter) *AniaBot {
	return &AniaBot{
		adapter: adapter,
	}
}

func NewAniaBotWithConfig(adapter adapter.Adapter, config *viper.Viper) *AniaBot {
	return &AniaBot{
		adapter: adapter,
		cfg:     config,
	}
}

func (ania *AniaBot) Run() {
	sort.SliceStable(ania.plugins, func(i, j int) bool {
		return ania.plugins[i].GetMeta().Order < ania.plugins[j].GetMeta().Order
	})
	trigger := adapter.TriggerWrapper{
		OnGroupMsg:          ania.onGroupEvent,
		OnFriendMsg:         ania.onFriendEvent,
		OnGroupUpload:       ania.onGroupUploadEvent,
		OnGroupAdmin:        ania.onGroupAdminEvent,
		OnGroupDecrease:     ania.onGroupDecreaseEvent,
		OnGroupIncrease:     ania.onGroupIncreaseEvent,
		OnGroupBan:          ania.onGroupBanEvent,
		OnFriendAdd:         ania.onFriendAddEvent,
		OnGroupRecall:       ania.onGroupRecallEvent,
		OnFriendRecall:      ania.onFriendRecallEvent,
		OnPoke:              ania.onPokeEvent,
		OnLuckyKing:         ania.onLuckyKingEvent,
		OnHonor:             ania.onHonorEvent,
		OnGroupMsgEmojiLike: ania.onGroupMsgEmojiLikeEvent,
		OnEssence:           ania.onEssenceEvent,
		OnGroupCard:         ania.onGroupCardEvent,
	}
	ania.adapter.SetTrigger(trigger)
	// config
	if ania.cfg == nil {
		cfg := viper.New()
		cfg.AddConfigPath("./")
		cfg.SetConfigName("config.dev")
		if err := cfg.ReadInConfig(); err == nil {
			log.Println("使用开发环境配置: config.dev.yaml")
		} else {
			cfg.SetConfigName("config")
			if err := cfg.ReadInConfig(); err != nil {
				log.Fatalf("无法读取配置文件: %v", err)
			}
			log.Println("使用默认配置: config.yaml")
		}
		ania.cfg = cfg
	}

	ania.admin = ania.cfg.GetUint("bot.admin_id")

	// 初始化事件
	log.Println("开始初始化插件...")
	for _, p := range ania.plugins {
		log.Println("初始化插件: ", p.GetMeta().Name)
		p.Start(ania.cfg)
	}

	ania.adapter.Serve(ania.cfg)
}

func (ania *AniaBot) onGroupEvent(msg message.Message) {
	defer func() {
		if err := recover(); err != nil {
			log.Println("群聊消息事件触发错误: ", err)
		}
	}()
	if msg.Sender.UserId == msg.SelfId {
		return
	}

	cmd := utils.ParseCommand(msg)
	if cmd != nil && cmd.Name == "help" && cmd.Mention {
		var pluginInfo strings.Builder
		pluginInfo.WriteString("\n欢迎使用AniaBot，已加载插件:")
		idx := 1
		for _, p := range ania.plugins {
			if p.GetMeta().AdminOnly && msg.Sender.UserId != ania.admin {
				continue
			}
			pName := p.GetMeta().Name
			pHelpWords := p.GetMeta().HelpWords
			pluginInfo.WriteString(fmt.Sprintf("\n%d. %s: %s", idx, pName, pHelpWords))
			idx += 1
		}
		c := msgchain.Builder.Group()
		c.Mention(msg.Sender.UserId)
		c.Text(pluginInfo.String())
		_, ok := ania.SendGroupMsg(msg.GroupId, c.Build())
		if !ok {
			log.Println("Bot消息发送失败，无法响应 /help")
		}
		return
	}

	for _, p := range ania.plugins {
		next := safeExecuteWithReturn("群聊消息事件", p, func(p plugin.Plugin) bool {
			return p.OnGroupMsg(ania, cmd, msg)
		})
		if !next {
			break
		}
	}
}

func (ania *AniaBot) onFriendEvent(msg message.Message) {
	defer func() {
		if err := recover(); err != nil {
			log.Println("私聊消息事件触发错误: ", err)
		}
	}()
	if msg.Sender.UserId == msg.SelfId {
		return
	}

	cmd := utils.ParseCommand(msg)
	if cmd != nil && cmd.Name == "help" {
		var pluginInfo strings.Builder
		pluginInfo.WriteString("欢迎使用AniaBot，已加载插件:")
		idx := 1
		for _, p := range ania.plugins {
			if p.GetMeta().AdminOnly && msg.Sender.UserId != ania.admin {
				continue
			}
			pName := p.GetMeta().Name
			pHelpWords := p.GetMeta().HelpWords
			pluginInfo.WriteString(fmt.Sprintf("\n%d. %s: %s", idx, pName, pHelpWords))
			idx += 1
		}
		c := msgchain.Builder.Friend()
		c.Text(pluginInfo.String())
		_, ok := ania.SendFriendMsg(msg.Sender.UserId, c.Build())
		if !ok {
			log.Println("Bot消息发送失败，无法响应 /help")
		}
		return
	}

	for _, p := range ania.plugins {
		next := safeExecuteWithReturn("私聊消息事件", p, func(p plugin.Plugin) bool {
			return p.OnFriendMsg(ania, cmd, msg)
		})
		if !next {
			break
		}
	}
}

func (ania *AniaBot) AddPlugin(p plugin.Plugin) {
	ania.plugins = append(ania.plugins, p)
	log.Println("已添加插件: ", p.GetMeta().Name)
}

func (ania *AniaBot) SendGroupMsg(groupId uint, chain msgchain.Chain) (msgId uint, success bool) {
	return ania.adapter.SendGroupMsg(groupId, chain)
}

func (ania *AniaBot) SendFriendMsg(userID uint, chain msgchain.Chain) (msgId uint, success bool) {
	return ania.adapter.SendFriendMsg(userID, chain)
}

func (ania *AniaBot) SendGroupAIVoiceMsg(groupId uint, character, msg string) (msgId uint, success bool) {
	return ania.adapter.SendGroupAIVoiceMsg(groupId, character, msg)
}

func (ania *AniaBot) SendPokeMsg(userId uint, groupId *uint) {
	ania.adapter.SendPokeMsg(userId, groupId)
}

func (ania *AniaBot) GetMsgDetail(msgId uint) (*message.Message, bool) {
	return ania.adapter.GetMsgDetail(msgId)
}

func (ania *AniaBot) SendGroupForwardMsg(groupId uint, chain msgchain.ForwardChain) (msgId uint, success bool) {
	return ania.adapter.SendGroupForwardMsg(groupId, chain)
}

func (ania *AniaBot) SendFriendForwardMsg(userId uint, chain msgchain.ForwardChain) (msgId uint, success bool) {
	return ania.adapter.SendFriendForwardMsg(userId, chain)
}

func (ania *AniaBot) GetGroupUserInfo(groupId, userId uint) (*message.GroupUserInfo, bool) {
	return ania.adapter.GetGroupUserInfo(groupId, userId)
}

func (ania *AniaBot) GetNCrkey() ([]message.NCrkey, bool) {
	return ania.adapter.GetNCrkey()
}
