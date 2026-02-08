package functool

import (
	"encoding/json"
	"log"
	"math/rand/v2"

	"github.com/go-resty/resty/v2"
	"github.com/jeanhua/AniaBot/ania/utils"
	"github.com/tmc/langchaingo/llms"
)

type memeParam struct {
	Text string `json:"text" desc:"表情包的文本描述"`
}

const (
	MEME_TOOL_NAME = "meme"
)

func MakeMemeTool() []llms.Tool {
	return []llms.Tool{
		utils.StructToOpenAITool("meme", "用于向用户发送表情包", memeParam{}),
	}
}

func TryHandleMemeFunc(call llms.ToolCall, msgFuncs OptionFuncs) (string, error) {
	log.Println("执行meme... 参数:", call.FunctionCall.Arguments)
	var param = memeParam{}
	if err := json.Unmarshal([]byte(call.FunctionCall.Arguments), &param); err != nil {
		return "发送失败", err
	}
	modifier, _ := utils.NewURLModifier("https://api.suol.cc/v1/meme.php")
	modifier.SetQuery("msg", param.Text)
	modifier.SetQuery("num", "100")
	type responseTy struct {
		Data []struct {
			ImageUrl string `json:"img_url"`
		} `json:"data"`
	}
	result := responseTy{}
	client := resty.New()
	_, err := client.R().SetResult(&result).Get(modifier.String())
	if err != nil || len(result.Data) == 0 {
		return "", err
	}
	id := rand.IntN(len(result.Data))
	ok := msgFuncs.SendImage(result.Data[id].ImageUrl)
	if ok {
		return "发送成功", nil
	} else {
		return "发送失败", nil
	}
}
