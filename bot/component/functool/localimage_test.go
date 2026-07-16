package functool

import (
	"context"
	"testing"

	"github.com/jeanhua/AniaBot/bot/component/llmtool"
)

func TestLoadLocalImageTool(t *testing.T) {
	tool := NewLoadLocalImageTool()
	called := false
	var gotPath string
	callbacks := llmtool.CallBackFuncs{
		LoadLocalImage: func(path string) (string, error) {
			called = true
			gotPath = path
			return "已加载本地图片 /tmp/cat.png", nil
		},
	}

	result, err := tool.Execute(context.Background(), &LoadLocalImageParams{Path: "/tmp/cat.png"}, callbacks)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !called {
		t.Fatal("LoadLocalImage callback was not called")
	}
	if gotPath != "/tmp/cat.png" {
		t.Fatalf("回调收到 path = %q, want /tmp/cat.png", gotPath)
	}
	if result != "已加载本地图片 /tmp/cat.png" {
		t.Fatalf("Execute() result = %q", result)
	}
}

func TestLoadLocalImageToolWithoutCallback(t *testing.T) {
	tool := NewLoadLocalImageTool()
	result, err := tool.Execute(context.Background(), &LoadLocalImageParams{Path: "/tmp/cat.png"}, llmtool.CallBackFuncs{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result != "当前会话不支持读取本地图片" {
		t.Fatalf("Execute() result = %q", result)
	}
}
