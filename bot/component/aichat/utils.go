package aichat

import (
	"regexp"
	"strings"
)

var thinkRegex = regexp.MustCompile(`(?s)<think>.*?</think>`)

// RemoveThinkContent 去除模型输出的 <think>...</think> 推理块并去除首尾空白。
// 流式场景下由调用方对累积缓冲统一调用（推理块可能跨多个增量）。
func RemoveThinkContent(s string) string {
	return strings.TrimSpace(thinkRegex.ReplaceAllString(s, ""))
}
