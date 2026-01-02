package utils

import (
	"fmt"
	"strings"

	"github.com/jeanhua/AniaBot/common/model/command"
)

func ParseCommand(input string) (*command.Command, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, fmt.Errorf("输入不能为空")
	}
	if input[0] != '/' {
		return nil, fmt.Errorf("命令必须以 / 开头")
	}

	input = input[1:]
	if len(input) == 0 {
		return nil, fmt.Errorf("命令名不能为空")
	}

	parts := strings.Fields(input)
	if len(parts) == 0 {
		return nil, fmt.Errorf("无效的命令格式")
	}

	cmd := &command.Command{
		Name: parts[0],
		Args: make([]string, 0),
	}

	if len(parts) > 1 {
		cmd.Args = append(cmd.Args, parts[1:]...)
	}

	return cmd, nil
}
