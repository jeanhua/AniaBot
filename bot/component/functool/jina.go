package functool

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/jeanhua/AniaBot/bot/component/llmtool"
	"github.com/jeanhua/AniaBot/bot/utils"
)

type WebSearchParams struct {
	Query  string `json:"query" desc:"需要搜索的内容"`
	Page   *int   `json:"page,omitempty" desc:"可选，用于翻页，从1开始"`
	Offset *int   `json:"offset,omitempty" desc:"可选，从该字符位置继续返回上次未读完的结果（上次结果被截断时，末尾会给出续读位置）"`
}

type WebExploreParams struct {
	Url    string `json:"url" desc:"需要浏览的网页链接"`
	Offset *int   `json:"offset,omitempty" desc:"可选，从该字符位置继续返回上次未读完的正文（上次结果被截断时，末尾会给出续读位置）"`
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
		BaseTool:    llmtool.MakeBaseTool("webExplore", "用于浏览网页信息，超长网页会被分段返回，可按提示用 offset 续读", WebExploreParams{}),
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

	client := newJinaClient()
	resp, err := client.R().
		SetContext(ctx).
		SetHeader("Authorization", "Bearer "+t.searchToken).
		SetHeader("X-Respond-With", "no-content").
		Get(modifier.String())

	if err != nil {
		return "", err
	}
	if resp.StatusCode() != http.StatusOK {
		// 401（token 失效）/429/5xx 的错误页是 HTML 而非搜索结果，
		// 直接返回会给模型一堆无用文本并诱导重试
		return "", fmt.Errorf("jina 搜索请求失败: HTTP %d", resp.StatusCode())
	}
	text := resp.String()
	return sliceJinaContent(text, params.Offset), nil
}

func (t *WebExploreTool) explore(ctx context.Context, params *WebExploreParams) (string, error) {
	link := "https://r.jina.ai/" + params.Url
	client := newJinaClient()
	resp, err := client.R().
		SetContext(ctx).
		SetHeader("Authorization", "Bearer "+t.searchToken).
		SetHeader("X-Referer", "https://www.google.com/").
		SetHeader("X-User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36").
		SetHeader("X-Retain-Images", "none").
		SetHeader("X-With-Links-Summary", "true").
		SetHeader("X-Engine", "cf-browser-rendering").
		Get(link)

	if err != nil {
		return "", err
	}
	if resp.StatusCode() != http.StatusOK {
		return "", fmt.Errorf("jina 网页抓取失败: HTTP %d", resp.StatusCode())
	}
	text := resp.String()
	return sliceJinaContent(text, params.Offset), nil
}

// jinaContentLimit 单次返回给模型的正文上限（字符数），控制上下文占用；
// 超出部分不丢弃，按 offset 分段续读。
const jinaContentLimit = 8000

// sliceJinaContent 按 offset 切片返回正文：无 offset 时返回开头一段，带 offset 时
// 从该位置续读；后面还有内容时在末尾附上续读提示，模型按提示再次调用即可读到剩余部分。
func sliceJinaContent(text string, offset *int) string {
	r := []rune(text)
	start := 0
	if offset != nil && *offset > 0 {
		if *offset >= len(r) {
			return fmt.Sprintf("offset=%d 超出内容长度（共 %d 字），没有更多内容", *offset, len(r))
		}
		start = *offset
	}
	if len(r)-start <= jinaContentLimit {
		return string(r[start:])
	}
	end := start + jinaContentLimit
	return string(r[start:end]) + fmt.Sprintf("\n...(内容未完，共 %d 字；用 offset=%d 再次调用可读取后续内容)", len(r), end)
}

// newJinaClient 创建带请求超时的 Jina 客户端。
// 若无超时，jina 服务挂起时整个会话会阻塞到消息预算耗尽（/stop 也无法中断工具调用）。
const jinaTimeout = 30 * time.Second

func newJinaClient() *resty.Client {
	return resty.New().SetTimeout(jinaTimeout)
}
