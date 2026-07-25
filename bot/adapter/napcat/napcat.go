package napcat

import (
	"time"

	"github.com/jeanhua/AniaBot/common/adapter"
)

func NewNapcatHttpAdapter() adapter.Adapter {
	return &napcatHttpAdapter{}
}

func NewNapcatWebSocketAdapter() adapter.Adapter {
	return &napcatWebSocketAdapter{
		// ackMng 必须在构造时就绪：首次启动等待设置向导时 Serve 尚未运行，
		// 插件（如系统插件 Awake 通知）仍可能调用发送接口，此时应优雅失败而非 panic。
		ackMng: &ackManager{timeout: time.Second * 10},
	}
}
