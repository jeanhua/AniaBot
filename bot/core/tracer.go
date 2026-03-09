package core

import (
	"context"
	"time"
)

const (
	PanicTimeout = 5 * time.Second
)

func (ania *AniaBot) Go(name string, f func()) {
	ania.goroutineNum.Add(1)
	ania.logger.Debug("启动协程", "name", name, "goroutineNum", ania.goroutineNum.Load())
	go func() {
		defer func() {
			ania.goroutineNum.Add(-1)
			ania.logger.Debug("协程结束", "name", name, "goroutineNum", ania.goroutineNum.Load())
		}()
		defer func() {
			if err := recover(); err != nil {
				ania.logger.Error("goroutine panic", "name", name, "err", err)
				for _, plugin := range ania.plugins {
					ctx, cancel := context.WithTimeout(ania.ctx, PanicTimeout)
					defer cancel()
					plugin.OnPanic(ctx, ania, name, err)
				}
			}
		}()
		f()
	}()
}
