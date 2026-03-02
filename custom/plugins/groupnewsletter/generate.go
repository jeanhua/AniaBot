package groupnewsletter

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jeanhua/AniaBot/common/bot"
	"github.com/jeanhua/AniaBot/common/msgchain"
	"github.com/jeanhua/AniaBot/custom/component/md2img"
	"github.com/tmc/langchaingo/llms"
)

func (p *GroupNewsletter) generateForGroup(ctx context.Context, b bot.Bot, groupId uint, force bool) {
	if !p.trySetGenerating(groupId) {
		p.Logger.Printf("[群刊] 群 %d 已在生成中，跳过", groupId)
		return
	}
	defer p.clearGenerating(groupId)

	// 取出消息快照，暂不清空（生成失败时需要回滚）
	msgs, ok := p.snapshotMessages(groupId, force)
	if !ok {
		return
	}

	p.Logger.Printf("[群刊] 群 %d 开始生成，共 %d 条消息", groupId, len(msgs))

	result, err := p.generateAI(ctx, msgs)
	if err != nil {
		p.Logger.Printf("[群刊] 群 %d 生成失败: %v，消息已回滚", groupId, err)
		// 生成失败：将快照消息还原到 buffer 头部
		p.rollbackMessages(groupId, msgs)

		builder := msgchain.Builder().Group()
		builder.Text("群刊生成失败，消息已保留，请稍后重试或使用 /gn gen 手动生成")
		b.SendGroupMsg(groupId, builder.Build())
		return
	}

	// 生成成功后才正式清空 buffer 并持久化
	p.clearMessages(groupId)

	name := fmt.Sprintf("群刊_%s_%s.md",
		time.Now().Format("2006-01-02_15-04-05"),
		uuid.NewString()[:8])

	b.SendGroupMsg(groupId, msgchain.Builder().Group().
		Text(fmt.Sprintf("📰 叮！消息达到阈值:%d条，本期群刊已生成，请查收~", p.config.msgThreshold)).
		Build())

	if p.config.fmt == "jpg" {
		imgData, err := md2img.GetImage(result)
		if err != nil {
			p.Logger.Printf("[群刊] md转图片失败: %v", err)
			b.SendGroupMsg(groupId, msgchain.Builder().Group().
				Text("转换失败，请查看原始md文件").Face(14).
				Build())
			return
		}
		b.SendGroupMsg(groupId, msgchain.Builder().Group().
			ImageBase64(base64.StdEncoding.EncodeToString(imgData)).
			Build())
	} else {
		b.SendGroupMsg(groupId, msgchain.Builder().Group().
			FileBase64(name, base64.StdEncoding.EncodeToString([]byte(result))).
			Build())
	}
}

// snapshotMessages 读取并临时清空 buffer，返回消息快照。
// 若消息不足（非 force 模式）或 buffer 为空，返回 false。
func (p *GroupNewsletter) snapshotMessages(groupId uint, force bool) ([]collectedMessage, bool) {
	p.msgsMu.Lock()
	defer p.msgsMu.Unlock()

	buffer, ok := p.groupMsgs[groupId]
	if !ok {
		return nil, false
	}

	buffer.mu.Lock()
	defer buffer.mu.Unlock()

	if len(buffer.messages) == 0 {
		return nil, false
	}
	if !force && len(buffer.messages) < p.config.msgThreshold {
		return nil, false
	}

	snapshot := make([]collectedMessage, len(buffer.messages))
	copy(snapshot, buffer.messages)
	// 暂时清空，防止生成期间重复触发；失败时由 rollbackMessages 还原
	buffer.messages = buffer.messages[:0]
	return snapshot, true
}

// rollbackMessages 将快照消息还原到 buffer 头部（追加到当前已有消息之前）
func (p *GroupNewsletter) rollbackMessages(groupId uint, snapshot []collectedMessage) {
	p.msgsMu.Lock()
	defer p.msgsMu.Unlock()

	buffer, ok := p.groupMsgs[groupId]
	if !ok {
		// buffer 已不存在，重建
		p.groupMsgs[groupId] = &groupMessageBuffer{messages: snapshot}
		return
	}

	buffer.mu.Lock()
	defer buffer.mu.Unlock()

	// 将快照放在最前，把生成期间新收到的消息追加到后面
	merged := make([]collectedMessage, 0, len(snapshot)+len(buffer.messages))
	merged = append(merged, snapshot...)
	merged = append(merged, buffer.messages...)

	// 超出上限时截断旧消息
	if len(merged) > p.config.maxMessages {
		merged = merged[len(merged)-p.config.maxMessages:]
	}
	buffer.messages = merged

	// 持久化回滚后的状态
	select {
	case p.saveChan <- groupId:
	default:
	}
}

// clearMessages 生成成功后正式清空 buffer 并触发持久化
func (p *GroupNewsletter) clearMessages(groupId uint) {
	p.msgsMu.Lock()
	defer p.msgsMu.Unlock()

	if buffer, ok := p.groupMsgs[groupId]; ok {
		buffer.mu.Lock()
		buffer.messages = buffer.messages[:0]
		buffer.mu.Unlock()
	}

	select {
	case p.saveChan <- groupId:
	default:
	}
}

func (p *GroupNewsletter) generateAI(ctx context.Context, msgs []collectedMessage) (string, error) {
	var sb strings.Builder
	var lastTime int64
	for _, msg := range msgs {
		// 距上一条消息超过 30 分钟，插入时间分隔线
		if lastTime == 0 || msg.Time-lastTime > 60*30 {
			lastTime = msg.Time
			sb.WriteString("\n")
			sb.WriteString(time.Unix(msg.Time, 0).Format("2006-01-02 15:04:05"))
			sb.WriteString("\n")
		}
		fmt.Fprintf(&sb, "[UserID]:%d\n[Username]:%s\n[Content]:%s\n",
			msg.UserId, msg.Nickname, msg.Content)
	}

	genCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()

	response, err := p.llm.GenerateContent(
		genCtx,
		[]llms.MessageContent{
			llms.TextParts(llms.ChatMessageTypeSystem, p.config.prompt),
			llms.TextParts(llms.ChatMessageTypeHuman, sb.String()),
		},
		llms.WithTemperature(1.2),
		llms.WithTopP(0.9),
	)
	if err != nil {
		return "", err
	}
	if len(response.Choices) == 0 {
		return "", fmt.Errorf("LLM 返回空结果")
	}

	return response.Choices[0].Content, nil
}
