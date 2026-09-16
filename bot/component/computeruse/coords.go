package computeruse

import "math"

// View 描述「截图图像坐标 → 屏幕坐标」的映射关系。
//
// AI 看到的是截图图像，它给出的点击/滚动坐标是图像像素坐标；
// 底层输入需要屏幕（虚拟桌面）坐标。为省上下文，截图可能被降采样，
// 区域截图时图像原点也可能偏移，故用「原点 + 缩放」二元组描述映射：
//
//	screenX = OriginX + imageX * ScaleX
//	spanY 同理。
//
// 未截图时使用单位映射（Origin=0, Scale=1），此时 AI 坐标即屏幕坐标。
type View struct {
	OriginX, OriginY float64 // 截图左上角在虚拟桌面坐标系中的位置
	ScaleX, ScaleY   float64 // 屏幕像素 / 图像像素（>=1 表示图像被缩小过）
}

// UnitView 返回单位映射（图像坐标 = 屏幕坐标）。
func UnitView() View {
	return View{OriginX: 0, OriginY: 0, ScaleX: 1, ScaleY: 1}
}

// ComputeView 根据截图参数计算映射：screenW/H 为本次截取的屏幕区域尺寸，
// imgW/H 为实际产出图像尺寸（可能因降采样变小），origin 为截取区域左上角的
// 屏幕坐标。imgW/H 与 screenW/H 一致时缩放为 1。
func ComputeView(originX, originY, screenW, screenH, imgW, imgH int) View {
	v := View{OriginX: float64(originX), OriginY: float64(originY)}
	if screenW > 0 && imgW > 0 {
		v.ScaleX = float64(screenW) / float64(imgW)
	}
	if screenH > 0 && imgH > 0 {
		v.ScaleY = float64(screenH) / float64(imgH)
	}
	return v
}

// ToScreen 把图像坐标换算为屏幕（虚拟桌面）坐标。不做范围夹取：
// 虚拟桌面原点可能为负（多显示器），最终注入前的夹取由
// VirtualToAbsolute（SendInput 归一化）与 RegionIn（区域截取）负责。
func (v View) ToScreen(imageX, imageY int) (int, int) {
	x := int(math.Round(v.OriginX + float64(imageX)*v.ScaleX))
	y := int(math.Round(v.OriginY + float64(imageY)*v.ScaleY))
	return x, y
}

// RegionIn 把「在当前视图坐标系中指定的矩形」换算为屏幕坐标矩形，
// 并夹取到 desktop 桌面范围内，宽高至少为 1。供区域截图使用：
// AI 想放大看某个区域时，直接用它上一张图里的坐标指定矩形。
func (v View) RegionIn(desktop Bounds, imageX, imageY, imageW, imageH int) Bounds {
	x0, y0 := v.ToScreen(imageX, imageY)
	x1, y1 := v.ToScreen(imageX+imageW, imageY+imageH)
	if x1 < x0 {
		x0, x1 = x1, x0
	}
	if y1 < y0 {
		y0, y1 = y1, y0
	}
	if x0 < desktop.X {
		x0 = desktop.X
	}
	if y0 < desktop.Y {
		y0 = desktop.Y
	}
	if x1 > desktop.X+desktop.W {
		x1 = desktop.X + desktop.W
	}
	if y1 > desktop.Y+desktop.H {
		y1 = desktop.Y + desktop.H
	}
	w, h := x1-x0, y1-y0
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	return Bounds{X: x0, Y: y0, W: w, H: h}
}

// VirtualToAbsolute 把虚拟桌面像素坐标换算为 SendInput（MOUSEEVENTF_ABSOLUTE |
// MOUSEEVENTF_VIRTUALDESK）的 0~65535 归一化坐标，并把越界坐标夹取回桌面。
func VirtualToAbsolute(b Bounds, x, y int) (int32, int32) {
	norm := func(pos, origin, size int) int32 {
		if size <= 1 {
			return 0
		}
		v := float64(pos-origin) * 65535.0 / float64(size-1)
		if v < 0 {
			v = 0
		}
		if v > 65535 {
			v = 65535
		}
		return int32(v + 0.5)
	}
	return norm(x, b.X, b.W), norm(y, b.Y, b.H)
}

// Downscaled 按最大宽度 maxW 计算降采样后的图像尺寸（保持宽高比）；
// 不需要缩放时 ok 为 false。
func Downscaled(w, h, maxW int) (newW, newH int, ok bool) {
	if maxW <= 0 || w <= maxW || w <= 0 || h <= 0 {
		return w, h, false
	}
	newW = maxW
	newH = int(math.Round(float64(h) * float64(maxW) / float64(w)))
	if newH < 1 {
		newH = 1
	}
	return newW, newH, true
}
