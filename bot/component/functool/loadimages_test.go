package functool

import (
	"context"
	"testing"

	"github.com/jeanhua/AniaBot/bot/component/llmtool"
)

func TestLoadImagesTool(t *testing.T) {
	tool := NewLoadImagesTool()
	var got []string
	callbacks := llmtool.CallBackFuncs{
		LoadImages: func(hashes []string) (string, error) {
			got = hashes
			return "已加载 2 张图片", nil
		},
	}

	want := []string{"a1b2c3d4", "e5f6a7b8"}
	result, err := tool.Execute(context.Background(), &LoadImagesParams{Hashes: want}, callbacks)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("LoadImages callback got hashes = %v, want %v", got, want)
	}
	if result != "已加载 2 张图片" {
		t.Fatalf("Execute() result = %q", result)
	}
}

func TestLoadImagesToolWithoutCallback(t *testing.T) {
	tool := NewLoadImagesTool()
	result, err := tool.Execute(context.Background(), &LoadImagesParams{Hashes: []string{"a1b2c3d4"}}, llmtool.CallBackFuncs{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result != "当前会话不支持加载图片" {
		t.Fatalf("Execute() result = %q", result)
	}
}
