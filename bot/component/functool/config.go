package functool

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jeanhua/AniaBot/bot/component/llmtool"
	"github.com/jeanhua/AniaBot/bot/component/oplog"
	"github.com/jeanhua/AniaBot/bot/component/sysrestart"
	"github.com/jeanhua/AniaBot/common/pluginconfig"
)

// ConfigStore 配置中心读写能力（由 core 的 configstore 经插件 DI 注入，
// 与 plugin.ConfigEditor 结构一致）。配置改动写入数据库，重启后生效。
type ConfigStore interface {
	Get(key string) (any, bool)
	Set(key string, val any) error
	Delete(key string) bool
	All() map[string]any
}

// configMask 敏感字段（API Key / Token 等）对 AI 的掩码，避免密钥进入对话上下文
const configMask = "********"

// configListMaxRunes config_get 全量列表输出的符文数上限，防止撑大上下文
const configListMaxRunes = 12000

// configFieldIndex 以配置键为索引的注册表快照（label/help/sensitive 元信息）。
func configFieldIndex() map[string]pluginconfig.Field {
	out := make(map[string]pluginconfig.Field)
	for _, f := range pluginconfig.Fields() {
		out[f.Key] = f
	}
	return out
}

// formatConfigValue 把配置值格式化为紧凑 JSON 文本。
func formatConfigValue(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(data)
}

// ---- config_get：查看配置 ----

type ConfigGetParams struct {
	Key string `json:"key,omitempty" desc:"要查看的配置键（如 plugin.ai_chat_bot.model）。留空则列出全部已注册配置项及其当前值"`
}

type ConfigGetTool struct {
	llmtool.BaseTool[ConfigGetParams]
	store ConfigStore
}

func NewConfigGetTool(store ConfigStore) *ConfigGetTool {
	return &ConfigGetTool{
		BaseTool: llmtool.MakeBaseTool("config_get", "查看 Bot 框架配置。留空 key 时列出全部配置项（键、当前值、说明）；指定 key 时返回该配置项的当前值。敏感字段（API Key/Token 等）已掩码。配置键为点分格式，如 plugin.ai_chat_bot.model、bot.admin_id", ConfigGetParams{}),
		store:    store,
	}
}

func (t *ConfigGetTool) Execute(ctx context.Context, params any, _ llmtool.CallBackFuncs) (string, error) {
	p, ok := params.(*ConfigGetParams)
	if !ok {
		return "", fmt.Errorf("config_get: 参数类型错误")
	}
	key := strings.ToLower(strings.TrimSpace(p.Key))
	log.Println("执行config_get... 参数:", key)

	fields := configFieldIndex()

	if key != "" {
		v, ok := t.store.Get(key)
		if !ok {
			if f, registered := fields[key]; registered {
				return fmt.Sprintf("配置项 %s（%s）当前未设置", key, f.Label), nil
			}
			return "", fmt.Errorf("config_get: 配置项 %s 不存在（留空 key 可列出全部配置项）", key)
		}
		val := formatConfigValue(v)
		if f, registered := fields[key]; registered && f.Sensitive {
			val = configMask + "（敏感字段已掩码）"
		}
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("%s = %s", key, val))
		if f, registered := fields[key]; registered {
			sb.WriteString(fmt.Sprintf("\n说明: %s", f.Label))
			if f.Help != "" {
				sb.WriteString(fmt.Sprintf("（%s）", f.Help))
			}
		}
		return sb.String(), nil
	}

	// 全量列表：按注册表的键排序逐行列出，敏感字段掩码
	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var sb strings.Builder
	sb.WriteString("全部配置项（key = 当前值 # 说明；修改配置用 config_set，重启后生效）：\n")
	truncated := 0
	for _, k := range keys {
		f := fields[k]
		val := "（未设置）"
		if v, ok := t.store.Get(k); ok {
			val = formatConfigValue(v)
			if f.Sensitive {
				val = configMask
			}
		}
		line := fmt.Sprintf("%s = %s # %s\n", k, val, f.Label)
		if utf8.RuneCountInString(sb.String())+utf8.RuneCountInString(line) > configListMaxRunes {
			truncated++
			continue
		}
		sb.WriteString(line)
	}
	if truncated > 0 {
		sb.WriteString(fmt.Sprintf("...（其余 %d 项因长度限制未列出，请用 key 参数单独查看）", truncated))
	}
	return sb.String(), nil
}

// ---- config_set：修改配置 ----

type ConfigSetParams struct {
	Key   string `json:"key" desc:"要修改的配置键（必须已在配置注册表中声明，如 plugin.ai_chat_bot.model）"`
	Value string `json:"value" desc:"新的配置值。字符串直接写内容；数字/布尔/数组/对象用 JSON 表示（如 true、0.6、[\"a\",\"b\"]）"`
}

type ConfigSetTool struct {
	llmtool.BaseTool[ConfigSetParams]
	store ConfigStore
}

func NewConfigSetTool(store ConfigStore) *ConfigSetTool {
	return &ConfigSetTool{
		BaseTool: llmtool.MakeBaseTool("config_set", "修改 Bot 框架配置（写入数据库，重启后生效；可用 restart_bot 工具重启使其生效）。只能修改已注册的配置键，先用 config_get 查看可用键与当前值。修改前请确认用户已明确要求", ConfigSetParams{}),
		store:    store,
	}
}

func (t *ConfigSetTool) Execute(ctx context.Context, params any, _ llmtool.CallBackFuncs) (string, error) {
	p, ok := params.(*ConfigSetParams)
	if !ok {
		return "", fmt.Errorf("config_set: 参数类型错误")
	}
	key := strings.ToLower(strings.TrimSpace(p.Key))
	if key == "" {
		return "", fmt.Errorf("config_set: key 不能为空")
	}
	log.Println("执行config_set... 参数:", key)

	// 只允许修改已注册的配置键：防止拼写错误产生垃圾键，也挡住 meta.* 等内部键
	fields := configFieldIndex()
	f, registered := fields[key]
	if !registered {
		return "", fmt.Errorf("config_set: 配置键 %s 未在注册表中声明，不允许修改（用 config_get 查看可用键）", key)
	}
	// 与面板一致：面板监听地址不允许置空
	if key == "bot.admin_panel.listen" && strings.TrimSpace(p.Value) == "" {
		return "", fmt.Errorf("config_set: %s 不允许置空", key)
	}

	// 值解析：优先按 JSON 解码（保留 bool/数字/数组类型），失败则按纯字符串
	var val any = p.Value
	if decoded, err := decodeJSONValue(p.Value); err == nil {
		val = decoded
	}

	old, hadOld := t.store.Get(key)
	if err := t.store.Set(key, val); err != nil {
		return "", fmt.Errorf("config_set: 写入失败: %w", err)
	}

	// 操作日志：敏感字段不记录真实值
	detailVal := formatConfigValue(val)
	detailOld := "（未设置）"
	if hadOld {
		detailOld = formatConfigValue(old)
	}
	if f.Sensitive {
		detailVal = configMask
		if hadOld {
			detailOld = configMask
		}
	}
	oplog.Record(oplog.CategoryAI, "config_set", fmt.Sprintf("AI 修改配置 %s: %s → %s", key, detailOld, detailVal))

	return fmt.Sprintf("已更新配置 %s = %s（重启后生效，可告知用户使用 restart_bot 重启）", key, detailVal), nil
}

// decodeJSONValue 按 JSON 解码配置值（bool/数字/数组/对象/带引号字符串）。
func decodeJSONValue(s string) (any, error) {
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return nil, err
	}
	return v, nil
}

// ---- restart_bot：重启使配置生效 ----

type RestartBotParams struct {
	DelaySec int `json:"delay_sec,omitempty" desc:"延迟多少秒后重启（默认 5 秒，给当前回复留出发送时间；范围 3-120）"`
}

// restartDelayBounds restart_bot 延迟秒数的限幅范围
const (
	restartDelayDefault = 5
	restartDelayMin     = 3
	restartDelayMax     = 120
)

type RestartBotTool struct {
	llmtool.BaseTool[RestartBotParams]
	logger *slog.Logger
}

func NewRestartBotTool(logger *slog.Logger) *RestartBotTool {
	if logger == nil {
		logger = slog.Default()
	}
	return &RestartBotTool{
		BaseTool: llmtool.MakeBaseTool("restart_bot", "重启 Bot 进程（以相同参数重新启动），用于使 config_set 修改的配置生效。重启期间会短暂离线，重启完成后自动恢复。执行前请确认用户已明确要求重启", RestartBotParams{}),
		logger:   logger,
	}
}

func (t *RestartBotTool) Execute(ctx context.Context, params any, _ llmtool.CallBackFuncs) (string, error) {
	p, ok := params.(*RestartBotParams)
	if !ok {
		return "", fmt.Errorf("restart_bot: 参数类型错误")
	}
	delay := p.DelaySec
	if delay <= 0 {
		delay = restartDelayDefault
	}
	delay = max(restartDelayMin, min(delay, restartDelayMax))
	log.Printf("执行restart_bot... 延迟: %ds", delay)

	oplog.Record(oplog.CategoryAI, "restart", fmt.Sprintf("AI 请求重启 Bot（延迟 %d 秒）", delay))

	// 延迟重启：先返回工具结果让回复走完发送流程，再替换进程
	go func() {
		time.Sleep(time.Duration(delay) * time.Second)
		sysrestart.Self(t.logger)
	}()
	return fmt.Sprintf("已安排 %d 秒后重启 Bot，重启期间将短暂离线，配置修改会在重启后生效", delay), nil
}
