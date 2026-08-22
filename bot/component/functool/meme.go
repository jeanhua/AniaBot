package functool

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"math/rand/v2"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/jeanhua/AniaBot/bot/component/llmtool"
	"github.com/tidwall/gjson"
)

// memeHTTPTimeout 表情包接口与图片下载的请求超时
const memeHTTPTimeout = 30 * time.Second

// meme 工具默认配置：GIPHY stickers 搜索（全球最大 GIF/表情包平台，
// 免费 API Key 在 https://developers.giphy.com 申请）。
// 默认取 fixed_width 尺寸，兼顾清晰度与下载体积（original 动辄数 MB）
const (
	DefaultMemeURL      = "https://api.giphy.com/v1/stickers/search?api_key=${key}&q=${msg}&limit=${num}"
	DefaultMemeListPath = "data"
	DefaultMemeImgField = "images.fixed_width.url"
	DefaultMemeNum      = 50
)

// MemeConfig meme 工具配置：请求地址模板 + gjson 解析路径。
// 任何「返回一组图片 URL」的接口都能接入，接口再挂时改配置即可切换，
// 无需改代码。URL 支持 ${msg}（搜索词，URL 编码）、${num}（请求数量）、
// ${key}（API Key，URL 编码）占位符
type MemeConfig struct {
	URL      string // 请求地址模板，含 ${msg} ${num} ${key} 占位符
	Key      string // 接口 API Key，替换 ${key}
	ListPath string // gjson 路径，指向响应中的图片数组，如 data
	ImgField string // 数组元素中图片 URL 的 gjson 路径，如 img_url 或 images.fixed_width.url
	Num      int    // 每次请求的结果数量（随机从中挑一张发送）
}

// fillDefaults 为留空的字段补齐内置默认值
func (c *MemeConfig) fillDefaults() {
	if c.URL == "" {
		c.URL = DefaultMemeURL
	}
	if c.ListPath == "" {
		c.ListPath = DefaultMemeListPath
	}
	if c.ImgField == "" {
		c.ImgField = DefaultMemeImgField
	}
	if c.Num <= 0 {
		c.Num = DefaultMemeNum
	}
}

// buildURL 渲染请求地址模板
func (c *MemeConfig) buildURL(text string) string {
	u := strings.ReplaceAll(c.URL, "${msg}", url.QueryEscape(text))
	u = strings.ReplaceAll(u, "${num}", strconv.Itoa(c.Num))
	u = strings.ReplaceAll(u, "${key}", url.QueryEscape(c.Key))
	return u
}

type MemeParams struct {
	Text string `json:"text" desc:"表情包的文本描述,比如开心、生气、为什么、你真是的...等等短句"`
}

type MemeTool struct {
	llmtool.BaseTool[MemeParams]
	config MemeConfig
}

func NewMemeTool(config MemeConfig) *MemeTool {
	config.fillDefaults()
	return &MemeTool{
		BaseTool: llmtool.MakeBaseTool("meme", "用于向用户发送表情包", MemeParams{}),
		config:   config,
	}
}

// extractImageURLs 从接口响应中提取全部图片 URL
func (t *MemeTool) extractImageURLs(body []byte) []string {
	arr := gjson.GetBytes(body, t.config.ListPath).Array()
	urls := make([]string, 0, len(arr))
	for _, item := range arr {
		if u := item.Get(t.config.ImgField).String(); u != "" {
			urls = append(urls, u)
		}
	}
	return urls
}

func (t *MemeTool) Execute(ctx context.Context, params any, callbacks llmtool.CallBackFuncs) (string, error) {
	p := params.(*MemeParams)
	log.Println("执行meme... 参数:", p)

	cfg := t.config
	// 模板需要 API Key 但未配置时给出明确指引，而不是让上游返回 401 裸错误
	if strings.Contains(cfg.URL, "${key}") && cfg.Key == "" {
		return "", fmt.Errorf("meme 工具未配置 API Key：请在面板「AI 对话 · 表情包」填写表情包 API Key（默认接口为 GIPHY，可在 developers.giphy.com 免费申请），或改用其他免 Key 接口地址")
	}

	// 传入 ctx 并设置超时：否则接口挂起时请求永久阻塞，
	// 泄漏 goroutine 并占死一个全局速率限制槽位，/stop 也无法中断
	client := resty.New().SetTimeout(memeHTTPTimeout)
	resp, err := client.R().SetContext(ctx).Get(cfg.buildURL(p.Text))
	if err != nil {
		return "", err
	}
	if resp.StatusCode() != 200 {
		return "", fmt.Errorf("表情包接口请求失败: HTTP %d", resp.StatusCode())
	}

	imageURLs := t.extractImageURLs(resp.Body())
	if len(imageURLs) == 0 {
		// 接口返回空数据时必须提前返回，否则 rand.IntN(0) 会 panic；
		// 同时给出明确错误，避免 orchestrator 把空字符串当作成功结果反馈给 LLM
		return "", fmt.Errorf("未找到相关表情包")
	}

	imageURL := imageURLs[rand.IntN(len(imageURLs))]

	// 下载图片
	downloadClient := resty.New().SetTimeout(memeHTTPTimeout)
	resp2, err := downloadClient.R().SetContext(ctx).Get(imageURL)
	if err != nil {
		return fmt.Sprintf("下载表情包失败: %v", err), err
	}
	if resp2.StatusCode() != 200 {
		return fmt.Sprintf("下载表情包失败: HTTP %d", resp2.StatusCode()), fmt.Errorf("HTTP %d", resp2.StatusCode())
	}

	// 转为base64
	base64Data := base64.StdEncoding.EncodeToString(resp2.Body())

	_, err = callbacks.SendImage(base64Data)
	if err != nil {
		return fmt.Sprintf("表情包发送失败: %v", err), err
	}
	return fmt.Sprintf("已成功发送关于'%s'的表情包图片给用户", p.Text), nil
}
