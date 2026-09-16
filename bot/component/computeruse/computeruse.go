// Package computeruse 提供宿主机屏幕/鼠标/键盘的底层操作能力（computer use），
// 供 LLM 工具层（functool）封装为 AI 可调用的工具。
//
// 当前仅实现 Windows 后端（纯 syscall，无 CGO，交叉编译友好）；
// 其他平台编译通过但运行时报 ErrUnsupported。
// 本包只做系统调用，不依赖 LLM/消息链路；副作用与安全门禁在工具层处理。
package computeruse

import (
	"errors"
	"runtime"
)

// ErrUnsupported 当前平台不支持电脑操作（非 Windows）。
var ErrUnsupported = errors.New("computeruse: 当前平台不支持电脑操作（仅支持 Windows）")

// Available 报告当前平台是否支持电脑操作。
func Available() bool {
	return runtime.GOOS == "windows"
}
