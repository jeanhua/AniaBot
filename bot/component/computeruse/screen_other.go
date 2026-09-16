//go:build !windows

package computeruse

import "fmt"

// Bounds 一块屏幕区域（虚拟桌面坐标系）；非 Windows 平台仅保留类型定义。
type Bounds struct {
	X, Y, W, H int
}

// Capture 一次屏幕截图的产物；非 Windows 平台不会产出。
type Capture struct {
	PNG              []byte
	OriginX, OriginY int
	ScreenW, ScreenH int
	ImgW, ImgH       int
}

func VirtualScreenBounds() (Bounds, error) { return Bounds{}, ErrUnsupported }

func CaptureScreen(rect *Bounds, maxWidth int) (*Capture, error) {
	return nil, fmt.Errorf("截图失败: %w", ErrUnsupported)
}
