package llmtool

import "context"

type Tool interface {
	Name() string
	Description() string
	Params() any
	Execute(ctx context.Context, params any, callbacks CallBackFuncs) (string, error)
}

type CallBackFuncs struct {
	SendText          func(text string) (string, error)
	SendImage         func(bs64content string) (string, error)
	SendFile          func(name, bs64content string) (string, error)
	GetMsgHistory     func(count int, message_seq int) (string, error)
	GetPrivateFileURL func(fileId string) (string, error)
	LoadImages        func() (string, error)
	TakeLoadedImages  func() []string
	// LoadLocalImage 读取本地图片文件供 LLM 查看：主模型支持多模态时把 data URI
	// 推入待加载图片队列（下一轮上下文提供），否则交由备用识别模型描述。
	// 回调返回给 LLM 的提示文本；nil 表示当前会话不支持读取本地图片。
	LoadLocalImage func(path string) (string, error)
	// RequestApproval 请求人工批准一次危险操作（如 bash 三段式中既不在白名单也
	// 不在黑名单的命令）。返回 allowed=false 时 reason 说明原因（拒绝/超时/取消）。
	// nil 表示当前环境不支持审批（此时 bash 未列名命令默认放行，只认黑名单）。
	// 注意：本回调可能阻塞至审批超时（默认 120s），调用方与包装层不得持锁调用/包装它。
	RequestApproval func(ctx context.Context, toolName, summary string) (allowed bool, reason string)
}
