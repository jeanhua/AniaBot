package pluginnews

// newsConfig 每日新闻插件的配置结构体。实现 plugin.ConfigSchemaProvider 后，
// 框架启动时自动注册字段（面板渲染 + 默认值补齐），并在 Start 前填充完成。
type newsConfig struct {
	API    string `cfg:"plugin.dailynews.api" label:"API 端点" group:"每日新闻插件" default:"https://60s.viki.moe/v2/60s?encoding=image-proxy"`
	Cron   string `cfg:"plugin.dailynews.cron" label:"Cron 表达式" group:"每日新闻插件" help:"如 0 18 * * * 表示每天 18 点" default:"0 18 * * *"`
	Groups []int  `cfg:"plugin.dailynews.groups" label:"播报群列表" group:"每日新闻插件" help:"每行一个群号" default:"123456,7891011"`
}

// ConfigSchema 实现 plugin.ConfigSchemaProvider：返回配置结构体指针，
// 框架在 Start 前自动完成注册与填充，Start 里直接读 p.cfg。
func (p *NewsPlugin) ConfigSchema() any {
	return &p.cfg
}
