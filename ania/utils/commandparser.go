package utils

import (
	"strings"

	"github.com/jeanhua/AniaBot/common/model/command"
	"github.com/jeanhua/AniaBot/common/model/message"
)

func ParseCommand(msg message.Message) command.Command {
	input, mention := ExtraMessageStr(msg)
	if input == "" {
		return command.Command{
			Mention: mention,
		}
	}
	if input[0] != '/' {
		return command.Command{
			Mention: mention,
		}
	}

	input = input[1:]
	if len(input) == 0 {
		return command.Command{
			Mention: mention,
		}
	}

	parts := strings.Fields(input)
	if len(parts) == 0 {
		return command.Command{
			Mention: mention,
		}
	}

	cmd := command.Command{
		Name:    parts[0],
		Args:    make([]string, 0),
		Mention: mention,
	}

	if len(parts) > 1 {
		cmd.Args = append(cmd.Args, parts[1:]...)
	}

	return cmd
}
