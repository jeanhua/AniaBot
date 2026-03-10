package functool

import (
	"context"
	"fmt"
	"log"

	"github.com/go-resty/resty/v2"
	"github.com/jeanhua/AniaBot/bot/component/llmtool"
	"github.com/jeanhua/AniaBot/bot/utils"
)

type WebSearchParams struct {
	Query string `json:"query" desc:"需要搜索的内容"`
	Page  *int   `json:"page,omitempty" desc:"可选，用于翻页，从1开始"`
}

type WebExploreParams struct {
	Url string `json:"url" desc:"需要浏览的网页链接"`
}

type WebSearchTool struct {
	llmtool.BaseTool[WebSearchParams]
	searchToken string
}

type WebExploreTool struct {
	llmtool.BaseTool[WebExploreParams]
	searchToken string
}

func NewWebSearchTool(searchToken string) *WebSearchTool {
	return &WebSearchTool{
		BaseTool:    llmtool.MakeBaseTool("webSearch", "用于互联网搜索信息", WebSearchParams{}),
		searchToken: searchToken,
	}
}

func NewWebExploreTool(searchToken string) *WebExploreTool {
	return &WebExploreTool{
		BaseTool:    llmtool.MakeBaseTool("webExplore", "用于浏览网页信息", WebExploreParams{}),
		searchToken: searchToken,
	}
}

func (t *WebSearchTool) Execute(ctx context.Context, params any, callbacks llmtool.CallBackFuncs) (string, error) {
	p := params.(*WebSearchParams)
	log.Println("执行webSearch... 参数: ", p)
	return t.search(ctx, p)
}

func (t *WebExploreTool) Execute(ctx context.Context, params any, callbacks llmtool.CallBackFuncs) (string, error) {
	p := params.(*WebExploreParams)
	log.Println("执行webExplore... 参数:", p)
	return t.explore(ctx, p)
}

func (t *WebSearchTool) search(ctx context.Context, params *WebSearchParams) (string, error) {
	modifier, err := utils.NewURLModifier("https://s.jina.ai/")
	if err != nil {
		return "", err
	}
	modifier.SetQuery("q", params.Query)
	modifier.SetQuery("gl", "CN")
	if params.Page != nil {
		modifier.SetQuery("page", fmt.Sprintf("%d", *params.Page))
	}

	client := resty.New()
	resp, err := client.R().
		SetContext(ctx).
		SetHeader("Authorization", "Bearer "+t.searchToken).
		SetHeader("X-Respond-With", "no-content").
		Get(modifier.String())

	if err != nil {
		return "", err
	}
	text := resp.String()
	rText := []rune(text)
	if len(rText) > 8000 {
		return string(rText[:8000]) + "...", nil
	}
	return text, nil
}

func (t *WebExploreTool) explore(ctx context.Context, params *WebExploreParams) (string, error) {
	link := "https://r.jina.ai/" + params.Url
	client := resty.New()
	resp, err := client.R().
		SetContext(ctx).
		SetHeader("Authorization", "Bearer "+t.searchToken).
		SetHeader("X-Base", "final").
		SetHeader("X-Locale", "zh-CN").
		SetHeader("X-Referer", "https://www.google.com/").
		SetHeader("X-User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36").
		SetHeader("X-Retain-Images", "none").
		SetHeader("X-Return-Format", "markdown").
		SetHeader("X-Engine", "cf-browser-rendering").
		Get(link)

	if err != nil {
		return "", err
	}
	text := resp.String()
	rText := []rune(text)
	if len(rText) > 8000 {
		return string(rText[:8000]) + "...", nil
	}
	return text, nil
}
