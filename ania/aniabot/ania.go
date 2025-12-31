package aniabot

import (
	"log"
	"sort"

	"github.com/jeanhua/AniaBot/common/adapter"
	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/jeanhua/AniaBot/common/msgchain"
	"github.com/jeanhua/AniaBot/common/plugin"
	"github.com/spf13/viper"
)

type AniaBot struct {
	adapter adapter.Adapter
	plugins []plugin.PluginWrapper
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
	ania.adapter.SetGroupMsgEvent(func(m message.Message) {
		for _, p := range ania.plugins {
			if p.Event != nil {
				next := p.Event.OnGroupMsg(ania, m)
				if !next {
					break
				}
			}
		}
	})
	ania.adapter.SetFriendMsgEvent(func(m message.Message) {
		for _, p := range ania.plugins {
			if p.Event != nil {
				next := p.Event.OnFriendMsg(ania, m)
				if !next {
					break
				}
			}
		}
	})
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

	// 初始化事件
	for _, p := range ania.plugins {
		if p.InitFunc != nil {
			p.InitFunc.Init(cfg)
		}
	}

	ania.adapter.Serve(cfg)
}

func (ania *AniaBot) AddPlugin(pluginPointer interface{}) {
	if meta, ok := pluginPointer.(plugin.Plugin); !ok {
		log.Fatal("停停停停停! 你的插件没有实现 plugin.Plugin 接口!")
	} else {
		wraper := plugin.PluginWrapper{
			Plugin:   meta,
			Event:    nil,
			InitFunc: nil,
		}

		// 基础消息事件
		if e, ok := pluginPointer.(plugin.BasicEvent); ok {
			wraper.Event = e
		}

		// 初始化事件
		if e, ok := pluginPointer.(plugin.InitialEvent); ok {
			wraper.InitFunc = e
		}

		ania.plugins = append(ania.plugins, wraper)
	}
}

func (ania *AniaBot) SendGroupMsg(groupId uint, chain msgchain.Chain) {
	ania.adapter.SendGroupMsg(groupId, chain)
}

func (ania *AniaBot) SendFriendMsg(friendId uint, chain msgchain.Chain) {
	ania.adapter.SendFriendMsg(friendId, chain)
}
