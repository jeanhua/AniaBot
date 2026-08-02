package plugininterceptor

import (
	"context"
	"log/slog"
	"testing"

	"github.com/jeanhua/AniaBot/common/model/command"
	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/spf13/viper"
)

func newTestPlugin(t *testing.T, cfg interceptorConfig) *InterceptorPlugin {
	t.Helper()
	p := NewPlugin()
	// Logger 由框架注入，测试环境手动设置
	p.Logger = slog.Default()
	p.cfg = cfg
	if err := p.Start(context.Background(), viper.New()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	return p
}

func groupMsg(group, user message.QID) message.Message {
	return message.Message{
		MessageType: "group",
		GroupId:     group,
		Sender:      message.MessageSender{UserId: user},
	}
}

func friendMsg(user message.QID) message.Message {
	return message.Message{
		MessageType: "private",
		Sender:      message.MessageSender{UserId: user},
	}
}

func TestSplitGroupUser(t *testing.T) {
	cases := []struct {
		line      string
		wantGroup string
		wantUser  string
		wantOK    bool
	}{
		// QQ 纯数字
		{"123456:654321", "123456", "654321", true},
		// 双方均带平台前缀（Telegram 群 ID 为负数）
		{"tg:-1001234567:tg:98765", "tg:-1001234567", "tg:98765", true},
		// 飞书（原始 ID 含下划线）
		{"fs:oc_a1:fs:ou_b2", "fs:oc_a1", "fs:ou_b2", true},
		// 仅用户带前缀 / 仅群带前缀
		{"123:tg:456", "123", "tg:456", true},
		{"tg:123:456", "tg:123", "456", true},
		// 首尾空白容忍
		{"  tg:-1001:tg:9  ", "tg:-1001", "tg:9", true},
		// 非法输入
		{"", "", "", false},
		{"abc", "", "", false},
		{"tg:123", "", "", false},
		{":123", "", "", false},
		{"tg:123:", "", "", false},
	}
	for _, c := range cases {
		g, u, ok := splitGroupUser(c.line)
		if ok != c.wantOK || string(g) != c.wantGroup || string(u) != c.wantUser {
			t.Errorf("splitGroupUser(%q) = (%q, %q, %v), want (%q, %q, %v)",
				c.line, g, u, ok, c.wantGroup, c.wantUser, c.wantOK)
		}
	}
}

// 黑名单模式（默认）：群未被屏蔽即放行，群内屏蔽成员规则仅命中该群内指定用户；
// 私聊不受群内规则影响。
func TestGroupUserDenyBlacklist(t *testing.T) {
	p := newTestPlugin(t, interceptorConfig{
		Enable:     true,
		Mode:       modeBlacklist,
		GroupUsers: []string{"tg:-1001:tg:111", "tg:-1001:tg:222"},
	})
	ctx := context.Background()

	if allowed, _ := p.OnGroupMsg(ctx, nil, command.Command{}, groupMsg("tg:-1001", "tg:111")); allowed {
		t.Error("群内屏蔽成员应被拦截")
	}
	if allowed, _ := p.OnGroupMsg(ctx, nil, command.Command{}, groupMsg("tg:-1001", "tg:999")); !allowed {
		t.Error("群内其他成员应放行")
	}
	if allowed, _ := p.OnGroupMsg(ctx, nil, command.Command{}, groupMsg("tg:-1002", "tg:111")); !allowed {
		t.Error("屏蔽规则仅作用于指定群")
	}
	if allowed, _ := p.OnFriendMsg(ctx, nil, command.Command{}, friendMsg("tg:111")); !allowed {
		t.Error("群内屏蔽规则不应影响私聊")
	}
}

// 白名单模式：被放行的群对全体成员开放（无需逐个添加用户白名单），
// 未列入白名单的群整体拦截；用户名单仅作用于私聊。
func TestWhitelistGroupOpenToAll(t *testing.T) {
	p := newTestPlugin(t, interceptorConfig{
		Enable:  true,
		Mode:    modeWhitelist,
		Groups:  []string{"tg:-1001"},
		Friends: []string{"tg:111"},
	})
	ctx := context.Background()

	if allowed, _ := p.OnGroupMsg(ctx, nil, command.Command{}, groupMsg("tg:-1001", "tg:999")); !allowed {
		t.Error("白名单内的群应放行全部成员")
	}
	if allowed, _ := p.OnGroupMsg(ctx, nil, command.Command{}, groupMsg("tg:-1002", "tg:111")); allowed {
		t.Error("未列入白名单的群应拦截")
	}
	if allowed, _ := p.OnFriendMsg(ctx, nil, command.Command{}, friendMsg("tg:999")); allowed {
		t.Error("私聊仍按用户名单：未列入名单的用户应拦截")
	}
	if allowed, _ := p.OnFriendMsg(ctx, nil, command.Command{}, friendMsg("tg:111")); !allowed {
		t.Error("私聊应按用户名单放行")
	}
}

// "放行某群但禁止群内某人"：白名单模式放行整个群，群内屏蔽成员规则单独命中目标成员。
func TestWhitelistGroupDenyMember(t *testing.T) {
	p := newTestPlugin(t, interceptorConfig{
		Enable:     true,
		Mode:       modeWhitelist,
		Groups:     []string{"tg:-1001"},
		GroupUsers: []string{"tg:-1001:tg:111"},
	})
	ctx := context.Background()

	if allowed, _ := p.OnGroupMsg(ctx, nil, command.Command{}, groupMsg("tg:-1001", "tg:111")); allowed {
		t.Error("群内屏蔽成员应被拦截")
	}
	if allowed, _ := p.OnGroupMsg(ctx, nil, command.Command{}, groupMsg("tg:-1001", "tg:999")); !allowed {
		t.Error("群内其他成员应放行")
	}
	// 用户名单为空 → 私聊全部拦截（与群内规则无关）
	if allowed, _ := p.OnFriendMsg(ctx, nil, command.Command{}, friendMsg("tg:111")); allowed {
		t.Error("白名单模式下用户名单为空，私聊应拦截")
	}
}

// 回归：黑名单模式下用户名单对群聊发送者同样生效（原有行为不变）。
func TestFriendsBlacklistAppliesToGroup(t *testing.T) {
	p := newTestPlugin(t, interceptorConfig{
		Enable:  true,
		Mode:    modeBlacklist,
		Friends: []string{"tg:111"},
	})
	ctx := context.Background()

	if allowed, _ := p.OnGroupMsg(ctx, nil, command.Command{}, groupMsg("tg:-1001", "tg:111")); allowed {
		t.Error("黑名单用户应同时在群聊中被拦截")
	}
	if allowed, _ := p.OnGroupMsg(ctx, nil, command.Command{}, groupMsg("tg:-1001", "tg:999")); !allowed {
		t.Error("非黑名单用户应放行")
	}
}

// 非法规则行被忽略（告警日志），不影响其余规则与放行判定。
func TestMalformedGroupUserRuleIgnored(t *testing.T) {
	p := newTestPlugin(t, interceptorConfig{
		Enable:     true,
		Mode:       modeBlacklist,
		GroupUsers: []string{"garbage", "no-sep", "tg:-1001:tg:111"},
	})
	ctx := context.Background()

	if allowed, _ := p.OnGroupMsg(ctx, nil, command.Command{}, groupMsg("tg:-1001", "tg:111")); allowed {
		t.Error("合法规则应生效")
	}
	if allowed, _ := p.OnGroupMsg(ctx, nil, command.Command{}, groupMsg("tg:-1002", "tg:222")); !allowed {
		t.Error("非法规则被忽略，其余消息正常放行")
	}
}

func TestInterceptorDisabled(t *testing.T) {
	p := newTestPlugin(t, interceptorConfig{
		Enable:     false,
		GroupUsers: []string{"tg:-1001:tg:111"},
	})
	ctx := context.Background()

	if allowed, _ := p.OnGroupMsg(ctx, nil, command.Command{}, groupMsg("tg:-1001", "tg:111")); !allowed {
		t.Error("未启用时全部放行")
	}
}
