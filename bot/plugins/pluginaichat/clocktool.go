package pluginaichat

import (
	"context"
	"fmt"
	"strings"

	"github.com/jeanhua/AniaBot/bot/component/llmtool"
	"github.com/jeanhua/AniaBot/bot/component/tasklog"
	"github.com/jeanhua/AniaBot/common/model/message"
)

// clockToolBase 为所有定时任务工具共享调度器引用与当前会话默认触发对象。
type clockToolBase struct {
	mgr     *clockManager
	defType string // 当前会话默认触发对象类型
	defID   message.QID
}

// resolveTarget 解析工具参数中的触发对象：仅当类型与 ID 均合理时采用，否则回退到当前会话。
func (b clockToolBase) resolveTarget(givenType string, givenID uint64) (string, message.QID) {
	if givenID != 0 && (givenType == clockTargetGroup || givenType == clockTargetFriend) {
		return givenType, message.QID(givenID)
	}
	return b.defType, b.defID
}

// newClockTools 创建一组定时任务管理工具，注册到当前会话的执行器中。
// defType/defID 为当前会话的触发对象，作为工具参数缺省时的默认值。
func newClockTools(mgr *clockManager, defType string, defID message.QID) []llmtool.Tool {
	base := clockToolBase{mgr: mgr, defType: defType, defID: defID}
	return []llmtool.Tool{
		&clockCreateTool{
			BaseTool:     llmtool.MakeBaseTool("clock_create", "创建一个AI定时任务。到点时会以全新的一次性上下文自动执行任务内容（可调用工具完成复杂流程），并把结果发到触发对象。cron用5字段(分 时 日 月 周)或@every 1h等", clockCreateParams{}),
			clockToolBase: base,
		},
		&clockListTool{
			BaseTool:     llmtool.MakeBaseTool("clock_list", "列出定时任务。不传参数默认列出当前会话的任务", clockListParams{}),
			clockToolBase: base,
		},
		&clockUpdateTool{
			BaseTool:     llmtool.MakeBaseTool("clock_update", "更新已存在的定时任务，仅更新提供的字段", clockUpdateParams{}),
			clockToolBase: base,
		},
		&clockDeleteTool{
			BaseTool:     llmtool.MakeBaseTool("clock_delete", "按ID删除定时任务", clockDeleteParams{}),
			clockToolBase: base,
		},
		&clockLogTool{
			BaseTool:     llmtool.MakeBaseTool("clock_log", "查看定时任务最近的执行记录（成功/超时/失败）", clockLogParams{}),
			clockToolBase: base,
		},
	}
}

// ---- clock_create ----

type clockCreateParams struct {
	Cron       string `json:"cron" desc:"cron定时表达式，5字段(分 时 日 月 周)如 0 8 * * *，或 @every 1h、@daily 等"`
	Title      string `json:"title" desc:"任务标题，简短描述任务目的"`
	Content    string `json:"content" desc:"任务内容，触发时作为对话内容发送给AI，应写清完整可执行的指令"`
	TargetType string `json:"target_type,omitempty" desc:"触发对象类型 group(群聊)/friend(私聊)，不填默认当前会话"`
	TargetID   uint64 `json:"target_id,omitempty" desc:"触发对象ID(群号或QQ号)，不填默认当前会话"`
	TimeoutSec int    `json:"timeout_sec,omitempty" desc:"单次执行超时秒数，不填用默认值"`
	Note       string `json:"note,omitempty" desc:"备注信息"`
}

type clockCreateTool struct {
	llmtool.BaseTool[clockCreateParams]
	clockToolBase
}

func (t *clockCreateTool) Execute(_ context.Context, params any, _ llmtool.CallBackFuncs) (string, error) {
	p := params.(*clockCreateParams)
	targetType, targetID := t.resolveTarget(p.TargetType, p.TargetID)
	task := &ClockTask{
		Cron:       p.Cron,
		Title:      p.Title,
		Content:    p.Content,
		TargetType: targetType,
		TargetID:   targetID,
		TimeoutSec: p.TimeoutSec,
		Note:       p.Note,
		Enabled:    true,
	}
	id, err := t.mgr.Add(task)
	if err != nil {
		return "", err
	}
	next := ""
	if !task.NextRunAt.IsZero() {
		next = "，下次触发 " + task.NextRunAt.Local().Format("01-02 15:04")
	}
	return fmt.Sprintf("已创建定时任务 ID=%s，cron=%s，目标=%s/%d%s", id, p.Cron, targetType, uint64(targetID), next), nil
}

// ---- clock_list ----

type clockListParams struct {
	TargetType string `json:"target_type,omitempty" desc:"按触发对象类型过滤 group/friend，不填默认当前会话"`
	TargetID   uint64 `json:"target_id,omitempty" desc:"按触发对象ID过滤，不填默认当前会话"`
}

type clockListTool struct {
	llmtool.BaseTool[clockListParams]
	clockToolBase
}

func (t *clockListTool) Execute(_ context.Context, params any, _ llmtool.CallBackFuncs) (string, error) {
	p := params.(*clockListParams)
	// 始终限定在当前会话（或显式指定的同会话对象），不提供跨会话列举能力——
	// 跨会话的全部任务列举仅管理员可通过 /clock list all 命令使用
	targetType, targetID := t.resolveTarget(p.TargetType, p.TargetID)
	tasks := t.mgr.ListByTarget(targetType, targetID)
	if len(tasks) == 0 {
		return "没有符合条件的定时任务", nil
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("定时任务（共 %d 条）：\n", len(tasks)))
	for _, task := range tasks {
		sb.WriteString(formatTaskLine(task))
		sb.WriteString("\n")
	}
	return sb.String(), nil
}

// ---- clock_update ----

type clockUpdateParams struct {
	ID         string  `json:"id" desc:"任务ID"`
	Cron       *string `json:"cron,omitempty" desc:"新的cron表达式"`
	Title      *string `json:"title,omitempty" desc:"新的标题"`
	Content    *string `json:"content,omitempty" desc:"新的任务内容"`
	TargetType *string `json:"target_type,omitempty" desc:"新的触发对象类型 group/friend"`
	TargetID   *uint64 `json:"target_id,omitempty" desc:"新的触发对象ID"`
	Enabled    *bool   `json:"enabled,omitempty" desc:"是否启用"`
	TimeoutSec *int    `json:"timeout_sec,omitempty" desc:"新的超时秒数"`
	Note       *string `json:"note,omitempty" desc:"新的备注"`
}

type clockUpdateTool struct {
	llmtool.BaseTool[clockUpdateParams]
	clockToolBase
}

func (t *clockUpdateTool) Execute(_ context.Context, params any, _ llmtool.CallBackFuncs) (string, error) {
	p := params.(*clockUpdateParams)
	if strings.TrimSpace(p.ID) == "" {
		return "", fmt.Errorf("id 不能为空")
	}
	f := ClockUpdateFields{
		Cron:       p.Cron,
		Title:      p.Title,
		Content:    p.Content,
		TargetType: p.TargetType,
		Enabled:    p.Enabled,
		TimeoutSec: p.TimeoutSec,
		Note:       p.Note,
	}
	if p.TargetID != nil {
		id := message.QID(*p.TargetID)
		f.TargetID = &id
	}
	task, err := t.mgr.Update(p.ID, f)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("已更新定时任务 ID=%s，状态=%s，cron=%s", task.ID, enabledText(task.Enabled), task.Cron), nil
}

// ---- clock_delete ----

type clockDeleteParams struct {
	ID string `json:"id" desc:"任务ID"`
}

type clockDeleteTool struct {
	llmtool.BaseTool[clockDeleteParams]
	clockToolBase
}

func (t *clockDeleteTool) Execute(_ context.Context, params any, _ llmtool.CallBackFuncs) (string, error) {
	p := params.(*clockDeleteParams)
	if strings.TrimSpace(p.ID) == "" {
		return "", fmt.Errorf("id 不能为空")
	}
	if t.mgr.Delete(p.ID) {
		return "已删除定时任务 " + p.ID, nil
	}
	return "", fmt.Errorf("定时任务不存在: %s", p.ID)
}

// ---- clock_log ----

type clockLogParams struct {
	TaskID string `json:"task_id,omitempty" desc:"按任务ID过滤，不填则查看最近的全部记录"`
	Limit  int    `json:"limit,omitempty" desc:"返回条数，默认10"`
}

type clockLogTool struct {
	llmtool.BaseTool[clockLogParams]
	clockToolBase
}

func (t *clockLogTool) Execute(_ context.Context, params any, _ llmtool.CallBackFuncs) (string, error) {
	p := params.(*clockLogParams)
	limit := p.Limit
	if limit <= 0 {
		limit = 10
	}
	var logs []tasklog.Entry
	if strings.TrimSpace(p.TaskID) != "" {
		logs = t.mgr.log.RecentForTask(p.TaskID, limit)
	} else {
		logs = t.mgr.log.Recent(limit)
	}
	if len(logs) == 0 {
		return "暂无执行记录", nil
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("最近 %d 条执行记录：\n", len(logs)))
	for _, e := range logs {
		sb.WriteString(formatLogLine(e))
		sb.WriteString("\n")
	}
	return sb.String(), nil
}
