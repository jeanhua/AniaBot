package pluginantiwithdrawal

import (
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jeanhua/AniaBot/ania/utils"
	"github.com/jeanhua/AniaBot/common/bot"
	"github.com/jeanhua/AniaBot/common/model/command"
	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/jeanhua/AniaBot/common/msgchain"
	"github.com/jeanhua/AniaBot/common/plugin"
	"github.com/spf13/viper"
)

type AntiWithdrawalPlugin struct {
	plugin.Meta
	msg     sync.Map
	adminId uint
}

func NewPlugin() *AntiWithdrawalPlugin {
	p := &AntiWithdrawalPlugin{}
	p.Name = "群防撤回插件"
	p.HelpWords = "群聊回顾最近的n条消息，发送 /explore [n] 获取，n<=100，默认50"
	p.AdminOnly = false
	return p
}

func (p *AntiWithdrawalPlugin) OnGroupMsg(bot bot.Bot, cmd *command.Command, msg message.Message) bool {
	queueI, _ := p.msg.LoadOrStore(msg.GroupId, NewMessageQueue[*message.Message](100))
	queue := queueI.(*MessageQueue[*message.Message])
	if cmd != nil && cmd.Mention && cmd.Name == "explore" {
		n := 50
		if len(cmd.Args) >= 1 {
			num, err := strconv.Atoi(cmd.Args[0])
			if err == nil && num > 0 && num <= 100 {
				n = num
			}
		}
		cachemsg := queue.Get(n)
		fbuilder := msgchain.Builder.Forward()
		ncrkey, existRkey := bot.GetNCrkey()
		for _, m := range cachemsg {
			_builder := msgchain.Builder.Group()
			for i, seg := range m.Message {
				switch seg.Type {
				case "text", "face", "at", "reply", "json", "music":
					_builder.Raw(m.Message[i])
				case "forward":
					_builder.Text("[转发消息，暂不支持查看]")
				case "image":
					if !existRkey {
						if isAfter3Minute(m.Time) {
							_builder.Text("[图片消息，已经超过3分钟过期时间]")
						} else {
							_builder.Raw(m.Message[i])
						}
					} else {
						key := ""
						for _, k := range ncrkey {
							if k.Type == 20 {
								key = strings.TrimPrefix(ncrkey[1].Rkey, "&rkey=")
							}
						}
						if key == "" {
							log.Println("无法解析图片URL")
							return true
						}
						link := m.Message[i].Data["url"].(string)
						if link != "" {
							if modifyer, err := utils.NewURLModifier(link); err != nil {
								log.Println("无法解析图片URL")
								return true
							} else {
								newLink := modifyer.SetQuery("rkey", key).String()
								_builder.ImageUrl(newLink)
							}
						}
					}
				case "record":
					_builder.Text("[语音消息]")
				case "video":
					if isAfter3Minute(m.Time) {
						_builder.Text("[视频消息，已经超过3分钟过期时间]")
					} else {
						_builder.Raw(m.Message[i])
					}
				case "file":
					if isAfter3Minute(m.Time) {
						_builder.Text("[文件消息，已经超过3分钟过期时间]")
					} else {
						_builder.Raw(m.Message[i])
					}
				}
			}
			fbuilder.Message(m.Sender.UserId, m.Sender.Nickname, _builder.Build())
		}
		_, success := bot.SendGroupForwardMsg(msg.GroupId, fbuilder.Build())
		if !success {
			log.Println("[群聊防撤回插件]: 无法转发消息")
		}
		return false
	}
	queue.Add(&msg)
	return true
}

func isAfter3Minute(timestamp uint) bool {
	now := uint(time.Now().Unix())
	if now-timestamp > 180 {
		return true
	}
	return false
}

func (p *AntiWithdrawalPlugin) Start(cfg *viper.Viper) {
	p.adminId = cfg.GetUint("bot.admin_id")
}

func (p *AntiWithdrawalPlugin) OnFriendMsg(bot bot.Bot, cmd *command.Command, msg message.Message) bool {
	if cmd != nil && cmd.Name == "explore" && msg.Sender.UserId == p.adminId {
		if len(cmd.Args) == 0 {
			builder := msgchain.Builder.Friend()
			builder.Text("请输入完整参数 /explore [Group ID] [count]")
			bot.SendFriendMsg(msg.Sender.UserId, builder.Build())
			return false
		} else if len(cmd.Args) >= 1 {
			n := 50
			Gid, err := strconv.Atoi(cmd.Args[0])
			if err != nil {
				builder := msgchain.Builder.Friend()
				builder.Text("请输入正确参数:Group ID, 语法: /explore [Group ID] [count](option)")
				bot.SendFriendMsg(msg.Sender.UserId, builder.Build())
				return false
			}
			if len(cmd.Args) == 2 {
				num, err := strconv.Atoi(cmd.Args[1])
				if err == nil && num > 0 && num <= 100 {
					n = num
				}
			}
			queueI, ok := p.msg.Load(uint(Gid))
			if !ok {
				builder := msgchain.Builder.Friend()
				builder.Text("请输入正确参数:Group ID Error, 语法: /explore [Group ID] [count](option)")
				bot.SendFriendMsg(msg.Sender.UserId, builder.Build())
				return false
			}
			queue := queueI.(*MessageQueue[*message.Message])
			cachemsg := queue.Get(n)
			fbuilder := msgchain.Builder.Forward()
			ncrkey, existRkey := bot.GetNCrkey()
			for _, m := range cachemsg {
				_builder := msgchain.Builder.Group()
				for i, seg := range m.Message {
					switch seg.Type {
					case "text", "face", "at", "reply", "json", "music":
						_builder.Raw(m.Message[i])
					case "forward":
						_builder.Text("[转发消息，暂不支持查看]")
					case "image":
						if !existRkey {
							if isAfter3Minute(m.Time) {
								_builder.Text("[图片消息，已经超过3分钟过期时间]")
							} else {
								_builder.Raw(m.Message[i])
							}
						} else {
							key := ""
							for _, k := range ncrkey {
								if k.Type == 20 {
									key = strings.TrimPrefix(ncrkey[1].Rkey, "&rkey=")
								}
							}
							if key == "" {
								log.Println("无法解析图片URL")
								return true
							}
							link := m.Message[i].Data["url"].(string)
							if link != "" {
								if modifyer, err := utils.NewURLModifier(link); err != nil {
									log.Println("无法解析图片URL")
									return true
								} else {
									newLink := modifyer.SetQuery("rkey", key).String()
									_builder.ImageUrl(newLink)
								}
							}
						}
					case "record":
						_builder.Text("[语音消息]")
					case "video":
						if isAfter3Minute(m.Time) {
							_builder.Text("[视频消息，已经超过3分钟过期时间]")
						} else {
							_builder.Raw(m.Message[i])
						}
					case "file":
						if isAfter3Minute(m.Time) {
							_builder.Text("[文件消息，已经超过3分钟过期时间]")
						} else {
							_builder.Raw(m.Message[i])
						}
					}
				}
				fbuilder.Message(m.Sender.UserId, m.Sender.Nickname, _builder.Build())
			}
			_, success := bot.SendFriendForwardMsg(msg.Sender.UserId, fbuilder.Build())
			if !success {
				log.Println("[群聊防撤回插件]: 无法转发消息")
			}
			return false
		}
	}
	return true
}
