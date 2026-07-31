package pluginaichat

import (
	"context"
	"fmt"
	"strings"

	"github.com/jeanhua/AniaBot/bot/component/llmtool"
)

// knowledgeToolBase 为知识库工具共享管理器引用与当前会话 scope。
// scope 在会话创建时绑定（global / g:群号 / f:QQ号），AI 无法指定其它 scope，
// 从机制上保证知识库不会跨会话泄露（全局库所有会话可见，属预期行为）。
type knowledgeToolBase struct {
	mgr   *knowledgeManager
	scope string
}

// newKnowledgeTools 创建一组知识库工具，注册到当前会话的执行器中。
// sessionDesc 为当前会话的可读描述，写入工具描述让 AI 明确知识库的归属范围。
func newKnowledgeTools(mgr *knowledgeManager, scope string, sessionDesc string) []llmtool.Tool {
	base := knowledgeToolBase{mgr: mgr, scope: scope}
	return []llmtool.Tool{
		&kbSearchTool{
			BaseTool:          llmtool.MakeBaseTool("kb_search", "检索知识库（"+sessionDesc+"的会话库及全局库）。当用户询问知识库中的资料、或你回答不了需要查资料时调用。query 填写关键词（可多个，空格分隔）；返回相关文档片段及其来源标题", kbSearchParams{}),
			knowledgeToolBase: base,
		},
		&kbAddTool{
			BaseTool:          llmtool.MakeBaseTool("kb_add", "向知识库（"+sessionDesc+"的会话库）新增一条文档。当用户明确要求记录文档/资料/教程，或给了你有保存价值的完整知识内容时调用。相同标题与内容会自动去重", kbAddParams{}),
			knowledgeToolBase: base,
		},
	}
}

// ---- kb_search ----

type kbSearchParams struct {
	Query string `json:"query" desc:"检索关键词（可多个，空格分隔），如「部署 docker 配置」"`
	TopK  *int   `json:"top_k,omitempty" desc:"返回片段条数，默认5，最大10"`
}

type kbSearchTool struct {
	llmtool.BaseTool[kbSearchParams]
	knowledgeToolBase
}

func (t *kbSearchTool) Execute(_ context.Context, params any, _ llmtool.CallBackFuncs) (string, error) {
	p := params.(*kbSearchParams)
	topK := 5
	if p.TopK != nil {
		topK = *p.TopK
		if topK > 10 {
			topK = 10
		}
	}
	query := strings.TrimSpace(p.Query)
	if query == "" {
		return "", fmt.Errorf("query 不能为空")
	}
	results := t.mgr.search(t.scope, query, topK)
	if len(results) == 0 {
		return "知识库中没有找到与「" + query + "」相关的资料", nil
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("知识库相关片段（%d 条）：\n", len(results)))
	for i, r := range results {
		if i > 0 {
			sb.WriteString("\n")
		}
		if r.Title != "" {
			sb.WriteString("《" + r.Title + "》")
			if r.Scope != kbScopeGlobal {
				sb.WriteString("（" + r.Scope + "）")
			}
			sb.WriteString("：")
		}
		sb.WriteString(r.Chunk)
	}
	return sb.String(), nil
}

// ---- kb_add ----

type kbAddParams struct {
	Title   string   `json:"title" desc:"文档标题，简短概括内容，如「Docker 部署指南」"`
	Content string   `json:"content" desc:"文档正文，一条完整自洽的资料/教程；最长 8000 字符，超出会被截断"`
	Tags    []string `json:"tags,omitempty" desc:"分类标签，便于检索，如 [\"部署\",\"docker\"]"`
	Source  string   `json:"source,omitempty" desc:"内容来源，如网页链接"`
}

type kbAddTool struct {
	llmtool.BaseTool[kbAddParams]
	knowledgeToolBase
}

func (t *kbAddTool) Execute(_ context.Context, params any, _ llmtool.CallBackFuncs) (string, error) {
	p := params.(*kbAddParams)
	doc, err := t.mgr.add(t.scope, p.Title, p.Content, p.Tags, p.Source)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("已加入知识库（ID=%s）：%s", doc.ID, doc.Title), nil
}
