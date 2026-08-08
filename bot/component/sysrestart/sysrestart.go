// Package sysrestart 提供进程自重启能力。
//
// 供 Web 控制面板（重启按钮 / 自动更新）与 AI 工具（restart_bot）共用：
// 以相同命令行参数重启当前进程，使配置修改生效。
package sysrestart

import (
	"os"
	"sync"
	"sync/atomic"
)

// selfExe 进程启动时捕获的可执行文件路径。
//
// 自动更新的「改名交换」会把运行中的二进制 rename 为 <exe>.old，此后在
// Linux 上再调 os.Executable() 读到的 /proc/self/exe 跟随 inode，会指向
// 旧二进制（<exe>.old），导致 swap 换错文件、exec 重启回旧版本。
// 因此必须在任何 rename 发生之前（包初始化时）把路径固定下来，
// 更新替换与重启统一使用这个启动时的路径。
var selfExe = captureSelfExe()

var (
	restartMu      sync.Mutex
	restartStarted atomic.Bool
)

// captureSelfExe 返回当前可执行文件路径，失败时返回空串。
func captureSelfExe() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return exe
}

// Exe 返回启动时缓存的可执行文件路径（见 selfExe 的注释）。
func Exe() string {
	return selfExe
}
