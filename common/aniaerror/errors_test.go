package aniaerror

import (
	"errors"
	"fmt"
	"testing"
)

func TestParameterInitializeError(t *testing.T) {
	wrapped := fmt.Errorf("%w: 未配置 Base Url", ParameterInitializeError)
	if !errors.Is(wrapped, ParameterInitializeError) {
		t.Fatal("包装后的错误应能 errors.Is 匹配 ParameterInitializeError")
	}
	if ParameterInitializeError.Error() != "参数初始化错误" {
		t.Fatalf("哨兵文本变化: %s", ParameterInitializeError.Error())
	}
}
