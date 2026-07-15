package functool

import (
	"context"
	"testing"

	"github.com/jeanhua/AniaBot/bot/component/llmtool"
)

func TestLoadImagesTool(t *testing.T) {
	tool := NewLoadImagesTool()
	called := false
	callbacks := llmtool.CallBackFuncs{
		LoadImages: func() (string, error) {
			called = true
			return "已加载 2 张图片", nil
		},
	}

	result, err := tool.Execute(context.Background(), &LoadImagesParams{}, callbacks)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !called {
		t.Fatal("LoadImages callback was not called")
	}
	if result != "已加载 2 张图片" {
		t.Fatalf("Execute() result = %q", result)
	}
}

func TestLoadImagesToolWithoutCallback(t *testing.T) {
	tool := NewLoadImagesTool()
	result, err := tool.Execute(context.Background(), &LoadImagesParams{}, llmtool.CallBackFuncs{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result != "当前会话不支持加载图片" {
		t.Fatalf("Execute() result = %q", result)
	}
}
