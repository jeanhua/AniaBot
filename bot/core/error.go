package core

import (
	"context"
	"errors"
	"log"

	"github.com/jeanhua/AniaBot/common/plugin"
)

func logError(err error, p plugin.Plugin, tag string) {
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			log.Println(tag+"执行超时", p.GetMeta().Name)
		} else {
			log.Println(tag+"执行错误", p.GetMeta().Name, err)
		}
	}
}
