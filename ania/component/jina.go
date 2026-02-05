package component

import (
	"context"
	"encoding/json"
	"errors"
	"log"

	"github.com/go-resty/resty/v2"
	"github.com/jeanhua/AniaBot/ania/utils"
	"github.com/tmc/langchaingo/llms"
)

type webSearchParam struct {
	Query string `json:"query" desc:"需要搜索的内容"`
	Page  *int   `json:"page,omitempty" desc:"可选，用于翻页，从1开始"`
}

type webExploreParam struct {
	Url string `json:"url" desc:"需要浏览的网页链接"`
}

func MakeJinaTool() []llms.Tool {
	return []llms.Tool{
		utils.StructToOpenAITool("webSearch", "用于互联网搜索信息", webSearchParam{}),
		utils.StructToOpenAITool("webExplore", "用于浏览网页信息", webExploreParam{}),
	}
}

func TryHanleJina(ctx context.Context, token string, call llms.ToolCall) (string, error) {
	switch call.FunctionCall.Name {
	case "webSearch":
		log.Println("执行webSearch...")
		param := webSearchParam{}
		err := json.Unmarshal([]byte(call.FunctionCall.Arguments), &param)
		if err != nil {
			return "", err
		}
		callResult, err := search(ctx, token, param)
		log.Println("webSearch执行结果", callResult)
		return callResult, err
	case "webExplore":
		log.Println("执行webExplore...")
		param := webExploreParam{}
		err := json.Unmarshal([]byte(call.FunctionCall.Arguments), &param)
		if err != nil {
			return "", err
		}
		callResult, err := explore(ctx, token, param)
		log.Println("webExplore执行结果", callResult)
		return callResult, err
	}
	return "", errors.New("没有匹配的函数调用")
}

func search(ctx context.Context, token string, params webSearchParam) (string, error) {
	modifier, err := utils.NewURLModifier("https://s.jina.ai/")
	if err != nil {
		return "", err
	}
	modifier.SetQuery("q", params.Query)
	modifier.SetQuery("gl", "CN")

	client := resty.New()
	resp, err := client.R().
		SetContext(ctx).
		SetHeader("Authorization", "Bearer "+token).
		SetHeader("X-Respond-With", "no-content").
		Get(modifier.String())

	if err != nil {
		return "", err
	}
	text := resp.String()
	if len(text) > 5000 {
		return text[:5000], nil
	} else {
		return text, nil
	}
}

func explore(ctx context.Context, token string, params webExploreParam) (string, error) {
	link := "https://r.jina.ai/" + params.Url
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
		SetHeader("X-Engine", "browser").
		Get(link)

	if err != nil {
		return "", err
	}
	text := resp.String()
	if len(text) > 5000 {
		return text[:5000], nil
	} else {
		return text, nil
	}
}
