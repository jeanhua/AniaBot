package functool

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/jeanhua/AniaBot/bot/component/computeruse"
	"github.com/jeanhua/AniaBot/bot/component/llmtool"
	"github.com/jeanhua/AniaBot/bot/component/oplog"
)

// ComputerUseConfig 电脑操作（computer use）工具配置。
// 启用与否由调用方控制（配置开关 + computeruse.Available()，默认关闭）：
// 启用后 AI 可截图查看宿主机屏幕并控制鼠标键盘，等于把宿主机桌面交给
// AI 操作，必须管理员知情开启（opt-in）。
type ComputerUseConfig struct {
	// MaxWidth 截图最大宽度（像素），超出时等比降采样以节省上下文 token；
	// 0 表示不缩放（高分屏下慎用，图片体积和 token 消耗都会很大）
	MaxWidth int `json:"max_width" mapstructure:"max_width"`
}

// computerUseState 一组电脑操作工具共享的状态：坐标映射视图 + 截图宽度上限。
// AI 的操作坐标基于「最近一次 screenshot 的图像」，视图由截图工具更新、
// 输入工具换算，用一个互斥锁串行化（同一宿主机上的操作本来就该串行）。
// 注意：state 注册在基础 ToolExecuter 上、跨会话（群/私聊）共享，互斥锁只保证
// 单次视图读写的原子性——若两个会话并发「截图→点击」，后者会按前者的最新视图
// 换算。单宿主机 + 默认关闭场景下可接受，属已知限制。
type computerUseState struct {
	mu       sync.Mutex
	view     computeruse.View
	maxWidth int
}

func (s *computerUseState) setView(v computeruse.View) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.view = v
}

func (s *computerUseState) currentView() computeruse.View {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.view
}

// toScreen 把 AI 给出的图像坐标换算为屏幕坐标。
func (s *computerUseState) toScreen(x, y int) (int, int) {
	return s.currentView().ToScreen(x, y)
}

// ─────────────────────────────────────────────
// screenshot 截图工具
// ─────────────────────────────────────────────

// ScreenshotParams screenshot 工具参数：区域坐标基于最近一次截图的图像空间，
// 首次截图（无参照图）时为屏幕坐标；全部可选，缺省截取整个桌面。
type ScreenshotParams struct {
	X      *int `json:"x,omitempty" desc:"可选，截取区域左上角横坐标（最近一次截图图像中的坐标；未截图时为屏幕坐标）"`
	Y      *int `json:"y,omitempty" desc:"可选，截取区域左上角纵坐标（同上）"`
	Width  *int `json:"width,omitempty" desc:"可选，截取区域宽度（像素）"`
	Height *int `json:"height,omitempty" desc:"可选，截取区域高度（像素）"`
}

type ScreenshotTool struct {
	llmtool.BaseTool[ScreenshotParams]
	state *computerUseState
}

func NewScreenshotTool(state *computerUseState) *ScreenshotTool {
	return &ScreenshotTool{
		BaseTool: llmtool.MakeBaseTool(
			"screenshot",
			"截取宿主机屏幕（整个桌面或指定区域）供查看。区域坐标基于最近一次截图的图像空间（未截图时为屏幕坐标），可用于放大查看小字/小按钮。屏幕内容会在下一轮上下文中以图片形式提供，请直接查看图片后回答或继续操作；需要精确点击时先截图再按图中坐标给 mouse_click。",
			ScreenshotParams{},
		),
		state: state,
	}
}

func (t *ScreenshotTool) Execute(_ context.Context, params any, callbacks llmtool.CallBackFuncs) (string, error) {
	p := params.(*ScreenshotParams)

	given := 0
	for _, v := range []*int{p.X, p.Y, p.Width, p.Height} {
		if v != nil {
			given++
		}
	}
	if given > 0 && given < 4 {
		return "", fmt.Errorf("screenshot: 区域截图需同时给出 x/y/width/height（不填则截取整个桌面）")
	}
	var rect *computeruse.Bounds
	if given == 4 {
		desktop, err := computeruse.VirtualScreenBounds()
		if err != nil {
			return "", err
		}
		region := t.state.currentView().RegionIn(desktop, *p.X, *p.Y, *p.Width, *p.Height)
		rect = &region
	}

	capture, err := computeruse.CaptureScreen(rect, t.state.maxWidth)
	if err != nil {
		return "", fmt.Errorf("截图失败: %w", err)
	}

	newView := computeruse.ComputeView(
		capture.OriginX, capture.OriginY,
		capture.ScreenW, capture.ScreenH,
		capture.ImgW, capture.ImgH)

	oplog.Record(oplog.CategoryAI, "screenshot", fmt.Sprintf("AI 截取屏幕区域 (%d,%d) %dx%d → 图像 %dx%d",
		capture.OriginX, capture.OriginY, capture.ScreenW, capture.ScreenH, capture.ImgW, capture.ImgH))

	viewInfo := fmt.Sprintf("【截图信息】截取范围：屏幕坐标 (%d,%d) 起 %dx%d 像素，输出图像 %dx%d。坐标空间已切换为本图：后续 mouse_click/mouse_move/mouse_scroll/区域截图的坐标请直接给本图中的像素位置（系统自动换算到屏幕）。",
		capture.OriginX, capture.OriginY, capture.ScreenW, capture.ScreenH, capture.ImgW, capture.ImgH)

	if callbacks.LoadLocalImage == nil {
		return viewInfo + "\n截图已生成，但当前会话不支持查看图片内容", nil
	}

	// 经临时文件复用 local_image 的回传管线（多模态入队 / OCR 备用识别），
	// 回调同步读取完成后即删除
	tmp, err := os.CreateTemp("", "aniabot-screen-*.png")
	if err != nil {
		return "", fmt.Errorf("创建截图临时文件失败: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(capture.PNG); err != nil {
		tmp.Close()
		return "", fmt.Errorf("写入截图临时文件失败: %w", err)
	}
	tmp.Close()

	result, err := callbacks.LoadLocalImage(tmpName)
	if err != nil {
		return "", fmt.Errorf("加载截图失败: %w", err)
	}
	// 图像已成功交付（入队/识别完成）才切换坐标映射：交付失败时 AI 看不到
	// 本图，若已切换，它基于旧图给出的操作坐标会被错误换算
	t.state.setView(newView)
	return result + "\n" + viewInfo, nil
}

// ─────────────────────────────────────────────
// 鼠标工具
// ─────────────────────────────────────────────

type MouseClickParams struct {
	X      int    `json:"x" desc:"点击位置的横坐标（最近一次截图图像中的像素坐标；未截图时为屏幕坐标）"`
	Y      int    `json:"y" desc:"点击位置的纵坐标（同上）"`
	Button string `json:"button,omitempty" desc:"可选，鼠标键 left/right/middle，默认 left"`
	Double bool   `json:"double,omitempty" desc:"可选，是否双击，默认单击"`
}

type MouseClickTool struct {
	llmtool.BaseTool[MouseClickParams]
	state *computerUseState
}

func NewMouseClickTool(state *computerUseState) *MouseClickTool {
	return &MouseClickTool{
		BaseTool: llmtool.MakeBaseTool(
			"mouse_click",
			"在宿主机屏幕指定位置点击鼠标（左键/右键/中键，可双击）。坐标基于最近一次 screenshot 的图像；操作前应先截图确认目标位置，操作后可再次截图确认效果。该操作会真实控制宿主机鼠标，请谨慎执行。",
			MouseClickParams{},
		),
		state: state,
	}
}

func (t *MouseClickTool) Execute(_ context.Context, params any, _ llmtool.CallBackFuncs) (string, error) {
	p := params.(*MouseClickParams)
	button := p.Button
	if button == "" {
		button = "left"
	}
	sx, sy := t.state.toScreen(p.X, p.Y)
	if err := computeruse.ClickMouseButton(button, sx, sy, p.Double); err != nil {
		return "", fmt.Errorf("鼠标点击失败: %w", err)
	}
	oplog.Record(oplog.CategoryAI, "mouse_click", fmt.Sprintf("AI %s鼠标 (%d,%d)（屏幕坐标）",
		mouseActionName(button, p.Double), sx, sy))
	action := "点击"
	if p.Double {
		action = "双击"
	}
	return fmt.Sprintf("已%s%s（屏幕坐标 %d,%d）", action, button, sx, sy), nil
}

func mouseActionName(button string, double bool) string {
	action := "单击"
	if double {
		action = "双击"
	}
	return action + button
}

type MouseMoveParams struct {
	X int `json:"x" desc:"目标位置横坐标（最近一次截图图像中的像素坐标；未截图时为屏幕坐标）"`
	Y int `json:"y" desc:"目标位置纵坐标（同上）"`
}

type MouseMoveTool struct {
	llmtool.BaseTool[MouseMoveParams]
	state *computerUseState
}

func NewMouseMoveTool(state *computerUseState) *MouseMoveTool {
	return &MouseMoveTool{
		BaseTool: llmtool.MakeBaseTool(
			"mousemove",
			"把宿主机鼠标移动到指定位置（不点击），用于悬停显示提示等场景。坐标基于最近一次 screenshot 的图像。",
			MouseMoveParams{},
		),
		state: state,
	}
}

func (t *MouseMoveTool) Execute(_ context.Context, params any, _ llmtool.CallBackFuncs) (string, error) {
	p := params.(*MouseMoveParams)
	sx, sy := t.state.toScreen(p.X, p.Y)
	if err := computeruse.MoveMouse(sx, sy); err != nil {
		return "", fmt.Errorf("移动鼠标失败: %w", err)
	}
	oplog.Record(oplog.CategoryAI, "mousemove", fmt.Sprintf("AI 移动鼠标到 (%d,%d)（屏幕坐标）", sx, sy))
	return fmt.Sprintf("鼠标已移动到屏幕坐标 (%d,%d)", sx, sy), nil
}

type MouseScrollParams struct {
	Delta      int  `json:"delta" desc:"滚动格数，正数向上滚、负数向下滚"`
	X          *int `json:"x,omitempty" desc:"可选，滚动前先把鼠标移动到该位置（横坐标，最近一次截图图像坐标）"`
	Y          *int `json:"y,omitempty" desc:"可选，滚动前先把鼠标移动到该位置（纵坐标）"`
	Horizontal bool `json:"horizontal,omitempty" desc:"可选，true 时横向滚动（正数向右），默认纵向"`
}

type MouseScrollTool struct {
	llmtool.BaseTool[MouseScrollParams]
	state *computerUseState
}

func NewMouseScrollTool(state *computerUseState) *MouseScrollTool {
	return &MouseScrollTool{
		BaseTool: llmtool.MakeBaseTool(
			"mouse_scroll",
			"在宿主机屏幕上滚动鼠标滚轮，用于翻页、滚动列表等。坐标基于最近一次 screenshot 的图像；不填坐标时在当前鼠标位置滚动。",
			MouseScrollParams{},
		),
		state: state,
	}
}

func (t *MouseScrollTool) Execute(_ context.Context, params any, _ llmtool.CallBackFuncs) (string, error) {
	p := params.(*MouseScrollParams)
	if p.Delta == 0 {
		return "", fmt.Errorf("mouse_scroll: delta 不能为 0")
	}
	var x, y = -1, -1
	if p.X != nil && p.Y != nil {
		x, y = t.state.toScreen(*p.X, *p.Y)
	}
	if err := computeruse.ScrollWheel(p.Delta, x, y, p.Horizontal); err != nil {
		return "", fmt.Errorf("滚轮滚动失败: %w", err)
	}
	oplog.Record(oplog.CategoryAI, "mouse_scroll", fmt.Sprintf("AI 滚动滚轮 delta=%d（横向=%v）@(%d,%d)", p.Delta, p.Horizontal, x, y))
	direction := "向上"
	if p.Delta < 0 {
		direction = "向下"
	}
	if p.Horizontal {
		direction = "向右"
		if p.Delta < 0 {
			direction = "向左"
		}
	}
	return fmt.Sprintf("已%s滚动 %d 格", direction, absInt(p.Delta)), nil
}

func absInt(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// ─────────────────────────────────────────────
// 键盘工具
// ─────────────────────────────────────────────

type KeyboardTypeParams struct {
	Text string `json:"text" desc:"要输入的文本内容（支持中文等任意字符，换行会转成回车键）；单次上限 5000 字符，超长请拆分多次输入"`
}

type KeyboardTypeTool struct {
	llmtool.BaseTool[KeyboardTypeParams]
}

// maxTypeLen keyboard_type 单次输入的文本长度上限（rune 数）：逐字符注入约
// 2ms/字符，超长文本耗时线性增长，会逼近消息事件超时预算。
const maxTypeLen = 5000

// validateTypeText 键入文本校验（空文本与超长文本拒绝）。独立成函数以便
// 跨平台测试：走 Execute 会在 Windows 宿主机上真的逐字输入。
func validateTypeText(text string) error {
	if text == "" {
		return fmt.Errorf("keyboard_type: text 不能为空")
	}
	if n := len([]rune(text)); n > maxTypeLen {
		return fmt.Errorf("keyboard_type: 文本过长（%d 字符，上限 %d），请拆分多次输入", n, maxTypeLen)
	}
	return nil
}

func NewKeyboardTypeTool() *KeyboardTypeTool {
	return &KeyboardTypeTool{
		BaseTool: llmtool.MakeBaseTool(
			"keyboard_type",
			"向宿主机当前焦点窗口输入文本（模拟键盘逐字输入，支持中文）。输入前确保目标输入框已获得焦点（必要时先用 mouse_click 点击输入框）。该操作会真实控制宿主机键盘，请谨慎执行。",
			KeyboardTypeParams{},
		),
	}
}

func (t *KeyboardTypeTool) Execute(_ context.Context, params any, _ llmtool.CallBackFuncs) (string, error) {
	p := params.(*KeyboardTypeParams)
	if err := validateTypeText(p.Text); err != nil {
		return "", err
	}
	if err := computeruse.TypeText(p.Text); err != nil {
		return "", fmt.Errorf("键盘输入失败: %w", err)
	}
	summary := p.Text
	if r := []rune(summary); len(r) > 50 {
		summary = string(r[:50]) + "…"
	}
	oplog.Record(oplog.CategoryAI, "keyboard_type", fmt.Sprintf("AI 键盘输入 %q（%d 字符）", summary, len([]rune(p.Text))))
	return fmt.Sprintf("已输入文本（%d 字符）", len([]rune(p.Text))), nil
}

type KeyboardPressParams struct {
	Keys  string `json:"keys" desc:"按键或组合键，如 enter、ctrl+c、ctrl+shift+t；支持：ctrl/alt/shift/win 修饰键、字母数字、f1~f16、方向键、esc/enter/tab/space/backspace/delete/home/end/pageup/pagedown 等"`
	Times int    `json:"times,omitempty" desc:"可选，连按次数，默认 1，最大 20"`
}

type KeyboardPressTool struct {
	llmtool.BaseTool[KeyboardPressParams]
}

func NewKeyboardPressTool() *KeyboardPressTool {
	return &KeyboardPressTool{
		BaseTool: llmtool.MakeBaseTool(
			"keyboard_press",
			"向宿主机当前焦点窗口发送按键/组合键（如 enter、ctrl+c、alt+f4、win+d）。发送前确保目标窗口已获得焦点。该操作会真实控制宿主机键盘，请谨慎执行。",
			KeyboardPressParams{},
		),
	}
}

func (t *KeyboardPressTool) Execute(_ context.Context, params any, _ llmtool.CallBackFuncs) (string, error) {
	p := params.(*KeyboardPressParams)
	times := p.Times
	if times < 1 {
		times = 1
	}
	if times > 20 {
		times = 20
	}
	if err := computeruse.PressKeys(p.Keys, times); err != nil {
		return "", fmt.Errorf("按键失败: %w", err)
	}
	oplog.Record(oplog.CategoryAI, "keyboard_press", fmt.Sprintf("AI 按键 %q x%d", p.Keys, times))
	if times > 1 {
		return fmt.Sprintf("已按下 %s x%d", p.Keys, times), nil
	}
	return fmt.Sprintf("已按下 %s", p.Keys), nil
}

// ─────────────────────────────────────────────
// 窗口信息工具（只读）
// ─────────────────────────────────────────────

type ActiveWindowParams struct{}

type ActiveWindowTool struct {
	llmtool.BaseTool[ActiveWindowParams]
}

func NewActiveWindowTool() *ActiveWindowTool {
	return &ActiveWindowTool{
		BaseTool: llmtool.MakeBaseTool(
			"active_window",
			"查看宿主机当前前台窗口的标题、类名和位置尺寸，用于确认键盘/鼠标操作的目标窗口是否正确。",
			ActiveWindowParams{},
		),
	}
}

func (t *ActiveWindowTool) Execute(_ context.Context, _ any, _ llmtool.CallBackFuncs) (string, error) {
	info, err := computeruse.ForegroundWindow()
	if err != nil {
		return "", err
	}
	if info.Title == "" {
		return "当前前台窗口无标题", nil
	}
	return fmt.Sprintf("前台窗口：%s（类名 %s，位置 %d,%d，大小 %dx%d）",
		info.Title, info.Class, info.X, info.Y, info.W, info.H), nil
}

type ListWindowsParams struct {
	Limit int `json:"limit,omitempty" desc:"可选，最多返回的窗口数量，默认 50"`
}

type ListWindowsTool struct {
	llmtool.BaseTool[ListWindowsParams]
}

func NewListWindowsTool() *ListWindowsTool {
	return &ListWindowsTool{
		BaseTool: llmtool.MakeBaseTool(
			"list_windows",
			"列出宿主机上所有可见的顶层窗口（标题、类名、位置、大小），用于找到要操作的应用窗口。",
			ListWindowsParams{},
		),
	}
}

func (t *ListWindowsTool) Execute(_ context.Context, params any, _ llmtool.CallBackFuncs) (string, error) {
	p := params.(*ListWindowsParams)
	limit := p.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	list, err := computeruse.ListWindows(limit)
	if err != nil {
		return "", err
	}
	if len(list) == 0 {
		return "没有找到可见的顶层窗口", nil
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "共 %d 个可见窗口：", len(list))
	for _, w := range list {
		fmt.Fprintf(&sb, "\n- %s（类名 %s，%dx%d @ %d,%d）", w.Title, w.Class, w.W, w.H, w.X, w.Y)
	}
	return sb.String(), nil
}

// ─────────────────────────────────────────────
// 注册入口
// ─────────────────────────────────────────────

// NewComputerUseTools 创建全部电脑操作工具（8 个：screenshot / mouse_click /
// mousemove / mouse_scroll / keyboard_type / keyboard_press / active_window /
// list_windows）。调用方需先确认 computeruse.Available()，并用配置开关控制
// 是否注册（默认关闭）。
func NewComputerUseTools(cfg ComputerUseConfig) []llmtool.Tool {
	state := &computerUseState{view: computeruse.UnitView(), maxWidth: cfg.MaxWidth}
	return []llmtool.Tool{
		NewScreenshotTool(state),
		NewMouseClickTool(state),
		NewMouseMoveTool(state),
		NewMouseScrollTool(state),
		NewKeyboardTypeTool(),
		NewKeyboardPressTool(),
		NewActiveWindowTool(),
		NewListWindowsTool(),
	}
}
