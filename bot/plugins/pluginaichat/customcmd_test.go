package pluginaichat

import (
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/jeanhua/AniaBot/common/model/command"
	"github.com/jeanhua/AniaBot/common/model/message"
)

func newTestCommandManager(editor *fakeConfigEditor) *commandManager {
	return newCommandManager(editor, commandsConfigKey, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func TestCustomCommandValidation(t *testing.T) {
	cases := []struct {
		name    string
		cmdName string
		tpl     string
		wantErr string
	}{
		{"正常", "translate", "把以下内容翻译成英文：$args", ""},
		{"带数字连字符", "my-cmd_2", "模板", ""},
		{"数字开头", "1abc", "模板", "名称非法"},
		{"空名", "", "模板", "名称非法"},
		{"含空格", "my cmd", "模板", "名称非法"},
		{"超长名", strings.Repeat("a", 33), "模板", "名称非法"},
		{"撞内置clock", "clock", "模板", "撞名"},
		{"撞内置stop大写", "STOP", "模板", "撞名"},
		{"空模板", "mycmd", "  ", "模板不能为空"},
		{"超长模板", "mycmd", strings.Repeat("长", customCommandMaxTemplateRunes+1), "上限"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateCustomCommand(strings.ToLower(tc.cmdName), tc.tpl)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("不应报错: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("期望错误含 %q, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestExpandCustomCommand(t *testing.T) {
	cases := []struct {
		tpl  string
		args []string
		want string
	}{
		{"把以下内容翻译成英文：$args", []string{"你好", "世界"}, "把以下内容翻译成英文：你好 世界"},
		{"$args 和 $args", []string{"a"}, "a 和 a"},        // 多处占位符
		{"总结群聊内容", []string{}, "总结群聊内容"},                 // 无占位符无参数
		{"总结群聊内容", []string{"最近100条"}, "总结群聊内容\n最近100条"}, // 无占位符有参数→末尾追加
		{"直接问：$args", []string{}, "直接问："},                // 空参数
	}
	for _, tc := range cases {
		if got := expandCustomCommand(tc.tpl, tc.args); got != tc.want {
			t.Errorf("expand(%q, %v) = %q, want %q", tc.tpl, tc.args, got, tc.want)
		}
	}
}

// TestCommandManagerAddDelList 读写往返：add/del 落库到配置中心并即时生效。
func TestCommandManagerAddDelList(t *testing.T) {
	editor := newFakeConfigEditor()
	m := newTestCommandManager(editor)

	if err := m.add("Translate", "翻译成英文：$args"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if tpl, ok := m.lookup("translate"); !ok || tpl != "翻译成英文：$args" {
		t.Fatalf("add 后应可查（大小写归一）, ok=%v tpl=%q", ok, tpl)
	}
	if err := m.add("hello", "打招呼"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if names := m.list(); len(names) != 2 || names[0] != "hello" || names[1] != "translate" {
		t.Fatalf("list 应排序, got %v", names)
	}

	// 落库内容可被独立实例读回（持久化语义）
	raw, _ := editor.Get(commandsConfigKey)
	m2 := newTestCommandManager(editor)
	if tpl, ok := m2.lookup("hello"); !ok || tpl != "打招呼" {
		t.Fatalf("配置中心读回失败, raw=%v", raw)
	}

	if err := m.del("translate"); err != nil {
		t.Fatalf("del: %v", err)
	}
	if _, ok := m.lookup("translate"); ok {
		t.Fatal("删除后不应再命中")
	}
	if err := m.del("ghost"); err == nil {
		t.Fatal("删除不存在的命令应报错")
	}
	if err := m.add("clock", "覆盖内置"); err == nil {
		t.Fatal("撞内置名应报错")
	}
}

// TestCommandManagerHotReload 面板直接改 files.commands_json 后 TTL 重读生效；
// 配置损坏时沿用旧快照。
func TestCommandManagerHotReload(t *testing.T) {
	editor := newFakeConfigEditor()
	m := newTestCommandManager(editor)

	// 首次查询必加载（lastCheck 零值）；此后进入 TTL 窗口
	if _, ok := m.lookup("a"); ok {
		t.Fatal("空配置不应命中")
	}

	// TTL 内不重读
	editor.Set(commandsConfigKey, `{"commands":{"a":"模板a"}}`)
	if _, ok := m.lookup("a"); ok {
		t.Fatal("TTL 内不应重读")
	}

	// 越过 TTL 生效
	m.lastCheck = time.Now().Add(-time.Minute)
	if tpl, ok := m.lookup("a"); !ok || tpl != "模板a" {
		t.Fatalf("TTL 后应读到新配置, ok=%v tpl=%q", ok, tpl)
	}

	// 配置损坏：沿用旧快照
	editor.Set(commandsConfigKey, `{bad json`)
	m.lastCheck = time.Now().Add(-time.Minute)
	if _, ok := m.lookup("a"); !ok {
		t.Fatal("配置损坏应沿用旧快照")
	}

	// 非法条目整体拒绝（不半截生效）
	editor.Set(commandsConfigKey, `{"commands":{"b":"模板b","clock":"撞内置"}}`)
	m.lastCheck = time.Now().Add(-time.Minute)
	if _, ok := m.lookup("b"); ok {
		t.Fatal("含非法条目的配置应整体拒绝")
	}
}

// TestRewriteCustomCommand 命中时消息被改写为展开后的单文本段，元信息保留；
// 未命中/非命令消息原样。
func TestRewriteCustomCommand(t *testing.T) {
	editor := newFakeConfigEditor()
	m := newTestCommandManager(editor)
	if err := m.add("translate", "翻译成英文：$args"); err != nil {
		t.Fatalf("add: %v", err)
	}

	msg := message.Message{
		MessageId: message.FromUint64(42),
		Sender:    message.MessageSender{UserId: message.FromUint64(123), Nickname: "小明"},
		Message:   []message.OB11Segment{{Type: "text", Data: map[string]any{"text": "/translate 你好"}}},
	}
	if !m.rewriteCustomCommand(command.Command{Name: "translate", Args: []string{"你好"}}, &msg) {
		t.Fatal("应命中自定义命令")
	}
	if len(msg.Message) != 1 || msg.Message[0].Data["text"] != "翻译成英文：你好" {
		t.Fatalf("消息应被改写为单文本段, got %+v", msg.Message)
	}
	if msg.MessageId != message.FromUint64(42) || msg.Sender.UserId != message.FromUint64(123) {
		t.Fatal("元信息应保留")
	}

	other := message.Message{Message: []message.OB11Segment{{Type: "text", Data: map[string]any{"text": "/unknown"}}}}
	if m.rewriteCustomCommand(command.Command{Name: "unknown"}, &other) {
		t.Fatal("未命中不应改写")
	}
	if m.rewriteCustomCommand(command.Command{}, &other) {
		t.Fatal("非命令消息不应改写")
	}
}
