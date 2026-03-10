package plugininfo

type PluginInfo struct {
	Name      string
	HelpWords string
	AdminOnly bool
	ShowFor   ShowFor

	Author  string
	Version string
}

type ShowFor int

const (
	_ ShowFor = 1 << iota

	ShowForGroup  // 对群聊显示
	ShowForFriend // 对私聊显示
	ShowForNone   // 隐藏
)
