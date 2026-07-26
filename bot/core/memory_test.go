package core

import "testing"

func TestMatchPattern(t *testing.T) {
	cases := []struct {
		s, pattern string
		want       bool
	}{
		{"anything", "*", true},
		{"exact", "exact", true},
		{"exact", "other", false},
		{"user:1:active", "user:*:active", true},
		// 前后缀在原串中重叠时不能误判：user:active 缺少中间段
		{"user:active", "user:*:active", false},
		{"axbxc", "a*b*c", true},
		// 中间字面段 b 不存在
		{"a1c", "a*b*c", false},
		// '*' 可匹配空串（与 Redis glob 一致）
		{"abc", "a*b*c", true},
		{"prefix:key", "prefix:*", true},
		{"other:key", "prefix:*", false},
		{"hello-world", "*world", true},
		{"hello-world", "*mars", false},
		{"xxabcyy", "*abc*", true},
		{"xxdefyy", "*abc*", false},
	}
	for _, c := range cases {
		got, err := matchPattern(c.s, c.pattern)
		if err != nil {
			t.Fatalf("matchPattern(%q, %q) error: %v", c.s, c.pattern, err)
		}
		if got != c.want {
			t.Errorf("matchPattern(%q, %q) = %v, want %v", c.s, c.pattern, got, c.want)
		}
	}
}
