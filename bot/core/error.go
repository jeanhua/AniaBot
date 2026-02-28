package core

import (
	"context"
	"errors"

	"github.com/jeanhua/AniaBot/common/plugin"
)

func logError(err error, p plugin.Plugin, tag string) {
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			Logger().Println(tag+"执行超时", p.GetMeta().Name)
		} else {
			Logger().Println(tag+"执行错误", p.GetMeta().Name, err)
		}
	}
}
