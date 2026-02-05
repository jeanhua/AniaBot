package component

import (
	"github.com/jeanhua/AniaBot/ania/utils"
	"github.com/tmc/langchaingo/llms"
)

type Jina struct {
}

type webSearchParam struct {
}

type webExploreParam struct {
}

func MakeJinaTool() []llms.Tool {
	return []llms.Tool{
		utils.StructToOpenAITool("webSearch", "用于互联网搜索信息", webSearchParam{}),
		utils.StructToOpenAITool("webExplore", "用于浏览网页信息", webExploreParam{}),
	}
}

func (j *Jina) Search(query string) (string, error) {
	return "", nil
}

func (j *Jina) Explore(link string) (string, error) {
	return "", nil
}
