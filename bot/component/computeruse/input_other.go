//go:build !windows

package computeruse

import "fmt"

func MoveMouse(x, y int) error {
	return fmt.Errorf("移动鼠标失败: %w", ErrUnsupported)
}

func ClickMouseButton(button string, x, y int, double bool) error {
	return fmt.Errorf("鼠标点击失败: %w", ErrUnsupported)
}

func ScrollWheel(delta int, x, y int, horizontal bool) error {
	return fmt.Errorf("滚轮滚动失败: %w", ErrUnsupported)
}

func TypeText(text string) error {
	return fmt.Errorf("键盘输入失败: %w", ErrUnsupported)
}

func PressKeys(combo string, times int) error {
	return fmt.Errorf("按键失败: %w", ErrUnsupported)
}
