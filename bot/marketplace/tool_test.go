package marketplace

import (
	"context"
	"strings"
	"testing"
)

func TestEnsureTool(t *testing.T) {
	ctx := context.Background()
	// 本机/CI 都应有 go
	if err := ensureTool(ctx, "go", "version"); err != nil {
		t.Fatalf("ensureTool(go) 失败: %v", err)
	}
	if err := ensureTool(ctx, "git", "--version"); err != nil {
		t.Fatalf("ensureTool(git) 失败: %v", err)
	}
	// 不存在的工具应返回带 PATH 的明确错误
	err := ensureTool(ctx, "definitely-not-a-real-tool-xyz", "--version")
	if err == nil || !strings.Contains(err.Error(), "未找到") {
		t.Fatalf("期望未找到错误, got %v", err)
	}
}
