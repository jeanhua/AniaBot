//go:build windows

package computeruse

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"sync"
	"unsafe"

	"golang.org/x/image/draw"
	"golang.org/x/sys/windows"
)

var (
	user32 = windows.NewLazySystemDLL("user32.dll")
	gdi32  = windows.NewLazySystemDLL("gdi32.dll")

	procGetSystemMetrics          = user32.NewProc("GetSystemMetrics")
	procGetDC                     = user32.NewProc("GetDC")
	procReleaseDC                 = user32.NewProc("ReleaseDC")
	procSetProcessDPIAware        = user32.NewProc("SetProcessDPIAware")
	procSetProcessDpiAwarenessCtx = user32.NewProc("SetProcessDpiAwarenessContext")
	procCreateCompatibleDC        = gdi32.NewProc("CreateCompatibleDC")
	procDeleteDC                  = gdi32.NewProc("DeleteDC")
	procCreateDIBSection          = gdi32.NewProc("CreateDIBSection")
	procDeleteObject              = gdi32.NewProc("DeleteObject")
	procSelectObject              = gdi32.NewProc("SelectObject")
	procBitBlt                    = gdi32.NewProc("BitBlt")
)

// GetSystemMetrics 索引（虚拟桌面：所有显示器拼成的大桌面）
const (
	smXVirtualScreen  = 76
	smYVirtualScreen  = 77
	smCXVirtualScreen = 78
	smCYVirtualScreen = 79

	srccopy = 0x00CC0020
)

// dpiOnce 保证进程 DPI 感知设置只执行一次；不感知 DPI 时 GetSystemMetrics
// 返回的是被系统虚拟化过的缩放值，截图会模糊且坐标错位。
var dpiOnce sync.Once

func ensureDPIAware() {
	dpiOnce.Do(func() {
		// DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 = -4（Win10 1607+），
		// 老系统回退 SetProcessDPIAware；均失败时静默继续（结果可能被缩放）
		const perMonitorV2 = ^uintptr(3) // (DPI_AWARENESS_CONTEXT)-4 的补码表示
		if procSetProcessDpiAwarenessCtx.Find() == nil {
			procSetProcessDpiAwarenessCtx.Call(perMonitorV2)
			return
		}
		procSetProcessDPIAware.Call()
	})
}

// Bounds 一块屏幕区域（虚拟桌面坐标系，原点可能在负数——多显示器场景）
type Bounds struct {
	X, Y, W, H int
}

// VirtualScreenBounds 返回虚拟桌面（全部显示器）的范围。
func VirtualScreenBounds() (Bounds, error) {
	ensureDPIAware()
	x, _, _ := procGetSystemMetrics.Call(smXVirtualScreen)
	y, _, _ := procGetSystemMetrics.Call(smYVirtualScreen)
	w, _, _ := procGetSystemMetrics.Call(smCXVirtualScreen)
	h, _, _ := procGetSystemMetrics.Call(smCYVirtualScreen)
	b := Bounds{X: int(int32(x)), Y: int(int32(y)), W: int(int32(w)), H: int(int32(h))}
	if b.W <= 0 || b.H <= 0 {
		return b, fmt.Errorf("获取虚拟桌面尺寸失败: %+v", b)
	}
	return b, nil
}

// bitmapinfoheader 与 bitmapinfo 对应 Win32 的 BITMAPINFOHEADER / BITMAPINFO。
// 负 BiHeight 表示自上而下的行序（与 Go image 一致，免去翻转）。
type bitmapinfoheader struct {
	BiSize          uint32
	BiWidth         int32
	BiHeight        int32
	BiPlanes        uint16
	BiBitCount      uint16
	BiCompression   uint32
	BiSizeImage     uint32
	BiXPelsPerMeter int32
	BiYPelsPerMeter int32
	BiClrUsed       uint32
	BiClrImportant  uint32
}

type bitmapinfo struct {
	BmiHeader bitmapinfoheader
	BmiColors uint32
}

// Capture 一次屏幕截图的产物：PNG 编码图像 + 还原坐标映射所需的全部参数。
type Capture struct {
	PNG              []byte
	OriginX, OriginY int // 截取区域左上角的屏幕坐标
	ScreenW, ScreenH int // 实际截取的屏幕区域尺寸
	ImgW, ImgH       int // 输出图像尺寸（可能因降采样小于 ScreenW/H）
}

// CaptureScreen 截取 rect 指定的屏幕区域（nil 或零值表示整个虚拟桌面），
// 宽度超过 maxWidth 时按比例降采样（0 表示不限制），返回 PNG 编码结果。
func CaptureScreen(rect *Bounds, maxWidth int) (*Capture, error) {
	desktop, err := VirtualScreenBounds()
	if err != nil {
		return nil, err
	}
	region := desktop
	if rect != nil && rect.W > 0 && rect.H > 0 {
		region = clampRect(*rect, desktop)
	}

	hdcScreen, _, err := procGetDC.Call(0)
	if hdcScreen == 0 {
		return nil, fmt.Errorf("GetDC 失败: %v", err)
	}
	defer procReleaseDC.Call(0, hdcScreen)

	hdcMem, _, err := procCreateCompatibleDC.Call(hdcScreen)
	if hdcMem == 0 {
		return nil, fmt.Errorf("CreateCompatibleDC 失败: %v", err)
	}
	defer procDeleteDC.Call(hdcMem)

	bmi := bitmapinfo{}
	bmi.BmiHeader.BiSize = uint32(unsafe.Sizeof(bmi.BmiHeader))
	bmi.BmiHeader.BiWidth = int32(region.W)
	bmi.BmiHeader.BiHeight = -int32(region.H)
	bmi.BmiHeader.BiPlanes = 1
	bmi.BmiHeader.BiBitCount = 32
	bmi.BmiHeader.BiCompression = 0 // BI_RGB

	var bits unsafe.Pointer
	hDib, _, err := procCreateDIBSection.Call(
		hdcMem, uintptr(unsafe.Pointer(&bmi)), 0, /*DIB_RGB_COLORS*/
		uintptr(unsafe.Pointer(&bits)), 0, 0)
	if hDib == 0 || bits == nil {
		return nil, fmt.Errorf("CreateDIBSection 失败: %v", err)
	}
	defer procDeleteObject.Call(hDib)

	prev, _, _ := procSelectObject.Call(hdcMem, hDib)
	defer procSelectObject.Call(hdcMem, prev)

	if r, _, err := procBitBlt.Call(hdcMem, 0, 0, uintptr(region.W), uintptr(region.H),
		hdcScreen, uintptr(int32(region.X)), uintptr(int32(region.Y)), srccopy); r == 0 {
		return nil, fmt.Errorf("BitBlt 失败: %v", err)
	}

	// DIB 内存布局为 BGRX（小端 32bpp），逐像素换到 RGBA 并补不透明 alpha
	rowBytes := region.W * 4
	raw := unsafe.Slice((*byte)(bits), rowBytes*region.H)

	img := image.NewRGBA(image.Rect(0, 0, region.W, region.H))
	for i, j := 0, 0; i < len(raw); i, j = i+4, j+4 {
		img.Pix[j+0] = raw[i+2] // R ← BGRX 的 X[2]
		img.Pix[j+1] = raw[i+1] // G
		img.Pix[j+2] = raw[i+0] // B
		img.Pix[j+3] = 0xFF
	}

	outW, outH := region.W, region.H
	if newW, newH, ok := Downscaled(region.W, region.H, maxWidth); ok {
		scaled := image.NewRGBA(image.Rect(0, 0, newW, newH))
		draw.CatmullRom.Scale(scaled, scaled.Bounds(), img, img.Bounds(), draw.Over, nil)
		img = scaled
		outW, outH = newW, newH
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("PNG 编码失败: %w", err)
	}
	return &Capture{
		PNG:     buf.Bytes(),
		OriginX: region.X, OriginY: region.Y,
		ScreenW: region.W, ScreenH: region.H,
		ImgW: outW, ImgH: outH,
	}, nil
}

// clampRect 把任意矩形夹取到桌面范围内，得到合法的截取区域。
func clampRect(rect, desktop Bounds) Bounds {
	x0, y0 := max(rect.X, desktop.X), max(rect.Y, desktop.Y)
	x1, y1 := min(rect.X+rect.W, desktop.X+desktop.W), min(rect.Y+rect.H, desktop.Y+desktop.H)
	if x1 <= x0 || y1 <= y0 {
		return desktop
	}
	return Bounds{X: x0, Y: y0, W: x1 - x0, H: y1 - y0}
}
