package llmtool

import (
	"context"
	"errors"
	"testing"
)

func TestCallBackFuncs_DescribeImage(t *testing.T) {
	cb := CallBackFuncs{
		DescribeImage: func(ctx context.Context, imageURL string) (string, error) {
			if imageURL == "" {
				return "", errors.New("empty url")
			}
			return "图片包含测试文字", nil
		},
	}

	if cb.DescribeImage == nil {
		t.Fatal("expected DescribeImage callback to be non-nil")
	}

	desc, err := cb.DescribeImage(context.Background(), "http://example.com/test.png")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if desc != "图片包含测试文字" {
		t.Errorf("expected '图片包含测试文字', got %q", desc)
	}

	_, err = cb.DescribeImage(context.Background(), "")
	if err == nil {
		t.Error("expected error for empty image url, got nil")
	}
}
