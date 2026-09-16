package computeruse

import "testing"

func TestUnitView(t *testing.T) {
	v := UnitView()
	if x, y := v.ToScreen(123, 456); x != 123 || y != 456 {
		t.Fatalf("单位映射应原样透传坐标, got (%d,%d)", x, y)
	}
}

func TestComputeViewAndToScreen(t *testing.T) {
	// 屏幕 2560x1600，整屏截图降采样到 1280x800：缩放应为 2
	v := ComputeView(0, 0, 2560, 1600, 1280, 800)
	if v.ScaleX != 2 || v.ScaleY != 2 {
		t.Fatalf("缩放系数应为 2, got %v/%v", v.ScaleX, v.ScaleY)
	}
	if x, y := v.ToScreen(100, 50); x != 200 || y != 100 {
		t.Fatalf("图像 (100,50) 应映射到屏幕 (200,100), got (%d,%d)", x, y)
	}

	// 区域截图：原点偏移 + 1:1 无缩放
	v = ComputeView(1920, 1080, 500, 400, 500, 400)
	if x, y := v.ToScreen(10, 20); x != 1930 || y != 1100 {
		t.Fatalf("区域截图坐标换算错误, got (%d,%d)", x, y)
	}

	// 非法尺寸（除零保护）：应得到缩放 0 而非 panic
	v = ComputeView(0, 0, 0, 0, 100, 100)
	if x, y := v.ToScreen(5, 5); x != 0 || y != 0 {
		t.Fatalf("零尺寸截图应缩放为 0, got (%d,%d)", x, y)
	}
}

func TestToScreenRaw(t *testing.T) {
	// ToScreen 是纯换算，不做夹取（负原点桌面下负坐标合法）
	v := UnitView()
	if x, y := v.ToScreen(-5, -7); x != -5 || y != -7 {
		t.Fatalf("单位映射应原样透传（含负值）, got (%d,%d)", x, y)
	}
}

func TestVirtualToAbsoluteClamp(t *testing.T) {
	desktop := Bounds{X: 0, Y: 0, W: 1920, H: 1080}
	// 越界坐标在归一化时被夹取到 [0, 65535]
	ax, ay := VirtualToAbsolute(desktop, -100, 5000)
	if ax != 0 {
		t.Fatalf("负坐标应夹取为 0, got %d", ax)
	}
	if ay != 65535 {
		t.Fatalf("超界纵坐标应夹取为 65535, got %d", ay)
	}
	// 正常换算：像素 960 在 1920 宽桌面下的归一化值
	ax, _ = VirtualToAbsolute(desktop, 960, 540)
	if ax != 32785 { // 960 * 65535 / 1919 = 32785.17 → 32785
		t.Fatalf("像素 960 归一化结果错误, got %d", ax)
	}
	// 极小桌面防除零
	ax, ay = VirtualToAbsolute(Bounds{X: 0, Y: 0, W: 1, H: 1}, 0, 0)
	if ax != 0 || ay != 0 {
		t.Fatalf("单像素桌面应归一化为 0, got (%d,%d)", ax, ay)
	}
}

func TestRegionIn(t *testing.T) {
	desktop := Bounds{X: 0, Y: 0, W: 1920, H: 1080}
	v := UnitView()
	// 正常区域
	r := v.RegionIn(desktop, 10, 10, 100, 80)
	if r.X != 10 || r.Y != 10 || r.W != 100 || r.H != 80 {
		t.Fatalf("区域换算错误: got (%d,%d,%d,%d)", r.X, r.Y, r.W, r.H)
	}
	// 越出右边界：应被 clamp
	r = v.RegionIn(desktop, 1900, 0, 100, 50)
	if r.X != 1900 || r.W != 20 {
		t.Fatalf("越界区域应被夹取: got x=%d w=%d", r.X, r.W)
	}
	// 宽高退化到至少 1
	r = v.RegionIn(desktop, 50, 50, 0, 0)
	if r.W < 1 || r.H < 1 {
		t.Fatalf("零尺寸区域应保证至少 1 像素: got %dx%d", r.W, r.H)
	}
	// 负原点桌面（多显示器）：负坐标区域合法
	neg := Bounds{X: -1920, Y: 0, W: 3840, H: 1080}
	r = v.RegionIn(neg, -1000, 0, 500, 400)
	if r.X != -1000 || r.W != 500 {
		t.Fatalf("负原点桌面区域换算错误: got (%d,%d,%d,%d)", r.X, r.Y, r.W, r.H)
	}
}

func TestDownscaled(t *testing.T) {
	if w, h, ok := Downscaled(1000, 500, 1280); ok || w != 1000 || h != 500 {
		t.Fatalf("宽度未超限不应缩放, got %dx%d ok=%v", w, h, ok)
	}
	if w, h, ok := Downscaled(2560, 1600, 0); ok || w != 2560 || h != 1600 {
		t.Fatalf("maxWidth=0 表示不限制, got %dx%d ok=%v", w, h, ok)
	}
	w, h, ok := Downscaled(2560, 1600, 1280)
	if !ok || w != 1280 || h != 800 {
		t.Fatalf("降采样应保持宽高比, got %dx%d ok=%v", w, h, ok)
	}
}
