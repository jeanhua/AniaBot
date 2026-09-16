package llmtool

import (
	"context"

	"github.com/jeanhua/AniaBot/common/aitool"
)

// providerTool 把平台注入工具（aitool.Tool）桥接为会话可注册的 llmtool.Tool。
// 平台工具不经过 CallBackFuncs：其执行所需能力在构造时已经 aitool.Context
// 绑定（bot 外观 + 会话目标），桥接层把回调参数置零透传。
type providerTool struct {
	inner aitool.Tool
}

// AdaptProviderTool 桥接平台注入工具为 llmtool.Tool。
func AdaptProviderTool(t aitool.Tool) Tool {
	return &providerTool{inner: t}
}

func (p *providerTool) Name() string        { return p.inner.Name() }
func (p *providerTool) Description() string { return p.inner.Description() }
func (p *providerTool) Params() any         { return p.inner.Params() }

func (p *providerTool) Execute(ctx context.Context, params any, _ CallBackFuncs) (string, error) {
	return p.inner.Execute(ctx, params)
}
