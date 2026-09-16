package computeruse

import (
	"fmt"
	"sort"
	"strings"
)

// vkCodes 常用虚拟键码表（键名 → Windows VK 码）。
// 修饰键组合（如 ctrl+c）由 ParseKeyCombo 拆分后逐个查表；
// 英文字母与数字由 rune 直算，不在此表。
var vkCodes = map[string]uint16{
	"ctrl": 0x11, "control": 0x11, "alt": 0x12, "shift": 0x10, "win": 0x5B, "meta": 0x5B, "cmd": 0x5B,
	"enter": 0x0D, "return": 0x0D, "tab": 0x09, "esc": 0x1B, "escape": 0x1B, "space": 0x20,
	"backspace": 0x08, "delete": 0x2E, "del": 0x2E, "insert": 0x2D, "ins": 0x2D,
	"home": 0x24, "end": 0x23, "pageup": 0x21, "pagedown": 0x22,
	"up": 0x26, "down": 0x28, "left": 0x25, "right": 0x27,
	"printscreen": 0x2C, "prtsc": 0x2C, "capslock": 0x14, "numlock": 0x90, "scrolllock": 0x91,
	"pause": 0x13, "menu": 0x5D, "apps": 0x5D,
	"f1": 0x70, "f2": 0x71, "f3": 0x72, "f4": 0x73, "f5": 0x74, "f6": 0x75,
	"f7": 0x76, "f8": 0x77, "f9": 0x78, "f10": 0x79, "f11": 0x7A, "f12": 0x7B,
	"f13": 0x7C, "f14": 0x7D, "f15": 0x7E, "f16": 0x7F,
}

// KeyNames 返回支持的按键名列表（字母/数字之外），按字典序排序。
// 供错误提示拼装，顺序必须确定。
func KeyNames() []string {
	names := make([]string, 0, len(vkCodes)+36)
	for name := range vkCodes {
		names = append(names, name)
	}
	for c := 'a'; c <= 'z'; c++ {
		names = append(names, string(c))
	}
	for d := 0; d <= 9; d++ {
		names = append(names, fmt.Sprint(d))
	}
	sort.Strings(names)
	return names
}

// resolveVK 把单个键名解析为 VK 码；支持 a-z、0-9 及 vkCodes 表中的名称。
func resolveVK(name string) (uint16, error) {
	key := strings.ToLower(strings.TrimSpace(name))
	if key == "" {
		return 0, fmt.Errorf("按键名不能为空")
	}
	if len(key) == 1 {
		c := key[0]
		switch {
		case c >= 'a' && c <= 'z':
			return uint16(0x41 + c - 'a'), nil
		case c >= '0' && c <= '9':
			return uint16(0x30 + c - '0'), nil
		}
	}
	if vk, ok := vkCodes[key]; ok {
		return vk, nil
	}
	return 0, fmt.Errorf("不支持的按键 %q（支持：%s）", name, strings.Join(KeyNames(), " "))
}

// isModifier 判断键名是否为修饰键（组合中需按住不放的键）。
func isModifier(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "ctrl", "control", "alt", "shift", "win", "meta", "cmd":
		return true
	}
	return false
}

// ParseKeyCombo 解析组合键表达式（如 "ctrl+shift+t"、"enter"）：
// 返回按住顺序（修饰键在前）与最后敲击的键。命名倒序排列保证
// 释放顺序与按住顺序相反，结果对相同输入确定。
func ParseKeyCombo(combo string) (hold []uint16, tap uint16, err error) {
	parts := strings.Split(combo, "+")
	if len(parts) == 0 || strings.TrimSpace(combo) == "" {
		return nil, 0, fmt.Errorf("按键组合不能为空")
	}
	var mods []string
	var main string
	for _, part := range parts {
		name := strings.TrimSpace(part)
		if name == "" {
			return nil, 0, fmt.Errorf("按键组合 %q 中存在空按键名", combo)
		}
		if isModifier(name) {
			mods = append(mods, strings.ToLower(name))
			continue
		}
		if main != "" {
			return nil, 0, fmt.Errorf("按键组合 %q 中出现多个非修饰键（只能有一个主键）", combo)
		}
		main = name
	}
	if main == "" {
		// 全是修饰键：没有主键时把最后一个修饰键当主键敲一下（如只按 shift）
		main = mods[len(mods)-1]
		mods = mods[:len(mods)-1]
	}
	for _, m := range mods {
		vk, err := resolveVK(m)
		if err != nil {
			return nil, 0, err
		}
		hold = append(hold, vk)
	}
	tap, err = resolveVK(main)
	if err != nil {
		return nil, 0, err
	}
	return hold, tap, nil
}
