package agenthook

import (
	"fmt"
	"regexp"
	"strings"
)

// compiledHook 编译后的钩子：matcher 已预编译（nil = 匹配全部）
type compiledHook struct {
	spec    ShellHookSpec
	matcher *regexp.Regexp
}

// matches 工具事件按工具名匹配；非工具事件 toolName 为空串，
// 带 matcher 的钩子自然不匹配（空 matcher 匹配一切）。
func (c compiledHook) matches(toolName string) bool {
	return c.matcher == nil || c.matcher.MatchString(toolName)
}

// compileHooks 编译钩子配置：空命令跳过；非法事件名/正则返回 error
// （含事件名与规则文本），整体配置视为无效、沿用旧快照。
func compileHooks(cfg *FileConfig) (map[Event][]compiledHook, error) {
	out := make(map[Event][]compiledHook)
	if cfg == nil {
		return out, nil
	}
	for ev, specs := range cfg.Hooks {
		if !ev.valid() {
			return nil, fmt.Errorf("未知钩子事件 %q（可选 SessionStart/UserPromptSubmit/PreToolUse/PostToolUse/Stop/SubagentStop/PreCompact）", ev)
		}
		for _, spec := range specs {
			if strings.TrimSpace(spec.Command) == "" {
				continue // 空命令无意义，跳过
			}
			ch := compiledHook{spec: spec}
			if strings.TrimSpace(spec.Matcher) != "" {
				re, err := regexp.Compile(spec.Matcher)
				if err != nil {
					return nil, fmt.Errorf("编译 %s 钩子的匹配规则 %q 失败: %w", ev, spec.Matcher, err)
				}
				ch.matcher = re
			}
			out[ev] = append(out[ev], ch)
		}
	}
	return out, nil
}
