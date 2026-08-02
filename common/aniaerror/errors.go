package aniaerror

import "errors"

// ParameterInitializeError 插件参数初始化失败。
// 插件 Start/StartCron 返回时应使用 fmt.Errorf("%w: 具体原因", ParameterInitializeError)
// 包装具体原因，由框架统一记录错误日志。
var ParameterInitializeError = errors.New("参数初始化错误")
