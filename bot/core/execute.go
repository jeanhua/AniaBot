package core

import (
	"github.com/jeanhua/AniaBot/common/plugin"
)

func safeExecuteWithReturn[T any](label string, p plugin.Plugin, f func(p plugin.Plugin) T) T {
	defer func() {
		if err := recover(); err != nil {
			Logger().Printf("%s: 插件[%s]触发错误 %v", label, p.GetMeta().Name, err)
		}
	}()
	return f(p)
}

func safeExecute(label string, p plugin.Plugin, f func(p plugin.Plugin)) {
	defer func() {
		if err := recover(); err != nil {
			Logger().Printf("%s: 插件[%s]触发错误 %v", label, p.GetMeta().Name, err)
		}
	}()
	f(p)
}
