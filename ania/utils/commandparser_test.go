package utils

import (
	"fmt"
	"testing"
)

func TestParseCommand(t *testing.T) {
	// 测试
	testCases := []string{
		"/help",
		"/help arg1",
		"/help arg1 arg2",
		"/start 123",
		"/send 'hello there' world",
		"/set name 'John Doe'",
		"invalid",     // 应该报错
		"",            // 应该报错
		"/",           // 应该报错
		"/命令 参数1 参数2", // 中文测试
		"/my_command  arg1 arg2 arg3",
	}

	fmt.Println("=== 简单版本测试 ===")
	for _, tc := range testCases {
		fmt.Printf("\n输入: %q\n", tc)
		cmd := ParseCommand(tc)
		fmt.Printf("命令: %s, 参数: %v\n", cmd.Name, cmd.Args)
	}
}
