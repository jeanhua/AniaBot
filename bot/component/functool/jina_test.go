package functool

import (
	"strings"
	"testing"
)

func intPtr(i int) *int { return &i }

func TestSliceJinaContent(t *testing.T) {
	// 短内容：原样返回，无续读提示
	short := strings.Repeat("字", 100)
	if got := sliceJinaContent(short, nil); got != short {
		t.Fatalf("短内容应原样返回，got 长度 %d", len([]rune(got)))
	}

	// 超长内容（无 offset）：返回前 8000 字 + 续读提示
	long := strings.Repeat("字", 9000)
	got := sliceJinaContent(long, nil)
	gotRunes := []rune(got)
	if string(gotRunes[:8000]) != strings.Repeat("字", 8000) {
		t.Fatalf("无 offset 应返回开头 8000 字")
	}
	if !strings.Contains(got, "offset=8000") {
		t.Fatalf("截断提示应给出续读位置 offset=8000，got 末尾 %q", string(gotRunes[8000:]))
	}

	// 带 offset 续读：返回剩余部分且不再附提示
	got = sliceJinaContent(long, intPtr(8000))
	if got != strings.Repeat("字", 1000) {
		t.Fatalf("offset=8000 应返回剩余 1000 字，got 长度 %d", len([]rune(got)))
	}

	// offset 超出内容长度：明确提示无更多内容
	got = sliceJinaContent(long, intPtr(9999))
	if !strings.Contains(got, "没有更多内容") {
		t.Fatalf("offset 超长应提示没有更多内容，got %q", got)
	}
}
