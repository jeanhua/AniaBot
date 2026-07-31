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
					// 立即释放：外层 recover 已消费本次 panic，插件 OnPanic 再 panic
					// 会传播出该 goroutine 直接终止整个进程，必须逐个恢复
					func() {
						defer cancel()
						defer func() { _ = recover() }()
						plugin.OnPanic(ctx, ania, name, err)
					}()
				}
			}
		}()
		f()
	}()
}
