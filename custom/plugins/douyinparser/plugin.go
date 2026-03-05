package douyinparser

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/go-resty/resty/v2"
	"github.com/jeanhua/AniaBot/bot/utils"
	"github.com/jeanhua/AniaBot/common/aniaerror"
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

func (p *DouyinParser) Start(ctx context.Context, cfg *viper.Viper) error {
	p.re = regexp.MustCompile(`https://v\.douyin\.com/[a-zA-Z0-9\-_]+(?:/|\b)`)
	if p.re == nil {
		return aniaerror.ParameterInitializeError
	}
	return nil
}

func (p *DouyinParser) OnGroupMsg(ctx context.Context, bot bot.Bot, cmd command.Command, msg message.Message) (bool, error) {
	if cmd.Mention && cmd.Name == "douyin" {
		text, _ := utils.ExtraMessageStr(msg)
		link, err := p.extractDouyinLink(text)
		if err != nil {
			builder := msgchain.Builder().Group()
			builder.Mention(msg.Sender.UserId)
			builder.Text(" 无法从分享的内容提取出抖音链接，请重新检查试试哦")
			bot.SendGroupMsg(msg.GroupId, builder.Build())
			return false, nil
		}

		result, err := getResourse(p.RestyClient, link)
		if err != nil {
			builder := msgchain.Builder().Group()
			builder.Mention(msg.Sender.UserId)
			builder.Text(" 无法解析，请稍后再试")
			bot.SendGroupMsg(msg.GroupId, builder.Build())
			return false, nil
		}

		builder := msgchain.Builder().Group()
		builder.Mention(msg.Sender.UserId)
		builder.Text(" ").Face(24).Text(fmt.Sprintf("解析成功\n博主: %s\n标题: %s\n签名: %s\n视频直链: %s",
			result.Data.AdditionalData[0].Nickname,
			result.Data.AdditionalData[0].Desc,
			result.Data.AdditionalData[0].Signature,
			result.Data.VideoURL,
		))
		bot.SendGroupMsg(msg.GroupId, builder.Build())
		return false, nil

	}
	return true, nil
}

func (p *DouyinParser) OnFriendMsg(ctx context.Context, bot bot.Bot, cmd command.Command, msg message.Message) (bool, error) {
	if cmd.Name == "douyin" {
		text, _ := utils.ExtraMessageStr(msg)
		link, err := p.extractDouyinLink(text)
		if err != nil {
			builder := msgchain.Builder().Friend()
			builder.Text(" 无法从分享的内容提取出抖音链接，请重新检查试试哦")
			bot.SendFriendMsg(msg.Sender.UserId, builder.Build())
			return false, nil
		}

		result, err := getResourse(p.RestyClient, link)
		if err != nil {
			builder := msgchain.Builder().Friend()
			builder.Text(" 无法解析，请稍后再试")
			bot.SendFriendMsg(msg.Sender.UserId, builder.Build())
			return false, nil
		}

		builder := msgchain.Builder().Friend()
		builder.Face(24).Text(fmt.Sprintf("解析成功\n博主: %s\n标题: %s\n签名: %s\n视频直链: %s",
			result.Data.AdditionalData[0].Nickname,
			result.Data.AdditionalData[0].Desc,
			result.Data.AdditionalData[0].Signature,
			result.Data.VideoURL,
		))
		bot.SendFriendMsg(msg.Sender.UserId, builder.Build())
		return false, nil
	}
	return true, nil
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

func getResourse(client *resty.Client, link string) (*responseTy, error) {
	result := responseTy{}
	modifier, _ := utils.NewURLModifier("https://api.xinyew.cn/api/douyinjx")
	modifier.SetQuery("url", link)
	_, err := client.R().SetResult(&result).Get(modifier.String())
	if err != nil {
		return nil, err
	}
	if result.Code != 200 {
		return nil, fmt.Errorf("解析失败")
	}
	return &result, nil
}

type responseTy struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		PlayURL        string `json:"play_url"`
		VideoURL       string `json:"video_url"`
		ParseTime      string `json:"parse_time"`
		AdditionalData []struct {
			Desc      string `json:"desc"`
			URL       string `json:"url"`
			Nickname  string `json:"nickname"`
			Signature string `json:"signature"`
		} `json:"additional_data"`
	} `json:"data"`
}
