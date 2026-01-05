package utils

import (
	"strings"

	"github.com/jeanhua/AniaBot/common/model/command"
)

func ParseCommand(input string) *command.Command {
	if input == "" {
		return nil
	}
	if input[0] != '/' {
		return nil
	}

	input = input[1:]
	if len(input) == 0 {
		return nil
	}

	parts := strings.Fields(input)
	if len(parts) == 0 {
		return nil
	}

	cmd := &command.Command{
		Name: parts[0],
		Args: make([]string, 0),
	}

	if len(parts) > 1 {
		cmd.Args = append(cmd.Args, parts[1:]...)
	}

	return cmd
}
