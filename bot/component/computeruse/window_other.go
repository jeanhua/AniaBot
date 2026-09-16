//go:build !windows

package computeruse

import "fmt"

// WindowInfo 一个顶层窗口的信息
type WindowInfo struct {
	Title string `json:"title"`
	Class string `json:"class"`
	X     int    `json:"x"`
	Y     int    `json:"y"`
	W     int    `json:"w"`
	H     int    `json:"h"`
}

func ForegroundWindow() (*WindowInfo, error) {
	return nil, fmt.Errorf("获取前台窗口失败: %w", ErrUnsupported)
}

func ListWindows(limit int) ([]WindowInfo, error) {
	return nil, fmt.Errorf("枚举窗口失败: %w", ErrUnsupported)
}
