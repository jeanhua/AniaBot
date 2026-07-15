package core

import (
	"github.com/jeanhua/AniaBot/common/plugin"
)

// safeExecuteWithReturn 执行 f 并捕获 panic，返回 f 的结果以及是否发生 panic。
// 调用方应将 panicked 视为“不应阻断后续流程”：例如消息事件链中某个插件 panic
// 时，不应让后续插件被跳过（与 notice 事件的 void safeExecute 行为一致）。
func safeExecuteWithReturn[T any](label string, p plugin.Plugin, f func(p plugin.Plugin) T) (result T, panicked bool) {
	defer func() {
		if err := recover(); err != nil {
			Logger().Error("插件触发错误", "label", label, "plugin", p.GetMeta().Name, "error", err)
			panicked = true
		}
	}()
	result = f(p)
	return result, false
}

func safeExecute(label string, p plugin.Plugin, f func(p plugin.Plugin)) {
	defer func() {
		if err := recover(); err != nil {
			Logger().Error("插件触发错误", "label", label, "plugin", p.GetMeta().Name, "error", err)
		}
	}()
	f(p)
}
