package functool

import "errors"

type OptionFuncs struct {
	SendText  func(text string) bool
	SendImage func(url string) bool
	SendFile  func(fileName, content string) bool
}

var (
	ToolExecuteError = errors.New("Function Tool执行错误")
)
