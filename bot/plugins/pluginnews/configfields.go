package pluginnews

import "github.com/jeanhua/AniaBot/common/pluginconfig"

// ConfigFields 实现 plugin.ConfigRegistrar：声明每日新闻插件的配置字段，
// 框架启动时补齐缺失的默认值，Web 面板据此动态渲染表单。
func (p *NewsPlugin) ConfigFields() []pluginconfig.Field {
	return configFields
}

var configFields = []pluginconfig.Field{
	{Key: "plugin.dailynews.api", Label: "API 端点", Type: "string", Group: "每日新闻插件", Default: "https://60s.viki.moe/v2/60s?encoding=image-proxy"},
	{Key: "plugin.dailynews.cron", Label: "Cron 表达式", Type: "string", Group: "每日新闻插件", Help: "如 0 18 * * * 表示每天 18 点", Default: "0 18 * * *"},
	{Key: "plugin.dailynews.groups", Label: "播报群列表", Type: "ints", Group: "每日新闻插件", Help: "每行一个群号", Default: []int{123456, 7891011}},
}
