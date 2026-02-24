package aniaerror

import (
	"context"
	"testing"
)

func TestErrorsConstants(t *testing.T) {
	if Timeout != context.DeadlineExceeded {
		t.Fatalf("Timeout constant mismatch")
	}
	if UnknownError.Error() != "未知错误" {
		t.Fatalf("UnknownError text changed: %s", UnknownError.Error())
	}
}
