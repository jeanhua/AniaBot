package qqofficial

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/jeanhua/AniaBot/common/model/message"
)

// ---------- 事件入站分发 ----------

// handleEvent 分发一条网关事件（在独立 goroutine 中调用）。
func (a *qqOfficialAdapter) handleEvent(eventType string, d json.RawMessage) {
	switch eventType {
	case "GROUP_AT_MESSAGE_CREATE":
		var ev groupMessageEvent
		if err := json.Unmarshal(d, &ev); err != nil {
			a.logger.Warn("解析群@消息事件失败", "error", err)
			return
		}
		a.onGroupMessage(&ev, true)
	case "GROUP_MESSAGE_CREATE":
		// 群消息全量模式（开放平台后台开启「接收所有消息」）：群内每一条消息
		// （不限于@机器人）都推送此事件，事件体与 GROUP_AT_MESSAGE_CREATE 一致。
		// 区别：mentions 包含被 @ 的机器人自身（AT 事件不含，按 bot 标志识别），
		// 且 content 不剥离 <@openid> 提及标记（翻译层按 mentions 剥离）。
		var ev groupMessageEvent
		if err := json.Unmarshal(d, &ev); err != nil {
			a.logger.Warn("解析群消息事件失败", "error", err)
			return
		}
		a.onGroupMessage(&ev, false)
	case "C2C_MESSAGE_CREATE":
		var ev c2cMessageEvent
		if err := json.Unmarshal(d, &ev); err != nil {
			a.logger.Warn("解析单聊消息事件失败", "error", err)
			return
		}
		a.onC2CMessage(&ev)
	case "FRIEND_ADD":
		var ev friendEvent
		if err := json.Unmarshal(d, &ev); err != nil {
			return
		}
		a.onFriendAdd(&ev)
	case "FRIEND_DEL":
		var ev friendEvent
		if err := json.Unmarshal(d, &ev); err != nil {
			return
		}
		a.emitPlatformEvent("qqofficial.friend_del", ev)
	case "GROUP_ADD_ROBOT":
		var ev groupRobotEvent
		if err := json.Unmarshal(d, &ev); err != nil {
			return
		}
		a.emitPlatformEvent("qqofficial.group_add_robot", ev)
	case "GROUP_DEL_ROBOT":
		var ev groupRobotEvent
		if err := json.Unmarshal(d, &ev); err != nil {
			return
		}
		a.emitPlatformEvent("qqofficial.group_del_robot", ev)
	case "GROUP_MSG_REJECT", "GROUP_MSG_RECEIVE", "C2C_MSG_REJECT", "C2C_MSG_RECEIVE":
		// 通知开关类事件：仅作平台事件透传（插件一般不关心）
		var ev groupRobotEvent
		if err := json.Unmarshal(d, &ev); err != nil {
			return
		}
		a.emitPlatformEvent("qqofficial."+strings.ToLower(eventType), ev)
	default:
		a.logger.Debug("忽略未处理的 QQ 官方事件", "event", eventType)
	}
}

// onGroupMessage 群消息事件 → 通用群消息。
// atOnly=true（GROUP_AT_MESSAGE_CREATE）时事件必然是 @机器人才推送，直接注入
// qq=SelfId 的 at 段作为首段；atOnly=false（GROUP_MESSAGE_CREATE 全量模式）时
// 仅当 mentions 含机器人自身才注入——非 @ 消息不带 at 段，与 NapCat 语义一致
// （aichat 仅在 @ 时响应，其余消息流经插件链做计数/清理）。
func (a *qqOfficialAdapter) onGroupMessage(ev *groupMessageEvent, atOnly bool) {
	selfRaw, selfName := a.selfInfo()
	// 过滤机器人消息（含全量模式下机器人自己发送的消息），防止 bot 循环
	if ev.Author.Bot || (selfRaw != "" && (ev.Author.ID == selfRaw || ev.Author.MemberOpenID == selfRaw)) {
		return
	}
	if ev.ID == "" || ev.GroupOpenID == "" || ev.Author.MemberOpenID == "" {
		a.logger.Warn("群消息事件缺少关键字段，丢弃", "id", ev.ID, "group", ev.GroupOpenID, "member", ev.Author.MemberOpenID)
		return
	}
	// 记录被动回复凭证（msg_id 5 分钟内可回复 5 次）
	a.storeReplyToken(ev.GroupOpenID, ev.ID)

	mentioned := atOnly
	if !mentioned {
		for _, m := range ev.Mentions {
			if isSelfMention(m, selfRaw, selfName) {
				mentioned = true
				break
			}
		}
	}

	segs := make([]message.OB11Segment, 0, 4)
	if mentioned {
		if self := a.selfQID(); self != "" {
			segs = append(segs, message.OB11Segment{
				Type: message.SegmentMention,
				Data: message.MentionMessage{QQ: self}.Marshal(),
			})
		}
	}
	segs = append(segs, contentToSegments(stripMentionMarkup(ev.Content, ev.Mentions), ev.MessageType, ev.MsgElements)...)
	segs = append(segs, attachmentsToSegments(ev.Attachments)...)
	for _, m := range ev.Mentions {
		openid := m.MemberOpenID
		if openid == "" {
			openid = m.ID
		}
		if openid == "" || isSelfMention(m, selfRaw, selfName) {
			continue // 机器人自身：已注入（或无需）SelfId at 段，不重复生成
		}
		segs = append(segs, message.OB11Segment{
			Type: message.SegmentMention,
			Data: message.MentionMessage{QQ: message.QID(idPrefix + openid)}.Marshal(),
		})
	}
	if len(segs) == 0 {
		return
	}
	msg := message.Message{
		Time:        uint(parseEventTime(ev.Timestamp)),
		PostType:    "message",
		MessageType: "group",
		MessageId:   message.QID(idPrefix + ev.ID),
		GroupId:     message.QID(idPrefix + ev.GroupOpenID),
		UserId:      message.QID(idPrefix + ev.Author.MemberOpenID),
		Message:     segs,
		RawMessage:  segmentsPlainText(segs),
		Sender: message.MessageSender{
			UserId:   message.QID(idPrefix + ev.Author.MemberOpenID),
			Nickname: ev.Author.Username,
			Role:     ev.Author.MemberRole, // member / admin / owner，与 OneBot role 语义一致
		},
		SelfId:   a.selfQID(),
		Platform: Platform,
	}
	a.msgCache.Push(ev.GroupOpenID, msg)
	if trig := a.triggerOf(); trig.OnGroupMsg != nil {
		trig.OnGroupMsg(msg)
	}
}

// isSelfMention 判断 mentions 条目是否指向机器人自身。
// READY 的 user.id 是机器人全局 ID，与群聊场景的 member_openid 不同源（实测不一致），
// 故主要依据官方 User 结构的 bot 标志（机器人条目 bot=true）；同群可能存在多个机器人，
// 辅以用户名比对（取自 READY，条目用户名缺失时放行避免漏判）；ID 全等作为兜底。
func isSelfMention(m eventUser, selfRaw, selfName string) bool {
	if m.Bot && (selfName == "" || m.Username == "" || m.Username == selfName) {
		return true
	}
	if selfRaw == "" {
		return false
	}
	return m.ID == selfRaw || (m.MemberOpenID != "" && m.MemberOpenID == selfRaw)
}

// stripMentionMarkup 剥离 content 中的 <@openid> 提及标记。
// 官方文档称 content「已去除@机器人的前缀」，但全量模式实测仍保留原始标记；
// 提及信息已由 at 段表达，正文残留标记只会污染 AI 输入。
func stripMentionMarkup(content string, mentions []eventUser) string {
	if !strings.Contains(content, "<@") {
		return content
	}
	for _, m := range mentions {
		for _, oid := range []string{m.MemberOpenID, m.ID} {
			if oid == "" {
				continue
			}
			content = strings.ReplaceAll(content, "<@"+oid+">", "")
			content = strings.ReplaceAll(content, "<@!"+oid+">", "")
		}
	}
	return content
}

// onC2CMessage 单聊消息 → 通用私聊消息（private + sub_type=friend，对齐 OneBot v11）。
func (a *qqOfficialAdapter) onC2CMessage(ev *c2cMessageEvent) {
	if ev.Author.Bot {
		return
	}
	userOpenID := ev.Author.UserOpenID
	if userOpenID == "" {
		userOpenID = ev.Author.ID
	}
	if ev.ID == "" || userOpenID == "" {
		a.logger.Warn("单聊消息事件缺少关键字段，丢弃", "id", ev.ID, "user", userOpenID)
		return
	}
	// 记录被动回复凭证（msg_id 60 分钟内可回复 4 次）
	a.storeReplyToken(userOpenID, ev.ID)

	segs := contentToSegments(ev.Content, ev.MessageType, ev.MsgElements)
	segs = append(segs, attachmentsToSegments(ev.Attachments)...)
	if len(segs) == 0 {
		return
	}
	msg := message.Message{
		Time:        uint(parseEventTime(ev.Timestamp)),
		PostType:    "message",
		MessageType: "private",
		SubType:     "friend",
		MessageId:   message.QID(idPrefix + ev.ID),
		UserId:      message.QID(idPrefix + userOpenID),
		Message:     segs,
		RawMessage:  segmentsPlainText(segs),
		Sender: message.MessageSender{
			UserId:   message.QID(idPrefix + userOpenID),
			Nickname: ev.Author.Username,
		},
		SelfId:   a.selfQID(),
		Platform: Platform,
	}
	a.msgCache.Push(userOpenID, msg)
	if trig := a.triggerOf(); trig.OnFriendMsg != nil {
		trig.OnFriendMsg(msg)
	}
}

// onFriendAdd 用户添加使用机器人 → 好友添加通知。
func (a *qqOfficialAdapter) onFriendAdd(ev *friendEvent) {
	trig := a.triggerOf()
	if trig.OnFriendAdd == nil || ev.OpenID == "" {
		return
	}
	n := message.FriendAddNotice{UserId: message.QID(idPrefix + ev.OpenID)}
	n.Time = uint(ev.Timestamp)
	n.PostType = "notice"
	n.NoticeType = "friend_add"
	n.SelfId = a.selfQID()
	n.SetPlatform(Platform)
	trig.OnFriendAdd(n)
}

func (a *qqOfficialAdapter) emitPlatformEvent(eventType string, data any) {
	trig := a.triggerOf()
	if trig.OnPlatformEvent == nil {
		return
	}
	trig.OnPlatformEvent(message.PlatformEvent{
		Platform: Platform,
		Type:     eventType,
		Data:     data,
	})
}

// ---------- 内容翻译 ----------

// contentToSegments 消息文本 → 通用段。
// message_type=103（引用消息）且正文为空时，取 msg_elements 的拼接内容兜底；
// ARK 卡片（message_type=3）的 content 已是平台拼好的文本摘要，直接使用。
func contentToSegments(content string, messageType int, elements []msgElement) []message.OB11Segment {
	text := strings.TrimSpace(content)
	if text == "" && messageType == 103 {
		var sb strings.Builder
		for _, e := range elements {
			if e.Content != "" {
				if sb.Len() > 0 {
					sb.WriteString("\n")
				}
				sb.WriteString(strings.TrimSpace(e.Content))
			}
		}
		text = sb.String()
	}
	if text == "" {
		return nil
	}
	return []message.OB11Segment{{
		Type: message.SegmentText,
		Data: message.TextMessage{Text: text}.Marshal(),
	}}
}

// attachmentsToSegments 附件 → 通用段：按 content_type 映射图片/语音/视频/文件。
// 附件 URL 为 QQ 多媒体 CDN 链接（https），直接写入 url 键（AI 插件 load_images 可加载）。
func attachmentsToSegments(atts []eventAttachment) []message.OB11Segment {
	var segs []message.OB11Segment
	for _, att := range atts {
		if att.URL == "" {
			continue
		}
		switch {
		case strings.HasPrefix(att.ContentType, "image/"):
			segs = append(segs, message.OB11Segment{
				Type: message.SegmentImage,
				Data: message.ImageMessage{File: att.URL, Url: att.URL}.Marshal(),
			})
		case att.ContentType == "voice":
			url := att.URL
			if att.VoiceWavURL != "" {
				url = att.VoiceWavURL // WAV 比 SILK 更通用（ASR 参考文本丢失不可恢复，保留原 URL 于 file 键）
			}
			segs = append(segs, message.OB11Segment{
				Type: message.SegmentRecord,
				Data: message.RecordMessage{URL: url}.Marshal(),
			})
		case strings.HasPrefix(att.ContentType, "video/"):
			segs = append(segs, message.OB11Segment{
				Type: message.SegmentVideo,
				Data: message.VideoMessage{URL: att.URL}.Marshal(),
			})
		default:
			segs = append(segs, message.OB11Segment{
				Type: message.SegmentFile,
				Data: message.FileMessage{File: att.URL, FileId: att.URL, Name: att.Filename}.Marshal(),
			})
		}
	}
	return segs
}

// parseEventTime 解析 RFC3339 事件时间戳（东八区），失败回退当前时间。
func parseEventTime(ts string) int64 {
	if t, err := time.Parse(time.RFC3339, ts); err == nil {
		return t.Unix()
	}
	return time.Now().Unix()
}

// segmentsPlainText 消息段的纯文本（供 RawMessage / 复读判等）。
func segmentsPlainText(segs []message.OB11Segment) string {
	var sb strings.Builder
	for _, s := range segs {
		if s.Type == message.SegmentText {
			if t, ok := s.Data["text"].(string); ok {
				sb.WriteString(t)
			}
		}
	}
	return sb.String()
}
