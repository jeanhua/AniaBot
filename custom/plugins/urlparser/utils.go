package urlparser

import (
	"context"
	"regexp"

	"github.com/go-resty/resty/v2"
	"github.com/jeanhua/AniaBot/bot/utils"
	"github.com/jeanhua/AniaBot/common/model/message"
)

func fetchPageContent(ctx context.Context, token string, url string) (string, error) {
	link := "https://r.jina.ai/" + url
	client := resty.New()
	resp, err := client.R().
		SetContext(ctx).
		SetHeader("Authorization", "Bearer "+token).
		SetHeader("X-Base", "final").
		SetHeader("X-Locale", "zh-CN").
		SetHeader("X-Referer", "https://www.google.com/").
		SetHeader("X-User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36").
		SetHeader("X-Retain-Images", "none").
		SetHeader("X-Return-Format", "markdown").
		SetHeader("X-With-Links-Summary", "true").
		SetHeader("X-Engine", "cf-browser-rendering").
		Get(link)

	if err != nil {
		return "", err
	}
	text := resp.String()
	rText := []rune(text)
	if len(rText) > 8000 {
		return string(rText[:8000]) + "...", nil
	} else {
		return text, nil
	}
}

var urlRegex *regexp.Regexp = regexp.MustCompile(`(https?|ftp|file)://[-A-Za-z0-9+&@#/%?=~_|!:,.;]+[-A-Za-z0-9+&@#/%=~_|]|[-A-Za-z0-9]+(\.[-A-Za-z0-9]+)+\.[A-Za-z]{2,}(/[-A-Za-z0-9+&@#/%?=~_|!:,.;]*)?`)

func getUrl(text string) string {
	url := urlRegex.FindString(text)
	return url
}

func getMsgText(msg message.Message) string {
	text, _ := utils.ExtraMessageStr(msg)
	return text
}
