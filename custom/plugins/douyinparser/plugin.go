package douyinparser

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/go-resty/resty/v2"
	"github.com/jeanhua/AniaBot/ania/utils"
	"github.com/jeanhua/AniaBot/common/bot"
	"github.com/jeanhua/AniaBot/common/model/command"
	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/jeanhua/AniaBot/common/msgchain"
	"github.com/jeanhua/AniaBot/common/plugin"
	"github.com/spf13/viper"
)

type DouyinParser struct {
	plugin.Meta
	re *regexp.Regexp
}

func Newplugin() *DouyinParser {
	return &DouyinParser{
		Meta: plugin.Meta{
			Name:      "抖音视频解析插件",
			HelpWords: "发送 /douyin [分享内容] 给我解析视频哦",
		},
	}
}

func (p *DouyinParser) Start(cfg *viper.Viper) {
	p.re = regexp.MustCompile(`https://v\.douyin\.com/[a-zA-Z0-9\-_]+(?:/|\b)`)
}

func (p *DouyinParser) OnGroupMsg(bot bot.Bot, cmd *command.Command, msg message.Message) bool {
	if cmd != nil && cmd.Mention && cmd.Name == "douyin" {
		text, _ := utils.ExtraMessageStr(msg)
		link, err := p.extractDouyinLink(text)
		if err != nil {
			builder := msgchain.Builder.Group()
			builder.Mention(msg.Sender.UserId)
			builder.Text(" 无法从分享的内容提取出抖音链接，请重新检查试试哦")
			bot.SendGroupMsg(msg.GroupId, builder.Build())
			return false
		}

		result, err := getResourse(link)
		if err != nil {
			builder := msgchain.Builder.Group()
			builder.Mention(msg.Sender.UserId)
			builder.Text(" 无法解析，请稍后再试")
			bot.SendGroupMsg(msg.GroupId, builder.Build())
			return false
		}

		builder := msgchain.Builder.Group()
		builder.Mention(msg.Sender.UserId)
		builder.Text(fmt.Sprintf(" 解析成功\n博主: %s\n描述: %s\n视频直链: %s",
			result.Data.Nickname,
			result.Data.Desc,
			result.Data.VideoUrl,
		))
		bot.SendGroupMsg(msg.GroupId, builder.Build())
		return false

	}
	return true
}

func (p *DouyinParser) OnFriendMsg(bot bot.Bot, cmd *command.Command, msg message.Message) bool {
	if cmd != nil && cmd.Name == "douyin" {
		text, _ := utils.ExtraMessageStr(msg)
		link, err := p.extractDouyinLink(text)
		if err != nil {
			builder := msgchain.Builder.Friend()
			builder.Text(" 无法从分享的内容提取出抖音链接，请重新检查试试哦")
			bot.SendFriendMsg(msg.Sender.UserId, builder.Build())
			return false
		}

		result, err := getResourse(link)
		if err != nil {
			builder := msgchain.Builder.Friend()
			builder.Text(" 无法解析，请稍后再试")
			bot.SendFriendMsg(msg.Sender.UserId, builder.Build())
			return false
		}

		builder := msgchain.Builder.Friend()
		builder.Text(fmt.Sprintf("解析成功\n博主: %s\n描述: %s\n视频直链: %s",
			result.Data.Nickname,
			result.Data.Desc,
			result.Data.VideoUrl,
		))
		bot.SendFriendMsg(msg.Sender.UserId, builder.Build())
		return false
	}
	return true
}

func (p *DouyinParser) extractDouyinLink(text string) (string, error) {
	match := p.re.FindString(text)
	if match == "" {
		return "", fmt.Errorf("未找到抖音链接")
	}
	match = strings.TrimSpace(match)
	match = strings.TrimRight(match, "!。，,；;")
	return match, nil
}

func getResourse(link string) (*responseTy, error) {
	client := resty.New()
	result := responseTy{}
	modifier, _ := utils.NewURLModifier("https://api.mmp.cc/api/Jiexi")
	modifier.SetQuery("url", link)
	_, err := client.R().SetResult(&result).Get(modifier.String())
	if err != nil {
		return nil, err
	}
	return &result, nil
}

type responseTy struct {
	Data struct {
		Type     string `json:"type"`
		VideoUrl string `json:"video_url"`
		Nickname string `json:"nickname"`
		Desc     string `json:"desc"`
	} `json:"data"`
}
