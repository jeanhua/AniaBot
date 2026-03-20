package aichat

import (
	"regexp"
	"strings"
)

var thinkRegex = regexp.MustCompile(`(?s)<think>.*?</think>`)

func removeThinkContent(s string) string {
	return strings.TrimSpace(thinkRegex.ReplaceAllString(s, ""))
}
