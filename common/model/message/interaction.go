package message

// 本文件定义内联交互（按钮键盘）的消息段与回调事件模型。
//
// 插件在消息链中附加 keyboard 段（msgchain Builder().Keyboard(...)），
// 支持的平台适配器（实现 adapter.InteractiveExt）把它翻译为平台原生组件
// （如 Telegram inline keyboard）；用户点击后适配器产生 InteractionEvent，
// core 按回调数据里的插件名前缀路由回对应插件的 OnInteraction。

// SegmentKeyboard 内联键盘段：附加在消息上的按钮行集合（每条消息至多一个，
// 多个时适配器取首个）。
const SegmentKeyboard = "keyboard"

// InlineButton 键盘中的一个按钮。
// Data 与 URL 二选一：Data 为回调按钮（点击产生 InteractionEvent，数据原样
// 返回，Telegram 限制 ≤64 字节）；URL 为链接按钮（点击打开网页，无回调）。
type InlineButton struct {
	Text string `json:"text"` // 按钮显示文本
	Data string `json:"data,omitempty"`
	URL  string `json:"url,omitempty"`
}

// KeyboardMessage 键盘段数据：rows × 按钮，一行内并排展示。
type KeyboardMessage struct {
	Rows [][]InlineButton `json:"rows"`
}

// Marshal 实现 typed 段数据 → OB11Segment.Data（键与 ParseKeyboard 一致）。
func (k KeyboardMessage) Marshal() map[string]any {
	return map[string]any{"rows": k.Rows}
}

// ParseKeyboard 从 keyboard 段解析键盘（读 Marshal 写入的 rows 键）。
// Data 兼容进程内 typed 值（[][]InlineButton）与 JSON 往返后的
// []any/map[string]any 两种形态。
func ParseKeyboard(s OB11Segment, k *KeyboardMessage) bool {
	if s.Type != SegmentKeyboard || s.Data == nil {
		return false
	}
	switch rows := s.Data["rows"].(type) {
	case [][]InlineButton:
		k.Rows = rows
		return len(rows) > 0
	case []any:
		parsed := make([][]InlineButton, 0, len(rows))
		for _, r := range rows {
			row, ok := r.([]any)
			if !ok {
				continue
			}
			btns := make([]InlineButton, 0, len(row))
			for _, b := range row {
				bm, ok := b.(map[string]any)
				if !ok {
					continue
				}
				btn := InlineButton{}
				btn.Text, _ = bm["text"].(string)
				btn.Data, _ = bm["data"].(string)
				btn.URL, _ = bm["url"].(string)
				if btn.Text == "" || (btn.Data == "" && btn.URL == "") {
					continue
				}
				btns = append(btns, btn)
			}
			if len(btns) > 0 {
				parsed = append(parsed, btns)
			}
		}
		k.Rows = parsed
		return len(parsed) > 0
	}
	return false
}

// ExtractKeyboard 取出消息链中的首个 keyboard 段：返回剩余段（新切片，
// 不修改入参）与键盘；链中无键盘段时 kb 为 nil。支持键盘的平台适配器
// 在出站时用它把键盘翻译为平台原生组件（不再随段链进入发送分支）。
func ExtractKeyboard(segs []OB11Segment) (rest []OB11Segment, kb *KeyboardMessage) {
	rest = make([]OB11Segment, 0, len(segs))
	for _, s := range segs {
		if s.Type == SegmentKeyboard && kb == nil {
			k := &KeyboardMessage{}
			if ParseKeyboard(s, k) {
				kb = k
				continue
			}
		}
		rest = append(rest, s)
	}
	return rest, kb
}

// StripKeyboardSegments 剥离消息链中的全部 keyboard 段（新切片，不动入参）。
// 平台不支持内联按钮时 core 在出站前调用，避免未知段类型透传导致发送失败。
func StripKeyboardSegments(segs []OB11Segment) []OB11Segment {
	found := false
	for _, s := range segs {
		if s.Type == SegmentKeyboard {
			found = true
			break
		}
	}
	if !found {
		return segs
	}
	rest := make([]OB11Segment, 0, len(segs))
	for _, s := range segs {
		if s.Type != SegmentKeyboard {
			rest = append(rest, s)
		}
	}
	return rest
}

// InteractionEvent 内联按钮点击回调事件的框架归一化形态
// （如 Telegram callback_query）。MessageType/GroupId/UserId/MessageId
// 与 Message 同词汇；CallbackId 为平台回调 ID（框架应答用）；
// Data 为按钮携带的回调数据去掉「插件名:」前缀后的载荷部分。
type InteractionEvent struct {
	Platform    string // 来源平台标识（与 Message.Platform 一致）
	MessageType string // "group" | "private"
	GroupId     QID    // 群聊点击时非空
	UserId      QID    // 点击者
	MessageId   QID    // 按钮所在消息
	CallbackId  string // 平台回调 ID（应答消除转圈用；平台无此概念时为空）
	Data        string // 回调载荷（无主/非法回调路由前由框架填充原始数据）
	// AnswerText 插件可填写的应答提示文本（如「已翻到第 3 页」，以 toast 形式
	// 展示在点击者客户端）；OnInteraction 返回后框架统一应答平台
	AnswerText string
	// Raw 平台原始回调数据（类型由适配器包定义，插件按需类型断言）
	Raw any
}
