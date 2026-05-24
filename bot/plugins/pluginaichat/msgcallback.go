package pluginaichat

import (
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
				return "发送成功", nil
			}
			return "", fmt.Errorf("发送失败")
		},
		SendImage: func(bs64content string) (string, error) {
			builder := msgchain.Builder().Group()
			builder.ImageBase64(bs64content)
			_, success := bot.SendGroupMsg(groupId, builder.Build())
			if success {
				logger.Info("发送图片", "group", groupId, "user", userId)
				return "发送成功", nil
			}
			return "", fmt.Errorf("发送失败")
		},
		SendFile: func(name, bs64content string) (string, error) {
			builder := msgchain.Builder().Group()
			builder.FileBase64(name, bs64content)
			_, success := bot.SendGroupMsg(groupId, builder.Build())
			if success {
				logger.Info("发送文件", "group", groupId, "user", userId, "file", name)
				return "发送成功", nil
			}
			return "", fmt.Errorf("发送失败")
		},
		GetMsgHistory: func(count int, message_seq int) (string, error) {
			msgs, ok := bot.GetGroupMsgHistory(groupId, count, message_seq)
			if !ok || msgs == nil {
				return "", fmt.Errorf("获取群聊历史消息失败")
			}
			var sb strings.Builder
			for _, msg := range *msgs {
				sb.WriteString(fmt.Sprintf("[message_seq:%d]\n", msg.MessageSeq))
				sb.WriteString(msg.FriendlyText(true,
					message.WithGetMsgFunc(bot.GetMsgDetail),
					message.WithGetForwardMsgFunc(bot.GetForwardMsg),
				))
				sb.WriteString("\n")
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
				return "发送成功", nil
			}
			return "", fmt.Errorf("发送失败")
		},
		SendImage: func(bs64content string) (string, error) {
			builder := msgchain.Builder().Friend()
			builder.ImageBase64(bs64content)
			_, success := bot.SendFriendMsg(userId, builder.Build())
			if success {
				logger.Info("发送图片", "user", userId)
				return "发送成功", nil
			}
			return "", fmt.Errorf("发送失败")
		},
		SendFile: func(name, bs64content string) (string, error) {
			builder := msgchain.Builder().Friend()
			builder.FileBase64(name, bs64content)
			_, success := bot.SendFriendMsg(userId, builder.Build())
			if success {
				logger.Info("发送文件", "user", userId, "file", name)
				return "发送成功", nil
			}
			return "", fmt.Errorf("发送失败")

		},
		GetMsgHistory: func(count int, message_seq int) (string, error) {
			msgs, ok := bot.GetFriendMsgHistory(userId, count, message_seq)
			if !ok || msgs == nil {
				return "", fmt.Errorf("获取好友历史消息失败")
			}
			var sb strings.Builder
			for _, msg := range *msgs {
				sb.WriteString(fmt.Sprintf("[message_seq:%d]\n", msg.MessageSeq))
				sb.WriteString(msg.FriendlyText(true,
					message.WithGetMsgFunc(bot.GetMsgDetail),
					message.WithGetForwardMsgFunc(bot.GetForwardMsg),
				))
				sb.WriteString("\n")
			}
			return sb.String(), nil
		},
	}
	return msgFuncs
}
