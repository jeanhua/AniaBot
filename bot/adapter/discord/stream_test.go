package discord

import (
	"testing"
	"time"
)

func TestStreamInitialSend(t *testing.T) {
	fake := &fakeDiscordAPI{}
	a := newSendAdapter(fake)
	h, ok := a.SendGroupStream("dc:c1", segsChain{atSeg("dc:123"), textSeg("初始")})
	if !ok || h == nil {
		t.Fatal("流式创建失败")
	}
	if len(fake.sends) != 1 || fake.sends[0].Content != "<@123> 初始" {
		t.Fatalf("初始内容 = %+v", fake.sends)
	}
}

func TestStreamPatchThrottle(t *testing.T) {
	fake := &fakeDiscordAPI{}
	a := newSendAdapter(fake)
	h, ok := a.SendGroupStream("dc:c1", segsChain{textSeg("x")})
	if !ok {
		t.Fatal("流式创建失败")
	}
	// 节流窗口内的 Patch 不发送
	if err := h.Patch("一"); err != nil {
		t.Fatal(err)
	}
	if len(fake.edits) != 0 {
		t.Fatalf("节流窗口内不应编辑: %v", fake.edits)
	}
	// 模拟时间流逝后 Patch 发送
	sh := h.(*discordStreamHandle)
	sh.mu.Lock()
	sh.lastPatch = time.Now().Add(-streamPatchInterval)
	sh.mu.Unlock()
	if err := h.Patch("二"); err != nil {
		t.Fatal(err)
	}
	if len(fake.edits) != 1 || fake.edits[0] != "二" {
		t.Fatalf("编辑 = %v", fake.edits)
	}
}

func TestStreamEditKeepsPrefix(t *testing.T) {
	fake := &fakeDiscordAPI{}
	a := newSendAdapter(fake)
	h, ok := a.SendGroupStream("dc:c1", segsChain{atSeg("dc:123"), textSeg("x")})
	if !ok {
		t.Fatal("流式创建失败")
	}
	sh := h.(*discordStreamHandle)
	sh.mu.Lock()
	sh.lastPatch = time.Now().Add(-streamPatchInterval)
	sh.mu.Unlock()
	_ = h.Patch("AI 增量")
	if len(fake.edits) != 1 || fake.edits[0] != "<@123> AI 增量" {
		t.Fatalf("编辑应重带 prefix: %v", fake.edits)
	}
}

func TestStreamPatchSameContentSkipped(t *testing.T) {
	fake := &fakeDiscordAPI{}
	a := newSendAdapter(fake)
	h, ok := a.SendGroupStream("dc:c1", segsChain{textSeg("x")})
	if !ok {
		t.Fatal("流式创建失败")
	}
	sh := h.(*discordStreamHandle)
	sh.mu.Lock()
	sh.lastPatch = time.Now().Add(-2 * streamPatchInterval)
	sh.mu.Unlock()
	_ = h.Patch("同内容")
	sh.mu.Lock()
	sh.lastPatch = time.Now().Add(-2 * streamPatchInterval)
	sh.mu.Unlock()
	_ = h.Patch("同内容")
	if len(fake.edits) != 1 {
		t.Fatalf("内容未变的编辑应跳过: %v", fake.edits)
	}
}

func TestStreamEndForcesFinal(t *testing.T) {
	fake := &fakeDiscordAPI{}
	a := newSendAdapter(fake)
	h, ok := a.SendGroupStream("dc:c1", segsChain{textSeg("x")})
	if !ok {
		t.Fatal("流式创建失败")
	}
	_ = h.Patch("最终内容") // 节流窗口内仅记录
	h.End()
	if len(fake.edits) != 1 || fake.edits[0] != "最终内容" {
		t.Fatalf("End 应强制发送最终内容: %v", fake.edits)
	}
	// End 幂等 + End 后 Patch 无效
	h.End()
	_ = h.Patch("再来")
	if len(fake.edits) != 1 {
		t.Fatalf("End 后不应再编辑: %v", fake.edits)
	}
}

func TestStreamEmptyRejected(t *testing.T) {
	a := newSendAdapter(&fakeDiscordAPI{})
	if _, ok := a.SendGroupStream("dc:c1", segsChain{}); ok {
		t.Fatal("空内容应拒绝")
	}
	if _, ok := a.SendGroupStream("123456", segsChain{textSeg("x")}); ok {
		t.Fatal("非本平台 ID 应拒绝")
	}
}

func TestStreamFriendOpensDM(t *testing.T) {
	fake := &fakeDiscordAPI{}
	a := newSendAdapter(fake)
	h, ok := a.SendFriendStream("dc:u1", segsChain{textSeg("私聊流式")})
	if !ok || h == nil {
		t.Fatal("私聊流式创建失败")
	}
	if fake.sendChannel[0] != "dm-u1" {
		t.Fatalf("发送频道 = %q", fake.sendChannel[0])
	}
}
