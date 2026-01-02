package napcat

import (
	"github.com/jeanhua/AniaBot/common/adapter"
)

func NewNapcatHttpAdapter() adapter.Adapter {
	return &napcatHttpAdapter{}
}

func NewNapcatWebSocketAdapter() adapter.Adapter {
	return &napcatWebSocketAdapter{}
}
