package functool

import (
	"context"
	"strings"
	"testing"

	"github.com/jeanhua/AniaBot/bot/component/llmtool"
)

// TestComputerUseToolNames 工具名与注册数量：接入处（插件注册/计划模式阻断名单/
// 审批配置说明）依赖这些名字，改名会静默失效。
func TestComputerUseToolNames(t *testing.T) {
	tools := NewComputerUseTools(ComputerUseConfig{MaxWidth: 1280})
	want := []string{
		"screenshot", "mouse_click", "mousemove", "mouse_scroll",
		"keyboard_type", "keyboard_press", "active_window", "list_windows",
	}
	if len(tools) != len(want) {
		t.Fatalf("工具数量应为 %d, got %d", len(want), len(tools))
	}
	names := map[string]bool{}
	for i, tool := range tools {
		if tool.Name() != want[i] {
			t.Fatalf("第 %d 个工具应为 %s, got %s", i, want[i], tool.Name())
		}
		if tool.Description() == "" {
			t.Fatalf("工具 %s 缺少描述", tool.Name())
		}
		names[tool.Name()] = true
	}
	if !names["screenshot"] || !names["mouse_click"] {
		t.Fatal("核心工具缺失")
	}
}

// TestComputerUseParamValidation 参数校验发生在系统调用之前，跨平台可测。
func TestComputerUseParamValidation(t *testing.T) {
	tools := NewComputerUseTools(ComputerUseConfig{MaxWidth: 800})
	byName := map[string]llmtool.Tool{}
	for _, tool := range tools {
		byName[tool.Name()] = tool
	}
	ctx := context.Background()
	cbs := llmtool.CallBackFuncs{}

	// keyboard_type：空文本拒绝（不触发系统调用）
	_, err := byName["keyboard_type"].Execute(ctx, &KeyboardTypeParams{}, cbs)
	if err == nil {
		t.Fatal("空文本应被拒绝")
	}

	// mouse_scroll：delta=0 拒绝
	_, err = byName["mouse_scroll"].Execute(ctx, &MouseScrollParams{}, cbs)
	if err == nil {
		t.Fatal("delta=0 应被拒绝")
	}

	// screenshot：区域参数残缺应报错而非截全屏
	x, y, w := 10, 10, 200
	_, err = byName["screenshot"].Execute(ctx,
		&ScreenshotParams{X: &x, Y: &y, Width: &w, Height: nil}, cbs)
	if err == nil || !strings.Contains(err.Error(), "同时给出") {
		t.Fatalf("残缺区域参数应报错: %v", err)
	}
}

// TestKeyboardTypeTextValidation 键入文本校验：直接测校验函数而非 Execute——
// 超长文本若经 Execute 且校验失效，会在 Windows 宿主机上真的逐字输入。
func TestKeyboardTypeTextValidation(t *testing.T) {
	if err := validateTypeText(""); err == nil {
		t.Fatal("空文本应被拒绝")
	}
	if err := validateTypeText("你好，AniaBot"); err != nil {
		t.Fatalf("正常文本不应报错: %v", err)
	}
	long := strings.Repeat("a", maxTypeLen+1)
	if err := validateTypeText(long); err == nil || !strings.Contains(err.Error(), "过长") {
		t.Fatalf("超长文本应被拒绝: %v", err)
	}
	boundary := strings.Repeat("a", maxTypeLen)
	if err := validateTypeText(boundary); err != nil {
		t.Fatalf("恰好达到上限的文本不应报错: %v", err)
	}
}

// TestComputerUseParamsReflectable 参数结构必须可被 llmtool parser 反射
// （Params() 返回结构体指针）；screenshot 的区域参数为可选（omitempty），
// schema 细节由 llmtool/parser 的既有测试覆盖。
func TestComputerUseParamsReflectable(t *testing.T) {
	for _, tool := range NewComputerUseTools(ComputerUseConfig{}) {
		if tool.Params() == nil {
			t.Fatalf("工具 %s 的 Params() 不应返回 nil", tool.Name())
		}
	}
}
