package core

import (
	"testing"

	"github.com/jeanhua/AniaBot/common/adapter"
	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/jeanhua/AniaBot/common/msgchain"
	"github.com/spf13/viper"
)

// fakeAdapter 最小 Adapter 实现，仅用于测试路由。
type fakeAdapter struct {
	name     string
	platform string
}

func (f *fakeAdapter) Name() string                      { return f.name }
func (f *fakeAdapter) Platform() string                  { return f.platform }
func (f *fakeAdapter) SetTrigger(adapter.TriggerWrapper) {}
func (f *fakeAdapter) Serve(*viper.Viper)                {}
func (f *fakeAdapter) SendGroupMsg(message.QID, msgchain.GroupChain) (message.QID, bool) {
	return "", true
}
func (f *fakeAdapter) SendFriendMsg(message.QID, msgchain.FriendChain) (message.QID, bool) {
	return "", true
}
func (f *fakeAdapter) GetMsgDetail(message.QID) (*message.Message, bool)     { return nil, false }
func (f *fakeAdapter) GetGroupDetail(message.QID) (*message.GroupInfo, bool) { return nil, false }
func (f *fakeAdapter) GetGroupMsgHistory(message.QID, int, int) (*[]message.Message, bool) {
	return nil, false
}
func (f *fakeAdapter) GetFriendMsgHistory(message.QID, int, int) (*[]message.Message, bool) {
	return nil, false
}

// TestRouteIDPrefix 验证多适配器按统一 ID 前缀路由：QQ qq: 前缀路由到 NapCat，
// 飞书 fs: 前缀路由到飞书适配器，旧版裸数字 QQ ID 仍兼容回退到 QQ。
func TestRouteIDPrefix(t *testing.T) {
	a := &AniaBot{}
	qq := &fakeAdapter{name: "napcat", platform: "qq"}
	fs := &fakeAdapter{name: "feishu", platform: "feishu"}
	a.adapters = []*adapterEntry{
		{def: adapter.Definition{Name: "napcat", Platform: "qq", IDPrefix: message.QQIDPrefix}, adapter: qq},
		{def: adapter.Definition{Name: "feishu", Platform: "feishu", IDPrefix: "fs:"}, adapter: fs},
	}

	if got := a.route("qq:123456789"); got != qq {
		t.Fatalf("qq: 前缀应路由到 QQ 适配器，got %v", got.Name())
	}
	if got := a.route("fs:oc_123456"); got != fs {
		t.Fatalf("fs: 前缀应路由到飞书适配器，got %v", got.Name())
	}
	if got := a.route("123456789"); got != qq {
		t.Fatalf("旧版无前缀数字 ID 应兼容回退到 QQ 适配器，got %v", got.Name())
	}
}

// TestRouteEmptyAdapter 无适配器时路由返回 nil（发送接口优雅失败）。
func TestRouteEmptyAdapter(t *testing.T) {
	a := &AniaBot{}
	if got := a.route("123"); got != nil {
		t.Fatalf("无适配器时应返回 nil")
	}
	if _, ok := a.SendGroupMsg("123", nil); ok {
		t.Fatalf("无适配器时发送应失败")
	}
}
