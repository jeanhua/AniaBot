package pluginaichat

import (
	"encoding/base64"
	"fmt"
	"log/slog"
	"strings"

	"github.com/jeanhua/AniaBot/bot/component/llmtool"
	"github.com/jeanhua/AniaBot/common/bot"
	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/jeanhua/AniaBot/common/msgchain"
)

func MakeGroupCallback(bot bot.Bot, groupId, userId message.QID, logger *slog.Logger) llmtool.CallBackFuncs {
	msgFuncs := llmtool.CallBackFuncs{
		SendText: func(s string) (string, error) {
			builder := msgchain.Builder().Group()
			builder.Mention(userId)
			builder.Text(" " + s)
			_, success := bot.SendGroupMsg(groupId, builder.Build())
			if success {
				logger.Info("发送文本", "group", groupId, "user", userId, "text", s)
			}
			return "发送成功", nil
		},
		SendImage: func(url string) (string, error) {
			builder := msgchain.Builder().Group()
			builder.ImageUrl(url)
			_, success := bot.SendGroupMsg(groupId, builder.Build())
			if success {
				logger.Info("发送图片", "group", groupId, "user", userId, "image", url)
			}
			return "发送成功", nil
		},
		SendFile: func(fileName, content string) (string, error) {
			builder := msgchain.Builder().Group()
			builder.FileBase64(fileName, base64.StdEncoding.EncodeToString([]byte(content)))
			_, success := bot.SendGroupMsg(groupId, builder.Build())
			if success {
				logger.Info("发送文件", "group", groupId, "user", userId, "file", fileName)
			}
			return "发送成功", nil
		},
		GetMsgHistory: func(count int) (string, error) {
			msgs, ok := bot.GetGroupMsgHistory(groupId, count)
			if !ok || msgs == nil {
				return "", fmt.Errorf("获取群聊历史消息失败")
			}
			var sb strings.Builder
			for _, msg := range *msgs {
				var str = strings.Builder{}
				nickname := msg.Sender.Card
				if nickname == "" {
					nickname = msg.Sender.Nickname
				}
				str.WriteString(fmt.Sprintf("[nickname:%s id:%d]:", nickname, msg.Sender.UserId))
				for _, seg := range msg.Message {
					str.WriteString(
						seg.FriendlyText(
							message.WithGetMsgFunc(bot.GetMsgDetail),
							message.WithGetGroupUserInfo(msg.GroupId, bot.GetGroupUserInfo),
							message.WithGetForwardMsgFunc(bot.GetForwardMsg),
						),
					)
				}
				sb.WriteString(str.String())
			}
			return sb.String(), nil
		},
	}

	return msgFuncs
}

func MakeFriendCallback(bot bot.Bot, userId message.QID, logger *slog.Logger) llmtool.CallBackFuncs {
	msgFuncs := llmtool.CallBackFuncs{
		SendText: func(s string) (string, error) {
			builder := msgchain.Builder().Friend()
			builder.Text(s)
			_, success := bot.SendFriendMsg(userId, builder.Build())
			if success {
				logger.Info("发送文本", "user", userId, "text", s)
			}
			return "发送成功", nil
		},
		SendImage: func(url string) (string, error) {
			builder := msgchain.Builder().Friend()
			builder.ImageUrl(url)
			_, success := bot.SendFriendMsg(userId, builder.Build())
			if success {
				logger.Info("发送图片", "user", userId, "image", url)
			}
			return "发送成功", nil
		},
		SendFile: func(fileName, content string) (string, error) {
			builder := msgchain.Builder().Friend()
			builder.FileBase64(fileName, base64.StdEncoding.EncodeToString([]byte(content)))
			_, success := bot.SendFriendMsg(userId, builder.Build())
			if success {
				logger.Info("发送文件", "user", userId, "file", fileName)
			}
			return "发送成功", nil
		},
		GetMsgHistory: func(count int) (string, error) {
			msgs, ok := bot.GetFriendMsgHistory(userId, count)
			if !ok || msgs == nil {
				return "", fmt.Errorf("获取好友历史消息失败")
			}
			var sb strings.Builder
			for _, msg := range *msgs {
				var str = strings.Builder{}
				nickname := msg.Sender.Card
				if nickname == "" {
					nickname = msg.Sender.Nickname
				}
				str.WriteString(fmt.Sprintf("[nickname:%s id:%d]:", nickname, msg.Sender.UserId))
				for _, seg := range msg.Message {
					str.WriteString(
						seg.FriendlyText(
							message.WithGetMsgFunc(bot.GetMsgDetail),
							message.WithGetForwardMsgFunc(bot.GetForwardMsg),
						),
					)
				}
				sb.WriteString(str.String())
			}
			return sb.String(), nil
		},
	}
	return msgFuncs
}
