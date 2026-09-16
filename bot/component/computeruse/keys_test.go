package computeruse

import (
	"sort"
	"strings"
	"testing"
)

func TestResolveVK(t *testing.T) {
	cases := map[string]uint16{
		"a": 0x41, "Z": 0x5A, "5": 0x35,
		"enter": 0x0D, "esc": 0x1B, "f5": 0x74,
		"CTRL": 0x11, " ctrl ": 0x11,
	}
	for name, want := range cases {
		vk, err := resolveVK(name)
		if err != nil || vk != want {
			t.Fatalf("resolveVK(%q) = (%d,%v), want (%d,nil)", name, vk, err, want)
		}
	}
	if _, err := resolveVK(" nonsense "); err == nil {
		t.Fatal("未知按键应报错")
	}
	if _, err := resolveVK(""); err == nil {
		t.Fatal("空按键名应报错")
	}
}

func TestParseKeyCombo(t *testing.T) {
	hold, tap, err := ParseKeyCombo("ctrl+shift+t")
	if err != nil {
		t.Fatalf("解析 ctrl+shift+t 失败: %v", err)
	}
	if len(hold) != 2 || tap != 'T' {
		t.Fatalf("组合键解析错误: hold=%v tap=%d", hold, tap)
	}
	// 单键
	hold, tap, err = ParseKeyCombo("enter")
	if err != nil || len(hold) != 0 || tap != 0x0D {
		t.Fatalf("单键解析错误: hold=%v tap=%d err=%v", hold, tap, err)
	}
	// 全修饰键：最后一个作为主键敲击
	hold, tap, err = ParseKeyCombo("ctrl+shift")
	if err != nil || len(hold) != 1 || hold[0] != 0x11 || tap != 0x10 {
		t.Fatalf("纯修饰键组合解析错误: hold=%v tap=%d err=%v", hold, tap, err)
	}
	// 大小写与空白
	if _, tap, _ = ParseKeyCombo("CTRL + C"); tap != 'C' {
		t.Fatalf("大小写/空白容错失败: tap=%d", tap)
	}
}

func TestParseKeyComboErrors(t *testing.T) {
	for _, combo := range []string{"", "ctrl++c", "ctrl+c+v", "ctrl+foo"} {
		if _, _, err := ParseKeyCombo(combo); err == nil {
			t.Fatalf("组合 %q 应解析失败", combo)
		}
	}
}

func TestKeyNamesDeterministic(t *testing.T) {
	a := KeyNames()
	b := KeyNames()
	if len(a) == 0 || !sort.StringsAreSorted(a) {
		t.Fatal("KeyNames 应非空且有序")
	}
	if strings.Join(a, ",") != strings.Join(b, ",") {
		t.Fatal("KeyNames 顺序必须确定（错误提示顺序不稳定会困扰使用者）")
	}
	if !sort.StringsAreSorted(a) {
		t.Fatal("KeyNames 应按字典序排序")
	}
}
