package plugininterceptor

// interceptorConfig 请求拦截插件的配置结构体。实现 plugin.ConfigSchemaProvider
// 后，框架启动时自动注册字段（面板渲染 + 默认值补齐），并在 Start 前填充完成。
type interceptorConfig struct {
	Enable bool   `cfg:"plugin.interceptor.enable" label:"启用请求拦截" group:"请求拦截插件" help:"开启后按名单模式放行或屏蔽群聊/好友的 AI 请求" default:"false"`
	Mode   string `cfg:"plugin.interceptor.mode" label:"名单模式" type:"select" options:"blacklist,whitelist" group:"请求拦截插件" help:"blacklist=名单内的群/好友被屏蔽；whitelist=仅名单内的群/好友放行" default:"blacklist"`
	// 名单留空的语义：blacklist 模式下表示不屏蔽任何会话；
	// whitelist 模式下表示拦截所有会话（任何群/好友都无法触发后续插件）
	// ID 支持多平台格式：QQ 为纯数字，其他平台带前缀（如飞书 fs:oc_xxx）
	Groups  []string `cfg:"plugin.interceptor.groups" label:"群 ID 名单" group:"请求拦截插件" help:"每行一个群 ID（QQ 为群号，其他平台为带前缀的群 ID）"`
	Friends []string `cfg:"plugin.interceptor.friends" label:"用户 ID 名单" group:"请求拦截插件" help:"每行一个用户 ID（QQ 为 QQ 号，其他平台为带前缀的用户 ID），对私聊及群聊消息发送者均生效"`
}

// ConfigSchema 实现 plugin.ConfigSchemaProvider：返回配置结构体指针，
// 框架在 Start 前自动完成注册与填充，Start 里直接读 p.cfg。
func (p *InterceptorPlugin) ConfigSchema() any {
	return &p.cfg
}
