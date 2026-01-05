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
	plugins []plugin.PluginWrapper
	admin   uint
}

func NewAniaBot(adapter adapter.Adapter) *AniaBot {
	return &AniaBot{
		adapter: adapter,
	}
}

func (ania *AniaBot) Run() {
	sort.SliceStable(ania.plugins, func(i, j int) bool {
		return ania.plugins[i].Plugin.GetMeta().Order < ania.plugins[j].Plugin.GetMeta().Order
	})
	ania.adapter.SetGroupMsgEvent(ania.onGroupEvent)
	ania.adapter.SetFriendMsgEvent(ania.onFriendEvent)
	// config
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

	ania.admin = cfg.GetUint("bot.admin_id")

	// 初始化事件
	for _, p := range ania.plugins {
		if p.StartFunc != nil {
			p.StartFunc.Start(cfg)
		}
	}

	ania.adapter.Serve(cfg)
}

func (ania *AniaBot) onGroupEvent(msg message.Message) {
	if msg.Sender.UserId == msg.SelfId {
		return
	}

	text, mention := utils.ExtraMessageStr(msg)
	cmd := utils.ParseCommand(text)
	if cmd != nil && cmd.Name == "help" && mention {
		cmd.Mention = mention
		var pluginInfo strings.Builder
		pluginInfo.WriteString("\n欢迎使用AniaBot，已加载插件:")
		idx := 1
		for _, p := range ania.plugins {
			if p.Plugin.GetMeta().AdminOnly && msg.Sender.UserId != ania.admin {
				continue
			}
			pName := p.Plugin.GetMeta().Name
			pHelpWords := p.Plugin.GetMeta().HelpWords
			pluginInfo.WriteString(fmt.Sprintf("\n%d. %s: %s", idx, pName, pHelpWords))
			idx += 1
		}
		c := msgchain.Builder.Group()
		c.Mention(msg.Sender.UserId)
		c.Text(pluginInfo.String())
		ok, _ := ania.SendGroupMsg(msg.GroupId, c.Build())
		if !ok {
			log.Println("Bot消息发送失败，无法响应 /help")
		}
		return
	}

	for _, p := range ania.plugins {
		if p.Event != nil {
			next := p.Event.OnGroupMsg(ania, cmd, msg)
			if !next {
				break
			}
		}
	}
}

func (ania *AniaBot) onFriendEvent(msg message.Message) {
	if msg.Sender.UserId == msg.SelfId {
		return
	}

	text, mention := utils.ExtraMessageStr(msg)
	cmd := utils.ParseCommand(text)
	if cmd != nil && cmd.Name == "help" {
		cmd.Mention = mention
		var pluginInfo strings.Builder
		pluginInfo.WriteString("欢迎使用AniaBot，已加载插件:")
		idx := 1
		for _, p := range ania.plugins {
			if p.Plugin.GetMeta().AdminOnly && msg.Sender.UserId != ania.admin {
				continue
			}
			pName := p.Plugin.GetMeta().Name
			pHelpWords := p.Plugin.GetMeta().HelpWords
			pluginInfo.WriteString(fmt.Sprintf("\n%d. %s: %s", idx, pName, pHelpWords))
			idx += 1
		}
		c := msgchain.Builder.Friend()
		c.Text(pluginInfo.String())
		ok, _ := ania.SendFriendMsg(msg.Sender.UserId, c.Build())
		if !ok {
			log.Println("Bot消息发送失败，无法响应 /help")
		}
		return
	}

	for _, p := range ania.plugins {
		if p.Event != nil {
			next := p.Event.OnFriendMsg(ania, cmd, msg)
			if !next {
				break
			}
		}
	}
}

func (ania *AniaBot) AddPlugin(pluginPointer interface{}) {
	if meta, ok := pluginPointer.(plugin.Plugin); !ok {
		log.Fatal("停停停停停! 你的插件没有实现 plugin.Plugin 接口!")
	} else {
		wrapper := plugin.PluginWrapper{
			Plugin:    meta,
			Event:     nil,
			StartFunc: nil,
		}

		// 基础消息事件
		if e, ok := pluginPointer.(plugin.BasicEvent); ok {
			wrapper.Event = e
		}

		// 初始化事件
		if e, ok := pluginPointer.(plugin.StartupEvent); ok {
			wrapper.StartFunc = e
		}

		ania.plugins = append(ania.plugins, wrapper)
	}
}

func (ania *AniaBot) SendGroupMsg(groupId uint, chain msgchain.Chain) (success bool, msgId uint) {
	return ania.adapter.SendGroupMsg(groupId, chain)
}

func (ania *AniaBot) SendFriendMsg(userID uint, chain msgchain.Chain) (success bool, msgId uint) {
	return ania.adapter.SendFriendMsg(userID, chain)
}

func (ania *AniaBot) SendGroupAIVoiceMsg(groupId uint, character, msg string) (success bool, msgId uint) {
	return ania.adapter.SendGroupAIVoiceMsg(groupId, character, msg)
}

func (ania *AniaBot) SendPokeMsg(userId uint, groupId *uint) {
	ania.adapter.SendPokeMsg(userId, groupId)
}
