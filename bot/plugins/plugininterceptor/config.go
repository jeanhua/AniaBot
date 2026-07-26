package plugininterceptor

// interceptorConfig 请求拦截插件的配置结构体。实现 plugin.ConfigSchemaProvider
// 后，框架启动时自动注册字段（面板渲染 + 默认值补齐），并在 Start 前填充完成。
type interceptorConfig struct {
	Enable bool   `cfg:"plugin.interceptor.enable" label:"启用请求拦截" group:"请求拦截插件" help:"开启后按名单模式放行或屏蔽群聊/好友的 AI 请求" default:"false"`
	Mode   string `cfg:"plugin.interceptor.mode" label:"名单模式" type:"select" options:"blacklist,whitelist" group:"请求拦截插件" help:"blacklist=名单内的群/好友被屏蔽；whitelist=仅名单内的群/好友放行" default:"blacklist"`
	// 名单留空的语义：blacklist 模式下表示不屏蔽任何会话；
	// whitelist 模式下表示拦截所有会话（任何群/好友都无法触发后续插件）
	Groups  []int `cfg:"plugin.interceptor.groups" label:"群号名单" group:"请求拦截插件" help:"每行一个群号"`
	Friends []int `cfg:"plugin.interceptor.friends" label:"QQ号名单" group:"请求拦截插件" help:"每行一个 QQ 号"`
}

// ConfigSchema 实现 plugin.ConfigSchemaProvider：返回配置结构体指针，
// 框架在 Start 前自动完成注册与填充，Start 里直接读 p.cfg。
func (p *InterceptorPlugin) ConfigSchema() any {
	return &p.cfg
}
