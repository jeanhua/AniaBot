package aniaerror

import (
	"context"
	"errors"
)

var (
	UnknownError             = errors.New("未知错误")
	ParameterInitializeError = errors.New("参数初始化错误")
	JsonSeralizeError        = errors.New("Json序列化错误")

	Timeout = context.DeadlineExceeded

	NetworkError = errors.New("网络请求错误")
)
