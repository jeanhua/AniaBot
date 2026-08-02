package message

// PlatformEvent 平台特定事件：无法映射为公共事件（消息/通知）的平台自有事件，
// 如飞书的卡片回调、机器人被拉进群、消息已读回执等。
// 由平台适配器产生，core 广播给实现了 plugin.PlatformEventHandler
// 且 Meta.Platforms 包含该平台的插件；Data 的类型由各平台适配器包定义，
// 插件按需类型断言。
type PlatformEvent struct {
	Platform string // 平台标识，与 Message.Platform 一致（如 "qq"、"feishu"）
	Type     string // 事件类型，约定 "<平台>.<事件名>"（如 "feishu.card_action"）
	Data     any    // 平台原始事件数据
}
