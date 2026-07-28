package pluginaichat

import (
	"context"
	"fmt"
	"strings"

	"github.com/jeanhua/AniaBot/bot/component/llmtool"
)

// memoryToolBase 为记忆工具共享管理器引用与当前会话 scope。
// scope 在会话创建时绑定（g:群号 / f:QQ号），AI 无法指定其它 scope，
// 从机制上保证群与群、群与私聊之间的记忆隔离。
type memoryToolBase struct {
	mgr   *memoryManager
	scope string
}

// newMemoryTools 创建一组长期记忆管理工具，注册到当前会话的执行器中。
// sessionDesc 为当前会话的可读描述（如 "群聊（群号 123）"），写入工具描述
// 让 AI 明确记忆的归属范围。
func newMemoryTools(mgr *memoryManager, scope string, sessionDesc string) []llmtool.Tool {
	base := memoryToolBase{mgr: mgr, scope: scope}
	return []llmtool.Tool{
		&memorySaveTool{
			BaseTool:       llmtool.MakeBaseTool("memory_save", "保存一条长期记忆，跨会话、跨重启保留。当用户透露称呼/偏好/重要信息，或群里形成约定、发生值得记住的事件时主动调用。内容应是一条完整自洽的事实（含必要的背景，因为未来检索时不一定能看到当前对话）。记忆仅属于当前会话（"+sessionDesc+"），不会同步到其它群聊或私聊。相同内容会自动去重", memorySaveParams{}),
			memoryToolBase: base,
		},
		&memorySearchTool{
			BaseTool:       llmtool.MakeBaseTool("memory_search", "检索当前会话（"+sessionDesc+"）的长期记忆。当对话涉及过去的事情、用户的喜好或你不确定的背景时先调用。query 填写关键词（可多个，空格分隔），只返回相关记忆；不传则按时间倒序列出全部记忆。query 没命中时换个说法重试，或直接不传 query 看全量", memorySearchParams{}),
			memoryToolBase: base,
		},
		&memoryForgetTool{
			BaseTool:       llmtool.MakeBaseTool("memory_forget", "按 ID 删除一条长期记忆。当记忆过时、有误，或用户明确要求忘记时使用；需要修改记忆时先删除旧的再 memory_save 新的", memoryForgetParams{}),
			memoryToolBase: base,
		},
	}
}

// ---- memory_save ----

type memorySaveParams struct {
	Content string   `json:"content" desc:"要记住的内容，一条完整自洽的事实，如「群主小明（QQ 12345）讨厌被半夜@」；最长 2000 字符，超出会被截断"`
	UserID  string   `json:"user_id,omitempty" desc:"该记忆关联的群成员QQ号（其消息以 [nickname:昵称 id:QQ号] 开头，取其中的id）；属于整个群/会话的记忆不填"`
	Tags    []string `json:"tags,omitempty" desc:"分类标签，便于检索，如 [\"偏好\",\"称呼\"]"`
}

type memorySaveTool struct {
	llmtool.BaseTool[memorySaveParams]
	memoryToolBase
}

func (t *memorySaveTool) Execute(_ context.Context, params any, _ llmtool.CallBackFuncs) (string, error) {
	p := params.(*memorySaveParams)
	entry, err := t.mgr.add(t.scope, p.UserID, p.Content, p.Tags)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("已记住（ID=%s）：%s", entry.ID, entry.Content), nil
}

// ---- memory_search ----

type memorySearchParams struct {
	Query string `json:"query,omitempty" desc:"检索关键词（可多个，空格分隔），用于相关性排序；不填则返回全部记忆"`
}

type memorySearchTool struct {
	llmtool.BaseTool[memorySearchParams]
	memoryToolBase
}

func (t *memorySearchTool) Execute(_ context.Context, params any, _ llmtool.CallBackFuncs) (string, error) {
	p := params.(*memorySearchParams)
	entries := t.mgr.list(t.scope)
	if len(entries) == 0 {
		return "当前还没有任何长期记忆", nil
	}

	total := len(entries)
	if q := strings.TrimSpace(p.Query); q != "" {
		// 关键词打分：命中 tag 权重高于正文；零分条目直接过滤，
		// 避免无关记忆占用上下文、干扰模型判断
		entries = filterMemoryByRelevance(entries, strings.Fields(q))
		if len(entries) == 0 {
			return fmt.Sprintf("没有找到与「%s」相关的记忆（当前共 %d 条记忆，不传 query 可查看全部）", q, total), nil
		}
	} else {
		// 无 query 时按时间倒序（新记忆在前，时效性更强）
		for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
			entries[i], entries[j] = entries[j], entries[i]
		}
	}

	var sb strings.Builder
	if total != len(entries) {
		sb.WriteString(fmt.Sprintf("相关记忆（%d/%d 条）：\n", len(entries), total))
	} else {
		sb.WriteString(fmt.Sprintf("长期记忆（共 %d 条）：\n", total))
	}
	for _, e := range entries {
		sb.WriteString(formatMemoryLine(e))
		sb.WriteString("\n")
	}
	return sb.String(), nil
}

// scoreMemory 计算单条记忆与关键词集合的相关度：正文命中 +10，tag 命中 +20。
func scoreMemory(e memoryEntry, terms []string) int {
	score := 0
	content := strings.ToLower(e.Content)
	for _, term := range terms {
		term = strings.ToLower(term)
		if strings.Contains(content, term) {
			score += 10
		}
		for _, tag := range e.Tags {
			if strings.Contains(strings.ToLower(tag), term) {
				score += 20
			}
		}
	}
	return score
}

// filterMemoryByRelevance 过滤掉零分记忆，其余按相关度降序返回（稳定排序）。
func filterMemoryByRelevance(entries []memoryEntry, terms []string) []memoryEntry {
	type scored struct {
		e     memoryEntry
		score int
	}
	matched := make([]scored, 0, len(entries))
	for _, e := range entries {
		if s := scoreMemory(e, terms); s > 0 {
			matched = append(matched, scored{e, s})
		}
	}
	for i := 1; i < len(matched); i++ {
		for j := i; j > 0 && matched[j].score > matched[j-1].score; j-- {
			matched[j], matched[j-1] = matched[j-1], matched[j]
		}
	}
	out := make([]memoryEntry, len(matched))
	for i, m := range matched {
		out[i] = m.e
	}
	return out
}

// formatMemoryLine 格式化单条记忆：[ID] (关联QQ) [tags] 内容 (日期)
func formatMemoryLine(e memoryEntry) string {
	var sb strings.Builder
	sb.WriteString("[" + e.ID + "] ")
	if e.UserID != "" {
		sb.WriteString("(QQ " + e.UserID + ") ")
	}
	if len(e.Tags) > 0 {
		sb.WriteString("<" + strings.Join(e.Tags, ",") + "> ")
	}
	sb.WriteString(e.Content)
	sb.WriteString("（记于 " + e.CreatedAt.Local().Format("2006-01-02") + "）")
	return sb.String()
}

// ---- memory_forget ----

type memoryForgetParams struct {
	ID string `json:"id" desc:"要删除的记忆ID（memory_search 结果中方括号内的8位ID）"`
}

type memoryForgetTool struct {
	llmtool.BaseTool[memoryForgetParams]
	memoryToolBase
}

func (t *memoryForgetTool) Execute(_ context.Context, params any, _ llmtool.CallBackFuncs) (string, error) {
	p := params.(*memoryForgetParams)
	if strings.TrimSpace(p.ID) == "" {
		return "", fmt.Errorf("id 不能为空")
	}
	if t.mgr.remove(t.scope, p.ID) {
		return "已删除记忆 " + p.ID, nil
	}
	return "", fmt.Errorf("记忆不存在: %s", p.ID)
}
