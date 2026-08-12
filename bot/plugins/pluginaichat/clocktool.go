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
	defID   string
}

// resolveTarget 解析工具参数中的触发对象：仅当类型与 ID 均合理时采用，否则回退到当前会话。
func (b clockToolBase) resolveTarget(givenType string, givenID string) (string, string) {
	if givenID != "" && (givenType == clockTargetGroup || givenType == clockTargetFriend) {
		return givenType, givenID
	}
	return b.defType, b.defID
}

// resolveOwnedTarget 解析并校验触发对象：显式指定时必须与当前会话一致，
// 否则拒绝——AI 工具只能操作当前会话的定时任务，防止跨会话越权
// （任务 ID 为自增序号可枚举，按 ID 直接操作会命中其他会话的任务）。
func (b clockToolBase) resolveOwnedTarget(givenType string, givenID string) (string, string, error) {
	if givenID == "" {
		return b.defType, b.defID, nil
	}
	if givenType != b.defType || givenID != b.defID {
		return "", "", fmt.Errorf("只能操作当前会话的定时任务（当前会话为 %s/%s）", b.defType, b.defID)
	}
	return b.defType, b.defID, nil
}

// checkTaskOwned 校验任务归属：任务必须属于当前会话（AI 工具无管理员豁免）。
func (b clockToolBase) checkTaskOwned(t *ClockTask) error {
	if t == nil {
		return fmt.Errorf("定时任务不存在")
	}
	if t.TargetType != b.defType || t.TargetID != b.defID {
		return fmt.Errorf("无权操作该定时任务（只能操作当前会话的任务）")
	}
	return nil
}

// newClockTools 创建一组定时任务管理工具，注册到当前会话的执行器中。
// defType/defID 为当前会话的触发对象，作为工具参数缺省时的默认值。
func newClockTools(mgr *clockManager, defType string, defID string) []llmtool.Tool {
	base := clockToolBase{mgr: mgr, defType: defType, defID: defID}
	// 把当前会话场景写进工具描述，让 AI 明确自己处于群聊还是私聊、默认发到哪个对象
	sessionDesc := "私聊（对方 ID " + defID + "）"
	if defType == clockTargetGroup {
		sessionDesc = "群聊（会话 ID " + defID + "）"
	}
	return []llmtool.Tool{
		&clockCreateTool{
			BaseTool:      llmtool.MakeBaseTool("clock_create", "创建一个AI定时任务。到点时会以全新的一次性上下文自动执行任务内容（可调用工具完成复杂流程），并把结果发到触发对象。当前会话为"+sessionDesc+"，target_type/target_id不填时默认发到当前会话。cron用5字段(分 时 日 月 周)或@every 1h等。run_once=true为单次任务，触发执行一次后自动销毁；默认false为重复执行", clockCreateParams{}),
			clockToolBase: base,
		},
		&clockListTool{
			BaseTool:      llmtool.MakeBaseTool("clock_list", "列出定时任务。当前会话为"+sessionDesc+"，不传参数默认列出当前会话的任务", clockListParams{}),
			clockToolBase: base,
		},
		&clockUpdateTool{
			BaseTool:      llmtool.MakeBaseTool("clock_update", "更新已存在的定时任务，仅更新提供的字段", clockUpdateParams{}),
			clockToolBase: base,
		},
		&clockDeleteTool{
			BaseTool:      llmtool.MakeBaseTool("clock_delete", "按ID删除定时任务", clockDeleteParams{}),
			clockToolBase: base,
		},
		&clockLogTool{
			BaseTool:      llmtool.MakeBaseTool("clock_log", "查看定时任务最近的执行记录（成功/超时/失败）", clockLogParams{}),
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
	TargetID   string `json:"target_id,omitempty" desc:"触发对象ID（QQ 为 qq:群号/QQ号，其他平台为带前缀的ID，如 fs:oc_xxx），不填默认当前会话"`
	CreatedBy  string `json:"created_by,omitempty" desc:"群聊任务触发时要@的用户ID（QQ 为 qq:QQ号，其他平台为带前缀的ID，如 fs:ou_xxx），一般填当前让你创建任务的群成员（其消息以 [nickname:昵称 id:用户ID] 开头，取其中的id）；不填则触发时不@任何人；私聊任务无需填写"`
	RunOnce    bool   `json:"run_once,omitempty" desc:"是否为单次任务：true表示触发执行一次后自动销毁，false(默认)为重复执行"`
	TimeoutSec int    `json:"timeout_sec,omitempty" desc:"单次执行超时秒数，不填用默认值"`
	Note       string `json:"note,omitempty" desc:"备注信息"`
}

type clockCreateTool struct {
	llmtool.BaseTool[clockCreateParams]
	clockToolBase
}

func (t *clockCreateTool) Execute(_ context.Context, params any, _ llmtool.CallBackFuncs) (string, error) {
	p := params.(*clockCreateParams)
	// 只允许创建发到当前会话的任务，防止跨会话安排定时消息
	targetType, targetID, err := t.resolveOwnedTarget(p.TargetType, p.TargetID)
	if err != nil {
		return "", err
	}
	var creator message.QID
	if s := strings.TrimSpace(p.CreatedBy); s != "" {
		if s == "0" {
			return "", fmt.Errorf("created_by 必须是用户 ID")
		}
		creator = parseQID(s)
	}
	task := &ClockTask{
		Cron:       p.Cron,
		Title:      p.Title,
		Content:    p.Content,
		TargetType: targetType,
		TargetID:   targetID,
		RunOnce:    p.RunOnce,
		TimeoutSec: p.TimeoutSec,
		Note:       p.Note,
		Enabled:    true,
		CreatedBy:  creator,
	}
	id, err := t.mgr.Add(task)
	if err != nil {
		return "", err
	}
	next := ""
	if !task.NextRunAt.IsZero() {
		next = "，下次触发 " + task.NextRunAt.Local().Format("01-02 15:04")
	}
	mode := "重复"
	if p.RunOnce {
		mode = "单次"
	}
	return fmt.Sprintf("已创建定时任务 ID=%s，模式=%s，cron=%s，目标=%s/%s%s", id, mode, p.Cron, targetType, targetID, next), nil
}

// ---- clock_list ----

type clockListParams struct {
	TargetType string `json:"target_type,omitempty" desc:"按触发对象类型过滤 group/friend，不填默认当前会话"`
	TargetID   string `json:"target_id,omitempty" desc:"按触发对象ID过滤，不填默认当前会话"`
}

type clockListTool struct {
	llmtool.BaseTool[clockListParams]
	clockToolBase
}

func (t *clockListTool) Execute(_ context.Context, params any, _ llmtool.CallBackFuncs) (string, error) {
	p := params.(*clockListParams)
	// 始终限定在当前会话，不提供跨会话列举能力——
	// 跨会话的全部任务列举仅管理员可通过 /clock list all 命令使用
	targetType, targetID, err := t.resolveOwnedTarget(p.TargetType, p.TargetID)
	if err != nil {
		return "", err
	}
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
	TargetID   *string `json:"target_id,omitempty" desc:"新的触发对象ID"`
	Enabled    *bool   `json:"enabled,omitempty" desc:"是否启用"`
	RunOnce    *bool   `json:"run_once,omitempty" desc:"是否改为单次任务（true=触发一次后销毁，false=重复执行）"`
	TimeoutSec *int    `json:"timeout_sec,omitempty" desc:"新的超时秒数"`
	Note       *string `json:"note,omitempty" desc:"新的备注"`
	CreatedBy  *string `json:"created_by,omitempty" desc:"新的@提醒用户ID（仅群聊任务生效，格式同 clock_create），传空字符串表示触发时不再@任何人"`
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
	// 归属校验：只能更新当前会话的任务，且更新后的触发对象仍必须是当前会话
	task, ok := t.mgr.Get(p.ID)
	if !ok {
		return "", fmt.Errorf("定时任务不存在: %s", p.ID)
	}
	if err := t.checkTaskOwned(task); err != nil {
		return "", err
	}
	if p.TargetType != nil || p.TargetID != nil {
		tt := t.defType
		if p.TargetType != nil {
			tt = *p.TargetType
		}
		tid := t.defID
		if p.TargetID != nil {
			tid = *p.TargetID
		}
		if _, _, err := t.resolveOwnedTarget(tt, tid); err != nil {
			return "", err
		}
	}
	f := ClockUpdateFields{
		Cron:       p.Cron,
		Title:      p.Title,
		Content:    p.Content,
		TargetType: p.TargetType,
		TargetID:   p.TargetID,
		Enabled:    p.Enabled,
		RunOnce:    p.RunOnce,
		TimeoutSec: p.TimeoutSec,
		Note:       p.Note,
		CreatedBy:  p.CreatedBy,
	}
	task, err := t.mgr.Update(p.ID, f)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("已更新定时任务 ID=%s，状态=%s，模式=%s，cron=%s", task.ID, enabledText(task.Enabled), runOnceText(task.RunOnce), task.Cron), nil
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
	// 归属校验：只能删除当前会话的任务
	task, ok := t.mgr.Get(p.ID)
	if !ok {
		return "", fmt.Errorf("定时任务不存在: %s", p.ID)
	}
	if err := t.checkTaskOwned(task); err != nil {
		return "", err
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
		// 指定任务：校验归属后查该任务日志
		task, ok := t.mgr.Get(p.TaskID)
		if !ok {
			return "", fmt.Errorf("定时任务不存在: %s", p.TaskID)
		}
		if err := t.checkTaskOwned(task); err != nil {
			return "", err
		}
		logs = t.mgr.log.RecentForTask(p.TaskID, limit)
	} else {
		// 未指定任务：只返回当前会话任务的日志，不暴露其他会话的执行记录
		logs = t.mgr.log.RecentForTarget(t.defType, t.defID, limit)
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
