package plugininterceptor

import (
	"context"
	"testing"

	"github.com/jeanhua/AniaBot/common/model/command"
	"github.com/jeanhua/AniaBot/common/model/message"
)

func TestNormalizePlatformToken(t *testing.T) {
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"qq", "qq", true},
		{"QQ", "qq", true},
		{"qq:", "qq", true},
		{"qqofficial", "qqofficial", true},
		{"qo", "qqofficial", true},
		{"QO:", "qqofficial", true},
		{"telegram", "telegram", true},
		{"tg", "telegram", true},
		{"TG:", "telegram", true},
		{"feishu", "feishu", true},
		{"fs", "feishu", true},
		{"discord", "discord", true},
		{"dc", "discord", true},
		{"  tg  ", "telegram", true},
		{"", "", false},
		{"tgg", "", false},
		{"123", "", false},
		{"tg:-1001", "", false},
	}
	for _, c := range cases {
		got, ok := NormalizePlatformToken(c.in)
		if got != c.want || ok != c.ok {
			t.Errorf("NormalizePlatformToken(%q) = (%q, %v), want (%q, %v)", c.in, got, ok, c.want, c.ok)
		}
	}
}

func platformMsg(platform string, group, user message.QID) message.Message {
	m := groupMsg(group, user)
	m.Platform = platform
	return m
}

func platformFriend(platform string, user message.QID) message.Message {
	m := friendMsg(user)
	m.Platform = platform
	return m
}

func TestPlatformOfMessage(t *testing.T) {
	// 显式 Platform 优先（含简称/大小写容忍）
	if got := PlatformOfMessage(platformMsg("tg", "tg:-1001", "tg:111")); got != "telegram" {
		t.Errorf("Platform tg = %q", got)
	}
	if got := PlatformOfMessage(platformMsg("TG", "qq:123", "qq:456")); got != "telegram" {
		t.Errorf("显式 Platform 应优先于 ID 前缀, got %q", got)
	}
	// 为空时从 ID 推断
	if got := PlatformOfMessage(groupMsg("tg:-1001", "tg:111")); got != "telegram" {
		t.Errorf("从 tg 群 ID 推断 = %q", got)
	}
	if got := PlatformOfMessage(groupMsg("qq:123", "qq:456")); got != "qq" {
		t.Errorf("从 qq 群 ID 推断 = %q", got)
	}
	if got := PlatformOfMessage(friendMsg("fs:ou_abc")); got != "feishu" {
		t.Errorf("从 fs 用户 ID 推断 = %q", got)
	}
	if got := PlatformOfMessage(friendMsg("dc:123")); got != "discord" {
		t.Errorf("从 dc 用户 ID 推断 = %q", got)
	}
	if got := PlatformOfMessage(friendMsg("qo:abc")); got != "qqofficial" {
		t.Errorf("从 qo 用户 ID 推断 = %q", got)
	}
	// 裸数字视为 QQ 旧格式
	if got := PlatformOfMessage(friendMsg("123456")); got != "qq" {
		t.Errorf("裸数字应推断为 qq, got %q", got)
	}
	// 无法识别时透放（空串）
	if got := PlatformOfMessage(message.Message{}); got != "" {
		t.Errorf("空消息应返回空串, got %q", got)
	}
	if got := PlatformOfMessage(platformMsg("unknown", "tg:-1001", "tg:111")); got != "" {
		t.Errorf("未知 Platform 应返回空串, got %q", got)
	}
}

// 黑名单模式：取消勾选 tg 后，tg 群聊与私聊整体拦截，qq 不受影响；
// 勾选平台内仍按群名单屏蔽。
func TestPlatformGateBlacklist(t *testing.T) {
	p := newTestPlugin(t, interceptorConfig{
		Enable:    true,
		Mode:      modeBlacklist,
		Platforms: []string{"qq"},
		Groups:    []string{"qq:999"},
	})
	ctx := context.Background()

	if allowed, _ := p.OnGroupMsg(ctx, nil, command.Command{}, groupMsg("tg:-1001", "tg:111")); allowed {
		t.Error("未勾选平台的群聊应拦截")
	}
	if allowed, _ := p.OnFriendMsg(ctx, nil, command.Command{}, friendMsg("tg:111")); allowed {
		t.Error("未勾选平台的私聊应拦截")
	}
	if allowed, _ := p.OnGroupMsg(ctx, nil, command.Command{}, groupMsg("qq:123", "qq:456")); !allowed {
		t.Error("勾选平台的群聊应按名单放行")
	}
	if allowed, _ := p.OnGroupMsg(ctx, nil, command.Command{}, groupMsg("qq:999", "qq:456")); allowed {
		t.Error("勾选平台内命中群黑名单仍应拦截")
	}
	if allowed, _ := p.OnFriendMsg(ctx, nil, command.Command{}, friendMsg("qq:456")); !allowed {
		t.Error("勾选平台的私聊应放行")
	}
	// 显式 Platform 字段同样受总开关约束
	if allowed, _ := p.OnGroupMsg(ctx, nil, command.Command{}, platformMsg("telegram", "tg:-1001", "tg:111")); allowed {
		t.Error("显式 telegram 平台应拦截")
	}
	if allowed, _ := p.OnGroupMsg(ctx, nil, command.Command{}, platformMsg("qq", "qq:123", "qq:456")); !allowed {
		t.Error("显式 qq 平台应放行")
	}
}

// 白名单模式：平台开关是放行的必要条件——未勾选的平台直接拦截，
// 勾选的平台仍需命中群/用户名单。
func TestPlatformGateWhitelist(t *testing.T) {
	p := newTestPlugin(t, interceptorConfig{
		Enable:    true,
		Mode:      modeWhitelist,
		Platforms: []string{"telegram"},
		Groups:    []string{"tg:-1001"},
		Friends:   []string{"tg:111"},
	})
	ctx := context.Background()

	if allowed, _ := p.OnGroupMsg(ctx, nil, command.Command{}, groupMsg("tg:-1001", "tg:999")); !allowed {
		t.Error("勾选平台内命中群白名单应放行")
	}
	if allowed, _ := p.OnGroupMsg(ctx, nil, command.Command{}, groupMsg("tg:-1002", "tg:111")); allowed {
		t.Error("勾选平台内未命中群白名单应拦截")
	}
	if allowed, _ := p.OnGroupMsg(ctx, nil, command.Command{}, groupMsg("qq:123", "qq:456")); allowed {
		t.Error("未勾选平台的群聊应拦截")
	}
	if allowed, _ := p.OnFriendMsg(ctx, nil, command.Command{}, friendMsg("tg:111")); !allowed {
		t.Error("勾选平台内命中用户白名单应放行")
	}
	if allowed, _ := p.OnFriendMsg(ctx, nil, command.Command{}, friendMsg("qq:456")); allowed {
		t.Error("未勾选平台的私聊应拦截")
	}
}

// platforms 为 nil（从未配置）视为全选，保证旧调用与旧测试行为不变。
func TestPlatformsNilMeansAll(t *testing.T) {
	p := newTestPlugin(t, interceptorConfig{Enable: true, Mode: modeBlacklist})
	ctx := context.Background()
	if allowed, _ := p.OnGroupMsg(ctx, nil, command.Command{}, groupMsg("tg:-1001", "tg:111")); !allowed {
		t.Error("platforms 为 nil 时应视为全选透放")
	}
	for _, plat := range []string{"qq", "qqofficial", "telegram", "feishu", "discord"} {
		if !p.store.PlatformEnabled(plat) {
			t.Errorf("platforms 为 nil 时 %s 应启用", plat)
		}
	}
}

// 显式空切片（面板取消全选）表示全部平台拦截。
func TestPlatformsExplicitEmptyBlocksAll(t *testing.T) {
	p := newTestPlugin(t, interceptorConfig{Enable: true, Mode: modeBlacklist, Platforms: []string{}})
	ctx := context.Background()
	if allowed, _ := p.OnGroupMsg(ctx, nil, command.Command{}, groupMsg("qq:123", "qq:456")); allowed {
		t.Error("显式清空平台名单应拦截全部群聊")
	}
	if allowed, _ := p.OnFriendMsg(ctx, nil, command.Command{}, friendMsg("qq:456")); allowed {
		t.Error("显式清空平台名单应拦截全部私聊")
	}
}

// 未知平台名被忽略并上报，不影响合法项。
func TestUnknownPlatformTokenIgnored(t *testing.T) {
	var bad []string
	p := newTestPlugin(t, interceptorConfig{Enable: true, Mode: modeBlacklist, Platforms: []string{"tgg", "", "qq"}})
	_ = p
	s := NewListStore()
	s.LoadWithPlatforms(true, modeBlacklist, nil, nil, nil, []string{"tgg", "qq", "  "}, func(rule string) { bad = append(bad, rule) })
	if len(bad) != 1 || bad[0] != "tgg" {
		t.Errorf("非法平台名应上报, got %v", bad)
	}
	if !s.PlatformEnabled("qq") || s.PlatformEnabled("tg") {
		t.Error("合法平台名应生效，非法项应忽略")
	}
}

// 兼容旧签名的 Load 保留当前平台名单，仅重建群/用户名单。
func TestOldLoadKeepsPlatforms(t *testing.T) {
	s := NewListStore()
	s.LoadWithPlatforms(true, modeBlacklist, nil, nil, nil, []string{"qq"}, nil)
	s.Load(true, modeBlacklist, []string{"qq:123"}, nil, nil, nil)
	if !s.PlatformEnabled("qq") || s.PlatformEnabled("telegram") {
		t.Error("旧 Load 应保留平台名单")
	}
	if s.AllowGroup("qq:123") {
		// 黑名单模式：名单内群应拦截
		t.Error("旧 Load 应重建群名单")
	}
	if snap := s.Snapshot(); len(snap.Platforms) != 1 || snap.Platforms[0] != "qq" {
		t.Errorf("快照应包含平台名单, got %v", snap.Platforms)
	}
}
