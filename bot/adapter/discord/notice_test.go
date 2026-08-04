package discord

import (
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/jeanhua/AniaBot/common/model/message"
)

// snowflakeAt 构造指定时间的 Discord snowflake（测试用）。
func snowflakeAt(t time.Time) string {
	return strconv.FormatUint(uint64(t.UnixMilli()-1420070400000)<<22, 10)
}

// 测试中跳过审计日志传播等待
func init() { recallAuditLogDelay = 0 }

func deleteEvent(channelID, msgID string) *discordgo.MessageDelete {
	return &discordgo.MessageDelete{
		Message: &discordgo.Message{ID: msgID, ChannelID: channelID, GuildID: "g1"},
	}
}

func TestResolveDeleterModerator(t *testing.T) {
	fake := &fakeDiscordAPI{auditLog: &discordgo.GuildAuditLog{
		AuditLogEntries: []*discordgo.AuditLogEntry{
			{ // 不匹配：同频道但目标是别人
				ID:       snowflakeAt(time.Now()),
				TargetID: "someone-else",
				UserID:   "mod0",
				Options:  &discordgo.AuditLogOptions{ChannelID: "c1"},
			},
			{ // 匹配：作者 + 频道 + 时近
				ID:       snowflakeAt(time.Now()),
				TargetID: "u1",
				UserID:   "mod1",
				Options:  &discordgo.AuditLogOptions{ChannelID: "c1"},
			},
		},
	}}
	a := NewAdapter(nil)
	a.api = fake
	got := a.resolveDeleter(deleteEvent("c1", "m1"), message.QID("dc:u1"))
	if got != "dc:mod1" {
		t.Fatalf("管理删除应解析出操作者 dc:mod1，得到 %q", got)
	}
	if fake.auditLogCalls != 1 {
		t.Fatalf("应查询一次审计日志，实际 %d", fake.auditLogCalls)
	}
}

func TestResolveDeleterSelfDelete(t *testing.T) {
	// 审计日志不记录本人自删：无匹配条目 → 操作者按作者处理
	fake := &fakeDiscordAPI{auditLog: &discordgo.GuildAuditLog{
		AuditLogEntries: []*discordgo.AuditLogEntry{
			{ // 不匹配：其他频道
				ID:       snowflakeAt(time.Now()),
				TargetID: "u1",
				UserID:   "mod1",
				Options:  &discordgo.AuditLogOptions{ChannelID: "c2"},
			},
		},
	}}
	a := NewAdapter(nil)
	a.api = fake
	got := a.resolveDeleter(deleteEvent("c1", "m1"), message.QID("dc:u1"))
	if got != "dc:u1" {
		t.Fatalf("自删应解析出作者 dc:u1，得到 %q", got)
	}
}

func TestResolveDeleterStaleEntryIgnored(t *testing.T) {
	// 同作者同频道的历史删除条目不应张冠李戴
	fake := &fakeDiscordAPI{auditLog: &discordgo.GuildAuditLog{
		AuditLogEntries: []*discordgo.AuditLogEntry{
			{
				ID:       snowflakeAt(time.Now().Add(-time.Hour)),
				TargetID: "u1",
				UserID:   "mod1",
				Options:  &discordgo.AuditLogOptions{ChannelID: "c1"},
			},
		},
	}}
	a := NewAdapter(nil)
	a.api = fake
	got := a.resolveDeleter(deleteEvent("c1", "m1"), message.QID("dc:u1"))
	if got != "dc:u1" {
		t.Fatalf("历史条目应忽略并按自删处理，得到 %q", got)
	}
}

func TestResolveDeleterAPIError(t *testing.T) {
	// 无 View Audit Log 权限等查询失败：不作自删假设，操作者留空
	a := NewAdapter(nil)
	a.api = &fakeDiscordAPI{err: errors.New("403: Missing Permissions")}
	got := a.resolveDeleter(deleteEvent("c1", "m1"), message.QID("dc:u1"))
	if got != "" {
		t.Fatalf("查询失败应留空，得到 %q", got)
	}
}

func TestResolveDeleterUnknownAuthor(t *testing.T) {
	// 作者未入缓存：审计条目不含消息 ID，无法匹配，不发起查询
	fake := &fakeDiscordAPI{}
	a := NewAdapter(nil)
	a.api = fake
	got := a.resolveDeleter(deleteEvent("c1", "m1"), "")
	if got != "" {
		t.Fatalf("作者未知应留空，得到 %q", got)
	}
	if fake.auditLogCalls != 0 {
		t.Fatalf("作者未知不应查询审计日志，实际 %d 次", fake.auditLogCalls)
	}
}

func TestSnowflakeTime(t *testing.T) {
	ts := time.UnixMilli(1754300000000)
	if got := snowflakeTime(snowflakeAt(ts)); !got.Equal(ts) {
		t.Fatalf("snowflakeTime 往返失败：%v != %v", got, ts)
	}
	if got := snowflakeTime("not-a-snowflake"); !got.IsZero() {
		t.Fatalf("非法 snowflake 应返回零值，得到 %v", got)
	}
}
