package functool

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"math/rand/v2"

	"github.com/go-resty/resty/v2"
	"github.com/jeanhua/AniaBot/bot/component/llmtool"
	"github.com/jeanhua/AniaBot/bot/utils"
)

type MemeParams struct {
	Text string `json:"text" desc:"表情包的文本描述,比如开心、生气、为什么、你真是的...等等短句"`
}

type MemeTool struct {
	llmtool.BaseTool[MemeParams]
}

func NewMemeTool() *MemeTool {
	return &MemeTool{
		BaseTool: llmtool.MakeBaseTool("meme", "用于向用户发送表情包", MemeParams{}),
	}
}

func (t *MemeTool) Execute(ctx context.Context, params any, callbacks llmtool.CallBackFuncs) (string, error) {
	p := params.(*MemeParams)
	log.Println("执行meme... 参数:", p)

	modifier, _ := utils.NewURLModifier("https://api.suol.cc/v1/meme.php")
	modifier.SetQuery("msg", p.Text)
	modifier.SetQuery("num", "100")

	type responseTy struct {
		Data []struct {
			ImageUrl string `json:"img_url"`
		} `json:"data"`
	}
	result := responseTy{}
	client := resty.New()
	_, err := client.R().SetResult(&result).Get(modifier.String())
	if err != nil {
		return "", err
	}
	if len(result.Data) == 0 {
		// 接口返回空数据时必须提前返回，否则 rand.IntN(0) 会 panic；
		// 同时给出明确错误，避免 orchestrator 把空字符串当作成功结果反馈给 LLM
		return "", fmt.Errorf("未找到相关表情包")
	}

	id := rand.IntN(len(result.Data))
	imageUrl := result.Data[id].ImageUrl

	// 下载图片
	downloadClient := resty.New()
	resp2, err := downloadClient.R().Get(imageUrl)
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
