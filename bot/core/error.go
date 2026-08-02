package core

import (
	"context"
	"errors"

	"github.com/jeanhua/AniaBot/common/plugin"
)

func logError(err error, p plugin.Plugin, tag string) {
	if err == nil {
		return
	}
	switch {
	case errors.Is(err, context.Canceled):
		// 用户主动取消（如 /stop）或 Bot 停止，属正常流程，不记错误
	case errors.Is(err, context.DeadlineExceeded):
		Logger().Error("执行超时", "tag", tag, "plugin", p.GetMeta().Name)
	default:
		Logger().Error("执行错误", "tag", tag, "plugin", p.GetMeta().Name, "error", err)
	}
}
