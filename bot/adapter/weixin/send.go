package weixin

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/jeanhua/AniaBot/common/msgchain"
)

// SendFriendMsg 发送私聊消息。
func (a *weixinAdapter) SendFriendMsg(userId message.QID, chain msgchain.FriendChain) (message.QID, bool) {
	return a.sendToUser(userId, chain.GetFriendMsg())
}

// SendGroupMsg 微信 iLink bot 无群聊会话：群目标按同前缀用户直发（尽力而为），
// 其他平台的群 ID 无 wx: 前缀，直接忽略。
func (a *weixinAdapter) SendGroupMsg(groupId message.QID, chain msgchain.GroupChain) (message.QID, bool) {
	return a.sendToUser(groupId, chain.GetGroupMsg())
}

// sendToUser 向目标用户发送通用消息段。目标须为 wx: 前缀（非本平台 ID 返回 false，
// 避免把其他平台 ID 误当作微信用户）。
func (a *weixinAdapter) sendToUser(target message.QID, segs []message.OB11Segment) (message.QID, bool) {
	raw := target.TrimPrefix(idPrefix)
	c := a.currentClient() // 凭证失效重登时整体替换，加锁取用
	if raw == target.String() || raw == "" || c == nil {
		return "", false
	}
	// 回复段仅入站表达上下文（微信回复出站无引用 API），出站剔除
	body := make([]message.OB11Segment, 0, len(segs))
	for _, s := range segs {
		if s.Type != message.SegmentReply {
			body = append(body, s)
		}
	}
	if len(body) == 0 {
		return "", false
	}

	ctx := context.Background()
	var text strings.Builder
	sentAny := false
	// flushText 发送累积文本（超长分包）
	flushText := func() {
		if text.Len() == 0 {
			return
		}
		t := text.String()
		text.Reset()
		for _, part := range splitText(t) {
			if a.sendText(c, ctx, raw, part) {
				sentAny = true
			}
		}
	}

	for _, s := range body {
		switch s.Type {
		case message.SegmentText:
			if t, ok := s.Data["text"].(string); ok {
				text.WriteString(t)
			}
		case message.SegmentImage, message.SegmentFile, message.SegmentRecord, message.SegmentVideo:
			flushText()
			if a.sendMediaSegment(c, ctx, raw, s) {
				sentAny = true
			}
		case message.SegmentMention:
			// 1:1 会话无 @ 概念，静默丢弃
		default:
			// 不支持的段（face/json/music/forward）：有 text 键时退化为文本
			if t, ok := s.Data["text"].(string); ok {
				text.WriteString(t)
			} else {
				a.logger.Debug("忽略微信不支持的通用消息段", "segment", s.Type)
			}
		}
	}
	flushText()
	if !sentAny {
		return "", false
	}
	return frameMsgID(raw, "out"), true
}

// sendText 发送一条文本消息。
func (a *weixinAdapter) sendText(c *client, ctx context.Context, rawUserID, text string) bool {
	return a.sendMessageItem(c, ctx, rawUserID, &MessageItem{Type: ItemText, TextItem: &TextItem{Text: text}})
}

// sendMediaSegment 发送一个媒体段：解析段内文件源为字节（http(s)/data:/base64:///
// file://），经 CDN 加密上传后按段型组装条目发送。
func (a *weixinAdapter) sendMediaSegment(c *client, ctx context.Context, rawUserID string, s message.OB11Segment) bool {
	mediaType, defaultName := uploadKindOf(s)
	src := segmentFileSource(s.Data)
	if src == "" {
		a.logger.Debug("微信媒体段缺少文件源，跳过", "segment", s.Type)
		return false
	}
	data, ok := a.resolveSegmentBytes(c, ctx, src)
	if !ok {
		a.logger.Warn("微信媒体资源解析失败", "source", previewSource(src))
		return false
	}
	name := defaultName
	if s.Type == message.SegmentFile {
		if n, _ := s.Data["name"].(string); n != "" {
			name = n
		} else if base := baseNameOf(src); base != "" {
			name = base
		}
	}
	up, err := a.uploadMedia(c, ctx, data, rawUserID, mediaType)
	if err != nil {
		a.logger.Warn("微信媒体上传失败", "error", err)
		return false
	}
	// aes_key 线上编码为 base64(hex 串)（对齐 openclaw-weixin：Buffer.from(hex).toString("base64")）
	aesKeyB64 := base64.StdEncoding.EncodeToString([]byte(up.AeskeyHex))
	media := &CDNMedia{EncryptQueryParam: up.DownloadParam, AesKey: aesKeyB64, EncryptType: 1}
	var item *MessageItem
	switch s.Type {
	case message.SegmentImage:
		item = &MessageItem{Type: ItemImage, ImageItem: &ImageItem{Media: media, MidSize: up.CipherSize}}
	case message.SegmentVideo:
		item = &MessageItem{Type: ItemVideo, VideoItem: &VideoItem{Media: media, VideoSize: up.CipherSize}}
	case message.SegmentRecord:
		item = &MessageItem{Type: ItemVoice, VoiceItem: &VoiceItem{Media: media}}
	default:
		item = &MessageItem{Type: ItemFile, FileItem: &FileItem{Media: media, FileName: name, Len: strconv.FormatInt(up.RawSize, 10)}}
	}
	return a.sendMessageItem(c, ctx, rawUserID, item)
}

// uploadKindOf 段型 → (上传媒体类型, 默认文件名)。
func uploadKindOf(s message.OB11Segment) (int, string) {
	switch s.Type {
	case message.SegmentImage:
		return UploadMediaImage, "image.png"
	case message.SegmentVideo:
		return UploadMediaVideo, "video.mp4"
	case message.SegmentRecord:
		return UploadMediaVoice, "voice.mp3"
	}
	return UploadMediaFile, "file"
}

// sendMessageItem 组装 WeixinMessage 并发送；携带票据被拒（ret 非零）时降级为
// 无 context_token 重发一次。
func (a *weixinAdapter) sendMessageItem(c *client, ctx context.Context, rawUserID string, item *MessageItem) bool {
	var send func(withToken bool) bool
	send = func(withToken bool) bool {
		msg := &WeixinMessage{
			ToUserID:     rawUserID,
			ClientID:     newClientID(),
			MessageType:  MsgTypeBot,
			MessageState: MsgStateFinish,
			ItemList:     []*MessageItem{item},
		}
		if withToken {
			msg.ContextToken = a.ctxTokens.get(rawUserID)
		}
		err := c.sendMessage(ctx, msg)
		if err == nil {
			return true
		}
		if withToken && !StaleToken(err) {
			// 票据可能过期/失效：降级为无票据重发一次
			return send(false)
		}
		a.logger.Warn("微信消息发送失败", "to", rawUserID, "itemType", item.Type, "error", err)
		return false
	}
	return send(true)
}

// newClientID 出站消息的客户端幂等 ID（对齐协议：任意可读唯一串）。
func newClientID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("aniabot-%d", time.Now().UnixNano())
	}
	return "aniabot-" + hex.EncodeToString(b[:])
}

// segmentFileSource 从段数据提取文件源：url 优先，其次 file。
func segmentFileSource(data map[string]any) string {
	if v, ok := data["url"].(string); ok && v != "" {
		return v
	}
	if v, ok := data["file"].(string); ok && v != "" {
		return v
	}
	return ""
}

// baseNameOf 从 http(s)/file 源推断文件名（仅用于默认命名；内联负载返回空）。
func baseNameOf(src string) string {
	switch {
	case strings.HasPrefix(src, "http://"), strings.HasPrefix(src, "https://"):
		if i := strings.IndexAny(src, "?#"); i >= 0 {
			src = src[:i]
		}
		return filepath.Base(src)
	case strings.HasPrefix(src, "file://"):
		return filepath.Base(strings.TrimPrefix(src, "file://"))
	}
	return ""
}

// previewSource 文件源预览（日志脱敏：截断 data/base64 内联负载）。
func previewSource(src string) string {
	if len(src) > 64 {
		return src[:64] + "…"
	}
	return src
}

// resolveSegmentBytes 解析文件源为字节：http(s) URL 下载（60s 超时）、
// base64:// / data: / file:// 本地解析。
func (a *weixinAdapter) resolveSegmentBytes(c *client, ctx context.Context, src string) ([]byte, bool) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	switch {
	case strings.HasPrefix(src, "http://"), strings.HasPrefix(src, "https://"):
		resp, err := c.http.R().SetContext(ctx).SetDoNotParseResponse(true).Get(src)
		if err != nil {
			return nil, false
		}
		defer resp.RawBody().Close()
		if resp.StatusCode() != http.StatusOK {
			return nil, false
		}
		b, err := io.ReadAll(resp.RawBody())
		return b, err == nil && len(b) > 0
	case strings.HasPrefix(src, "base64://"):
		b, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(src, "base64://"))
		return b, err == nil && len(b) > 0
	case strings.HasPrefix(src, "file://"):
		b, err := os.ReadFile(strings.TrimPrefix(src, "file://"))
		return b, err == nil && len(b) > 0
	case strings.HasPrefix(src, "data:"):
		if _, after, ok := strings.Cut(src, ","); ok {
			b, err := base64.StdEncoding.DecodeString(after)
			return b, err == nil && len(b) > 0
		}
	}
	return nil, false
}

// splitText 按微信单条消息上限（约 4000 字符，对齐 openclaw 的 textChunkLimit）
// 按 rune 分包。
func splitText(text string) []string {
	const limit = 4000
	runes := []rune(text)
	if len(runes) <= limit {
		if len(runes) == 0 {
			return nil
		}
		return []string{text}
	}
	var parts []string
	for start := 0; start < len(runes); start += limit {
		end := start + limit
		if end > len(runes) {
			end = len(runes)
		}
		parts = append(parts, string(runes[start:end]))
	}
	return parts
}
