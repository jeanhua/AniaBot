package pluginantiwithdrawal

import (
	"context"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jeanhua/AniaBot/bot/utils"
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

const (
	ResourceTimeout = 60 * 3 // 时间戳，3分钟
)

func (p *AntiWithdrawalPlugin) OnGroupMsg(ctx context.Context, bot bot.Bot, cmd command.Command, msg message.Message) (bool, error) {
	queueI, _ := p.msg.LoadOrStore(msg.GroupId, NewMessageQueue[*message.Message](100))
	queue := queueI.(*MessageQueue[*message.Message])
	if cmd.Mention && cmd.Name == "explore" {
		n := 50
		if len(cmd.Args) >= 1 {
			num, err := strconv.Atoi(cmd.Args[0])
			if err == nil && num > 0 && num <= 100 {
				n = num
			}
		}
		cachemsg := queue.Get(n)
		if len(cachemsg) == 0 {
			builder := msgchain.Builder().Group()
			builder.Text("暂时没有保存到什么消息哦，请稍后再试")
			builder.Face(14)
			bot.SendGroupMsg(msg.GroupId, builder.Build())
			return false, nil
		}
		fbuilder := msgchain.Builder().GroupForward()
		ncrkey, existRkey := bot.GetNCrkey()
		for _, m := range cachemsg {
			_builder := msgchain.Builder().Group()
			for i, seg := range m.Message {
				switch seg.Type {
				case "text", "face", "at", "reply", "json", "music":
					_builder.Raw(m.Message[i])
				case "forward":
					_builder.Text("[转发消息，暂不支持查看]")
				case "image", "file":
					if !existRkey {
						if isTimeout(m.Time) {
							switch seg.Type {
							case "image":
								_builder.Text("[图片消息，已经超过3分钟过期时间]")
							case "file":
								_builder.Text("[文件消息，已经超过3分钟过期时间]")
							}
						} else {
							_builder.Raw(m.Message[i])
						}
					} else {
						key_20 := ""
						key_10 := ""
						for _, k := range ncrkey {
							switch k.Type {
							case 20:
								key_20 = strings.TrimPrefix(k.Rkey, "&rkey=")
							case 10:
								key_10 = strings.TrimPrefix(k.Rkey, "&rkey=")
							}
						}
						if key_20 == "" || key_10 == "" {
							switch seg.Type {
							case "image":
								log.Println("无法解析图片URL")
							case "file":
								log.Println("无法解析文件URL")
							}
							return true, nil
						}
						link, _ := m.Message[i].Data["url"].(string)
						if link != "" {
							if modifyer, err := utils.NewURLModifier(link); err != nil {
								log.Println("无法解析图片URL")
								return true, nil
							} else {
								newLink := modifyer.SetQuery("rkey", key_20).String()
								switch seg.Type {
								case "image":
									_builder.ImageUrl(newLink)
								case "file":
									if fileName, ok := m.Message[i].Data["file"].(string); ok {
										_builder.FileUrl(fileName, newLink)
									}
								}
							}
						}
					}
				case "video":
					if isTimeout(m.Time) {
						_builder.Text("[视频消息，已经超过3分钟过期时间]")
					} else {
						_builder.Raw(m.Message[i])
					}
				case "record":
					_builder.Text("[语音消息]")
				}
			}
			fbuilder.Message(m.Sender.UserId, m.Sender.Nickname, _builder.Build())
		}
		_, success := bot.SendGroupForwardMsg(msg.GroupId, fbuilder.Build())
		if !success {
			builder := msgchain.Builder().Group()
			builder.Text("无法获取消息列表")
			bot.SendGroupMsg(msg.GroupId, builder.Build())
			log.Println("[群聊防撤回插件]: 无法转发消息")
		}
		return false, nil
	}
	queue.Add(&msg)
	return true, nil
}

func isTimeout(timestamp uint) bool {
	now := uint(time.Now().Unix())
	return now-timestamp > ResourceTimeout
}

func (p *AntiWithdrawalPlugin) Start(ctx context.Context, cfg *viper.Viper) error {
	p.adminId = cfg.GetUint("bot.admin_id")
	return nil
}

func (p *AntiWithdrawalPlugin) OnFriendMsg(ctx context.Context, bot bot.Bot, cmd command.Command, msg message.Message) (bool, error) {
	if cmd.Name == "explore" && msg.Sender.UserId == p.adminId {
		if len(cmd.Args) == 0 {
			builder := msgchain.Builder().Friend()
			builder.Text("请输入完整参数 /explore [Group ID] [count]")
			bot.SendFriendMsg(msg.Sender.UserId, builder.Build())
			return false, nil
		} else if len(cmd.Args) >= 1 {
			n := 50
			Gid, err := strconv.Atoi(cmd.Args[0])
			if err != nil {
				builder := msgchain.Builder().Friend()
				builder.Text("请输入正确参数:Group ID, 语法: /explore [Group ID] [count](option)")
				bot.SendFriendMsg(msg.Sender.UserId, builder.Build())
				return false, nil
			}
			if len(cmd.Args) == 2 {
				num, err := strconv.Atoi(cmd.Args[1])
				if err == nil && num > 0 && num <= 100 {
					n = num
				}
			}
			queueI, ok := p.msg.Load(uint(Gid))
			if !ok {
				builder := msgchain.Builder().Friend()
				builder.Text("请输入正确参数:Group ID Error, 语法: /explore [Group ID] [count](option)")
				bot.SendFriendMsg(msg.Sender.UserId, builder.Build())
				return false, nil
			}
			queue := queueI.(*MessageQueue[*message.Message])
			cachemsg := queue.Get(n)
			if len(cachemsg) == 0 {
				builder := msgchain.Builder().Friend()
				builder.Text("暂时没有保存到什么消息哦，请稍后再试")
				builder.Face(14)
				bot.SendFriendMsg(msg.Sender.UserId, builder.Build())
				return false, nil
			}
			fbuilder := msgchain.Builder().FriendForward()
			ncrkey, existRkey := bot.GetNCrkey()
			for _, m := range cachemsg {
				_builder := msgchain.Builder().Friend()
				for i, seg := range m.Message {
					switch seg.Type {
					case "text", "face", "at", "reply", "json", "music":
						_builder.Raw(m.Message[i])
					case "forward":
						_builder.Text("[转发消息，暂不支持查看]")
					case "image", "file":
						if !existRkey {
							if isTimeout(m.Time) {
								switch seg.Type {
								case "image":
									_builder.Text("[图片消息，已经超过3分钟过期时间]")
								case "file":
									_builder.Text("[文件消息，已经超过3分钟过期时间]")
								}
							} else {
								_builder.Raw(m.Message[i])
							}
						} else {
							key_20 := ""
							key_10 := ""
							for _, k := range ncrkey {
								switch k.Type {
								case 20:
									key_20 = strings.TrimPrefix(k.Rkey, "&rkey=")
								case 10:
									key_10 = strings.TrimPrefix(k.Rkey, "&rkey=")
								}
							}
							if key_20 == "" || key_10 == "" {
								switch seg.Type {
								case "image":
									log.Println("无法解析图片URL")
								case "file":
									log.Println("无法解析文件URL")
								}
								return true, nil
							}
							link, _ := m.Message[i].Data["url"].(string)
							if link != "" {
								if modifyer, err := utils.NewURLModifier(link); err != nil {
									log.Println("无法解析图片URL")
									return true, nil
								} else {
									newLink := modifyer.SetQuery("rkey", key_20).String()
									switch seg.Type {
									case "image":
										_builder.ImageUrl(newLink)
									case "file":
										if fileName, ok := m.Message[i].Data["file"].(string); ok {
											_builder.FileUrl(fileName, newLink)
										}
									}
								}
							}
						}
					case "video":
						if isTimeout(m.Time) {
							_builder.Text("[视频消息，已经超过3分钟过期时间]")
						} else {
							_builder.Raw(m.Message[i])
						}
					case "record":
						_builder.Text("[语音消息]")
					}
				}
				fbuilder.Message(m.Sender.UserId, m.Sender.Nickname, _builder.Build())
			}
			_, success := bot.SendFriendForwardMsg(msg.Sender.UserId, fbuilder.Build())
			if !success {
				builder := msgchain.Builder().Friend()
				builder.Text("无法获取消息列表")
				bot.SendFriendMsg(msg.Sender.UserId, builder.Build())
				log.Println("[群聊防撤回插件]: 无法转发消息")
			}
			return false, nil
		}
	}
	return true, nil
}
