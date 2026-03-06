package core

import (
	"context"
	"errors"

	"github.com/jeanhua/AniaBot/common/plugin"
)

func logError(err error, p plugin.Plugin, tag string) {
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			Logger().Error("执行超时", "tag", tag, "plugin", p.GetMeta().Name)
		} else {
			Logger().Error("执行错误", "tag", tag, "plugin", p.GetMeta().Name, "error", err)
		}
	}
}
