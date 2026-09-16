//go:build windows

package computeruse

import (
	"fmt"
	"time"
	"unicode/utf16"
	"unsafe"
)

var procSendInput = user32.NewProc("SendInput")

// SendInput 相关常量
const (
	inputMouse    = 0
	inputKeyboard = 1

	mouseEventMove        = 0x0001
	mouseEventLeftDown    = 0x0002
	mouseEventLeftUp      = 0x0004
	mouseEventRightDown   = 0x0008
	mouseEventRightUp     = 0x0010
	mouseEventMiddleDown  = 0x0020
	mouseEventMiddleUp    = 0x0040
	mouseEventWheel       = 0x0800
	mouseEventHWheel      = 0x1000
	mouseEventVirtualDesk = 0x4000
	mouseEventAbsolute    = 0x8000

	keyEventKeyUp   = 0x0002
	keyEventUnicode = 0x0004

	wheelDelta = 120
)

// mouseInput 对应 Win32 MOUSEINPUT（联合体中最大的成员，用作 INPUT 载体，
// 保证 Go 侧 INPUT 与 C 侧布局/尺寸一致：amd64 40 字节、386 28 字节）。
type mouseInput struct {
	Dx, Dy      int32
	MouseData   uint32
	DwFlags     uint32
	Time        uint32
	DwExtraInfo uintptr
}

// keyboardInput 对应 Win32 KEYBDINPUT（复用 mouseInput 的内存空间）
type keyboardInput struct {
	WVk, WScan  uint16
	DwFlags     uint32
	Time        uint32
	DwExtraInfo uintptr
}

type input struct {
	Type uint32
	Mi   mouseInput // 与 KEYBDINPUT/HARDWAREINPUT 重叠的联合体
}

// sendInput 批量注入输入事件；返回 0 表示被系统拒绝（如 UIPI 拦截）。
func sendInput(inputs ...input) error {
	if len(inputs) == 0 {
		return nil
	}
	r, _, _ := procSendInput.Call(
		uintptr(len(inputs)),
		uintptr(unsafe.Pointer(&inputs[0])),
		unsafe.Sizeof(input{}))
	if r == 0 {
		return fmt.Errorf("SendInput 被系统拒绝（目标窗口权限高于本进程时会触发 UIPI 拦截）")
	}
	return nil
}

// moveMouse 把鼠标移动到虚拟桌面坐标（仅移动，不点击）。
func moveMouse(x, y int) error {
	b, err := VirtualScreenBounds()
	if err != nil {
		return err
	}
	ax, ay := VirtualToAbsolute(b, x, y)
	return sendInput(input{Type: inputMouse, Mi: mouseInput{
		Dx: ax, Dy: ay,
		DwFlags: mouseEventMove | mouseEventAbsolute | mouseEventVirtualDesk,
	}})
}

// ClickMouseButton 在 (x, y) 处点击鼠标键（坐标为虚拟桌面像素）；
// double 为 true 时连击两次（间隔落在系统双击判定时间内）。
func ClickMouseButton(button string, x, y int, double bool) error {
	var down, up uint32
	switch button {
	case "left", "":
		down, up = mouseEventLeftDown, mouseEventLeftUp
	case "right":
		down, up = mouseEventRightDown, mouseEventRightUp
	case "middle":
		down, up = mouseEventMiddleDown, mouseEventMiddleUp
	default:
		return fmt.Errorf("不支持的鼠标键 %q（可选 left/right/middle）", button)
	}
	if err := moveMouse(x, y); err != nil {
		return err
	}
	click := input{Type: inputMouse, Mi: mouseInput{DwFlags: down}}
	release := input{Type: inputMouse, Mi: mouseInput{DwFlags: up}}
	if err := sendInput(click, release); err != nil {
		return err
	}
	if !double {
		return nil
	}
	time.Sleep(30 * time.Millisecond)
	return sendInput(click, release)
}

// MoveMouse 把鼠标移动到虚拟桌面坐标。
func MoveMouse(x, y int) error { return moveMouse(x, y) }

// ScrollWheel 在当前位置滚动：delta 正值向上/向右，负值向下/向左，
// 单位为滚轮格数；horizontal 为 true 时横向滚动。x/y >= 0 时先移动鼠标。
func ScrollWheel(delta int, x, y int, horizontal bool) error {
	if x >= 0 && y >= 0 {
		if err := moveMouse(x, y); err != nil {
			return err
		}
	}
	flag := uint32(mouseEventWheel)
	if horizontal {
		flag = mouseEventHWheel
	}
	// 分多次注入，单事件 mouseData 按 UINT 回绕，过大的 delta 一次注入不可靠
	const chunk = 10
	remaining := delta
	step := func(n int) error {
		return sendInput(input{Type: inputMouse, Mi: mouseInput{
			MouseData: uint32(int32(n * wheelDelta)),
			DwFlags:   flag,
		}})
	}
	for remaining != 0 {
		n := remaining
		if n > chunk {
			n = chunk
		} else if n < -chunk {
			n = -chunk
		}
		if err := step(n); err != nil {
			return err
		}
		remaining -= n
		if remaining != 0 {
			time.Sleep(20 * time.Millisecond)
		}
	}
	return nil
}

// typeRune 以 Unicode 事件注入单个字符：目标应用按当前键盘布局之外也能收到
// 文本（中文、表情等不依赖布局）；增补平面字符拆成 UTF-16 代理对逐个注入。
func typeRune(r rune) error {
	feed := func(c rune, up bool) error {
		ki := keyboardInput{WScan: uint16(c), DwFlags: keyEventUnicode}
		if up {
			ki.DwFlags |= keyEventKeyUp
		}
		in := input{Type: inputKeyboard}
		*(*keyboardInput)(unsafe.Pointer(&in.Mi)) = ki
		return sendInput(in)
	}
	runes := []rune{r}
	if r > 0xFFFF {
		// 增补平面字符：拆成 UTF-16 代理对逐个注入
		hi, lo := utf16.EncodeRune(r)
		runes = []rune{hi, lo}
	}
	for _, c := range runes {
		if err := feed(c, false); err != nil {
			return err
		}
		if err := feed(c, true); err != nil {
			return err
		}
	}
	return nil
}

// tapVK 敲一下虚拟键（按下并抬起），键间留 10ms 给目标应用处理。
func tapVK(vk uint16) error {
	down := input{Type: inputKeyboard}
	*(*keyboardInput)(unsafe.Pointer(&down.Mi)) = keyboardInput{WVk: vk}
	up := input{Type: inputKeyboard}
	*(*keyboardInput)(unsafe.Pointer(&up.Mi)) = keyboardInput{WVk: vk, DwFlags: keyEventKeyUp}
	if err := sendInput(down, up); err != nil {
		return err
	}
	time.Sleep(10 * time.Millisecond)
	return nil
}

// TypeText 输入文本：换行转为回车键、制表符转为 Tab 键（比 Unicode 控制字符
// 兼容性好），其余字符走 Unicode 事件；每字符间留 2ms。文本过长时耗时随之增长。
func TypeText(text string) error {
	for _, r := range text {
		var err error
		switch r {
		case '\n':
			err = tapVK(0x0D) // VK_RETURN
		case '\t':
			err = tapVK(0x09) // VK_TAB
		case '\r':
			continue
		default:
			err = typeRune(r)
		}
		if err != nil {
			return err
		}
		time.Sleep(2 * time.Millisecond)
	}
	return nil
}

// PressKeys 敲击组合键（如 "ctrl+shift+t"）：修饰键按住 → 主键敲击 → 修饰键
// 逆序释放，整段一次 SendInput 注入；times 为连按次数（已由调用方限幅）。
func PressKeys(combo string, times int) error {
	hold, tap, err := ParseKeyCombo(combo)
	if err != nil {
		return err
	}
	if times < 1 {
		times = 1
	}
	for i := 0; i < times; i++ {
		events := make([]input, 0, len(hold)*2+2)
		add := func(vk uint16, up bool) {
			ki := keyboardInput{WVk: vk}
			if up {
				ki.DwFlags = keyEventKeyUp
			}
			in := input{Type: inputKeyboard}
			*(*keyboardInput)(unsafe.Pointer(&in.Mi)) = ki
			events = append(events, in)
		}
		for _, vk := range hold {
			add(vk, false)
		}
		add(tap, false)
		add(tap, true)
		for j := len(hold) - 1; j >= 0; j-- {
			add(hold[j], true)
		}
		if err := sendInput(events...); err != nil {
			return err
		}
		if i < times-1 {
			time.Sleep(50 * time.Millisecond)
		}
	}
	return nil
}
