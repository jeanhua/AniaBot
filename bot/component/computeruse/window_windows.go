//go:build windows

package computeruse

import (
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	procGetForegroundWindow  = user32.NewProc("GetForegroundWindow")
	procGetWindowTextW       = user32.NewProc("GetWindowTextW")
	procGetWindowTextLengthW = user32.NewProc("GetWindowTextLengthW")
	procGetClassNameW        = user32.NewProc("GetClassNameW")
	procGetWindowRect        = user32.NewProc("GetWindowRect")
	procIsWindowVisible      = user32.NewProc("IsWindowVisible")
	procGetWindowLongW       = user32.NewProc("GetWindowLongW")
	procEnumWindows          = user32.NewProc("EnumWindows")
)

const (
	// gwlExStyleIndex 即 GWL_EXSTYLE(-20) 的 uintptr 补码表示
	gwlExStyleIndex = ^uintptr(19)
	wsExToolWindow  = 0x00000080
	maxWindowTitle  = 200 // 窗口标题截断长度：标题可能极长，列表直出会刷爆上下文
)

type rect struct {
	Left, Top, Right, Bottom int32
}

// WindowInfo 一个顶层窗口的信息
type WindowInfo struct {
	Title string `json:"title"`
	Class string `json:"class"`
	X     int    `json:"x"`
	Y     int    `json:"y"`
	W     int    `json:"w"`
	H     int    `json:"h"`
}

// windowInfoOf 读取单个窗口的标题/类名/位置；读取失败的字段留空（不中断）。
func windowInfoOf(hwnd uintptr) WindowInfo {
	info := WindowInfo{}

	if n, _, _ := procGetWindowTextLengthW.Call(hwnd); n > 0 {
		buf := make([]uint16, n+1)
		procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), n+1)
		info.Title = truncateRunes(windows.UTF16ToString(buf), maxWindowTitle)
	}

	classBuf := make([]uint16, 256)
	procGetClassNameW.Call(hwnd, uintptr(unsafe.Pointer(&classBuf[0])), uintptr(len(classBuf)))
	info.Class = windows.UTF16ToString(classBuf)

	var r rect
	if _, _, _ = procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&r))); r.Right > r.Left && r.Bottom > r.Top {
		info.X, info.Y = int(r.Left), int(r.Top)
		info.W, info.H = int(r.Right-r.Left), int(r.Bottom-r.Top)
	}
	return info
}

// ForegroundWindow 返回当前前台（获得焦点）窗口的信息。
func ForegroundWindow() (*WindowInfo, error) {
	hwnd, _, _ := procGetForegroundWindow.Call()
	if hwnd == 0 {
		return nil, fmt.Errorf("当前没有前台窗口")
	}
	info := windowInfoOf(hwnd)
	return &info, nil
}

// ListWindows 枚举可见的顶层窗口（跳过隐藏窗口与工具窗口，如输入法悬浮条），
// 最多返回 limit 个（<=0 时默认 50）。
func ListWindows(limit int) ([]WindowInfo, error) {
	if limit <= 0 {
		limit = 50
	}
	list := make([]WindowInfo, 0, limit)
	cb := syscall.NewCallback(func(hwnd, lparam uintptr) uintptr {
		if len(list) >= limit {
			return 0 // 停止枚举
		}
		if vis, _, _ := procIsWindowVisible.Call(hwnd); vis == 0 {
			return 1
		}
		if style, _, _ := procGetWindowLongW.Call(hwnd, gwlExStyleIndex); style&wsExToolWindow != 0 {
			return 1
		}
		info := windowInfoOf(hwnd)
		if info.Title == "" {
			return 1
		}
		list = append(list, info)
		return 1
	})
	procEnumWindows.Call(cb, 0)
	return list, nil
}

// truncateRunes 按 rune 截断字符串（标题可能含多字节字符）。
func truncateRunes(s string, maxRunes int) string {
	r := []rune(s)
	if len(r) <= maxRunes {
		return s
	}
	return string(r[:maxRunes]) + "…"
}
