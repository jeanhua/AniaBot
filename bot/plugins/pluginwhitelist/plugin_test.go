package pluginwhitelist

import (
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/jeanhua/AniaBot/bot/plugins/plugininterceptor"
	"github.com/jeanhua/AniaBot/common/model/command"
	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/jeanhua/AniaBot/common/plugin"
)

// fakeEditor 内存版配置中心，模拟 ConfigEditor 的读写
type fakeEditor struct {
	vals    map[string]any
	failSet bool
}

func newFakeEditor() *fakeEditor {
	return &fakeEditor{vals: map[string]any{
		keyIntEnable: true,
		keyIntMode:   "whitelist",
	}}
}

func (f *fakeEditor) Get(key string) (any, bool) { v, ok := f.vals[key]; return v, ok }
func (f *fakeEditor) Set(key string, val any) error {
	if f.failSet {
		return fmt.Errorf("模拟写入失败")
	}
	f.vals[key] = val
	return nil
}
func (f *fakeEditor) Delete(key string) bool { delete(f.vals, key); return true }
func (f *fakeEditor) All() map[string]any    { return f.vals }

func newTestPlugin(t *testing.T) (*WhitelistPlugin, *fakeEditor) {
	t.Helper()
	p := NewPlugin()
	p.Logger = slog.Default()
	p.cfg = whitelistConfig{Enable: true, BlockAll: true}
	p.SystemConfig = plugin.SystemConfig{AdminId: message.FromString("qq:999")}
	ed := newFakeEditor()
	p.ConfigEditor = ed
	// 共享 store 是包级全局，逐个测试重置，避免互相串状态
	p.store.Load(true, "whitelist", nil, nil, nil, nil)
	return p, ed
}

func wlCmd(args ...string) command.Command {
	return command.Command{Name: "wl", Args: args, Mention: true}
}

func TestAddAndRemoveGroup(t *testing.T) {
	p, ed := newTestPlugin(t)
	group := message.FromString("qq:123")

	// 群里不带参数 = 加入本群
	out := p.handleCommand(wlCmd("add"), group, true)
	if !strings.Contains(out, "已加入") {
		t.Fatalf("add 未成功: %q", out)
	}
	if !p.store.AllowGroup(group) {
		t.Fatal("加入白名单后该群应放行（热生效失败）")
	}
	if got := ed.vals[keyIntGroups]; got == nil {
		t.Fatal("名单未写回配置中心")
	}

	// 重复添加应提示已存在
	if out := p.handleCommand(wlCmd("add"), group, true); !strings.Contains(out, "已经在名单里") {
		t.Fatalf("重复 add 应提示已存在: %q", out)
	}

	// 移出后不再放行
	if out := p.handleCommand(wlCmd("del"), group, true); !strings.Contains(out, "已移出") {
		t.Fatalf("del 未成功: %q", out)
	}
	if p.store.AllowGroup(group) {
		t.Fatal("移出白名单后该群不应再放行")
	}
	// 移出不存在的项应提示
	if out := p.handleCommand(wlCmd("del"), group, true); !strings.Contains(out, "不在名单里") {
		t.Fatalf("del 不存在项应提示: %q", out)
	}
}

// TestAddNormalizesBareNumericID 裸数字与 qq: 前缀应视为同一个 ID，
// 否则用户写 /wl add 123456 之后再 /wl del qq:123456 会删不掉。
func TestAddNormalizesBareNumericID(t *testing.T) {
	p, _ := newTestPlugin(t)
	p.handleCommand(wlCmd("add", "123456"), message.FromString("qq:1"), true)

	if !p.store.AllowGroup(message.FromString("qq:123456")) {
		t.Fatal("裸数字未规范化为 qq: 前缀")
	}
	out := p.handleCommand(wlCmd("del", "qq:123456"), message.FromString("qq:1"), true)
	if !strings.Contains(out, "已移出") {
		t.Fatalf("带前缀应能删掉裸数字写入的项: %q", out)
	}
}

func TestAddFriendInPrivate(t *testing.T) {
	p, ed := newTestPlugin(t)
	user := message.FromString("qq:555")

	// 私聊里不带参数 = 加入对方
	p.handleCommand(wlCmd("add"), user, false)
	if !p.store.AllowFriend(user) {
		t.Fatal("私聊 add 应写入用户名单")
	}
	if ed.vals[keyIntFriends] == nil {
		t.Fatal("用户名单未写回配置中心")
	}
	// 不应误写群名单
	if ed.vals[keyIntGroups] != nil {
		t.Fatal("私聊 add 不应动群名单")
	}
}

func TestSetModeAndToggle(t *testing.T) {
	p, ed := newTestPlugin(t)

	if out := p.handleCommand(wlCmd("mode", "blacklist"), "", false); !strings.Contains(out, "blacklist") {
		t.Fatalf("切换模式失败: %q", out)
	}
	if p.store.IsWhitelist() {
		t.Fatal("模式未热生效")
	}
	if out := p.handleCommand(wlCmd("mode", "bogus"), "", false); !strings.Contains(out, "只能是") {
		t.Fatalf("非法模式应被拒绝: %q", out)
	}

	if out := p.handleCommand(wlCmd("off"), "", false); !strings.Contains(out, "已关闭") {
		t.Fatalf("关闭失败: %q", out)
	}
	if p.store.Enabled() {
		t.Fatal("关闭未热生效")
	}
	if v, _ := ed.Get(keyIntEnable); v != false {
		t.Fatal("关闭未写回配置")
	}
	p.handleCommand(wlCmd("on"), "", false)
	if !p.store.Enabled() {
		t.Fatal("启用未热生效")
	}
}

// TestSaveFailureKeepsStoreConsistent 写配置失败时不应刷新内存状态，
// 否则内存放行了但重启后又变回拦截，行为前后不一致。
func TestSaveFailureKeepsStoreConsistent(t *testing.T) {
	p, ed := newTestPlugin(t)
	ed.failSet = true

	group := message.FromString("qq:777")
	out := p.handleCommand(wlCmd("add"), group, true)
	if !strings.Contains(out, "写入配置失败") {
		t.Fatalf("应上报写入失败: %q", out)
	}
	if p.store.AllowGroup(group) {
		t.Fatal("配置写入失败时不应让改动在内存生效")
	}
}

func TestHelpAndUnknownSubcommand(t *testing.T) {
	p, _ := newTestPlugin(t)
	if out := p.handleCommand(wlCmd("help"), "", false); !strings.Contains(out, "/wl add") {
		t.Fatalf("help 应包含用法: %q", out)
	}
	if out := p.handleCommand(wlCmd("bogus"), "", false); !strings.Contains(out, "未知的子命令") {
		t.Fatalf("未知子命令应提示: %q", out)
	}
	// 不带子命令返回状态
	if out := p.handleCommand(wlCmd(), "", false); !strings.Contains(out, "名单状态") {
		t.Fatalf("空参数应返回状态: %q", out)
	}
}

func TestListText(t *testing.T) {
	p, _ := newTestPlugin(t)
	if out := p.handleCommand(wlCmd("list"), "", false); !strings.Contains(out, "（空）") {
		t.Fatalf("空名单应显示（空）: %q", out)
	}
	p.handleCommand(wlCmd("add", "qq:123"), "", true)
	if out := p.handleCommand(wlCmd("list"), "", false); !strings.Contains(out, "qq:123") {
		t.Fatalf("list 应列出已加入的群: %q", out)
	}
}

// TestReadListHandlesAnySlice 配置中心返回 []any（JSON 解码形态）时也要能读，
// 否则重启后从数据库读回的名单会被当成空名单，白名单模式下拦住所有人。
func TestReadListHandlesAnySlice(t *testing.T) {
	p, ed := newTestPlugin(t)
	ed.vals[keyIntGroups] = []any{"qq:1", "qq:2", ""}

	got := p.readList(keyIntGroups)
	if len(got) != 2 || got[0] != "qq:1" || got[1] != "qq:2" {
		t.Fatalf("[]any 名单解析错误: %#v", got)
	}
}

// TestGateBlocksNonWhitelisted block_all 开启时非白名单会话应被拦下（返回 false），
// 且管理员恒放行——否则名单配错后管理员连 /wl 都发不进来。
func TestGateBlocksNonWhitelisted(t *testing.T) {
	p, _ := newTestPlugin(t)
	stranger := message.FromString("qq:111")
	admin := p.SystemConfig.AdminId

	msg := message.Message{Sender: message.MessageSender{UserId: stranger}}
	if pass, _ := p.gate(nil, message.FromString("qq:888"), true, msg); pass {
		t.Fatal("非白名单群应被拦下")
	}

	adminMsg := message.Message{Sender: message.MessageSender{UserId: admin}}
	if pass, _ := p.gate(nil, message.FromString("qq:888"), true, adminMsg); !pass {
		t.Fatal("管理员必须恒放行，否则会把自己关在门外")
	}

	// 加入白名单后放行
	p.handleCommand(wlCmd("add", "qq:888"), "", true)
	if pass, _ := p.gate(nil, message.FromString("qq:888"), true, msg); !pass {
		t.Fatal("已在白名单的群应放行")
	}
}

// TestGateRespectsBlockAllOff block_all 关闭时本插件不拦任何消息
// （拦截仍由 interceptor 在 AI 之前做）。
func TestGateRespectsBlockAllOff(t *testing.T) {
	p, _ := newTestPlugin(t)
	p.cfg.BlockAll = false

	msg := message.Message{Sender: message.MessageSender{UserId: message.FromString("qq:111")}}
	if pass, _ := p.gate(nil, message.FromString("qq:888"), true, msg); !pass {
		t.Fatal("block_all 关闭时本插件不应拦消息")
	}
}

// TestGateDisabledStoreAllowsAll 名单功能未启用时全部放行
func TestGateDisabledStoreAllowsAll(t *testing.T) {
	p, _ := newTestPlugin(t)
	p.store.Load(false, "whitelist", nil, nil, nil, nil)

	msg := message.Message{Sender: message.MessageSender{UserId: message.FromString("qq:111")}}
	if pass, _ := p.gate(nil, message.FromString("qq:888"), true, msg); !pass {
		t.Fatal("名单未启用时应全部放行")
	}
}

// TestSharedStoreIsInterceptorStore 本插件与 interceptor 必须共用同一份状态，
// 否则热重载改不到真正做拦截判定的那份名单。
func TestSharedStoreIsInterceptorStore(t *testing.T) {
	p, _ := newTestPlugin(t)
	if p.store != plugininterceptor.Store() {
		t.Fatal("未共用 interceptor 的名单存储")
	}
}
