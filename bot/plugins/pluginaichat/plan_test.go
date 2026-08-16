package pluginaichat

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/jeanhua/AniaBot/bot/component/agenthook"
	"github.com/jeanhua/AniaBot/bot/component/functool"
	"github.com/jeanhua/AniaBot/bot/component/llmtool"
	"github.com/jeanhua/AniaBot/common/model/message"
)

func TestPlanManagerOnOff(t *testing.T) {
	m := newPlanManager()
	const key = "g:1"
	if m.IsOn(key) {
		t.Fatal("默认应为关闭")
	}
	m.Set(key, true)
	if !m.IsOn(key) {
		t.Fatal("Set(true) 后应为开启")
	}
	m.Set(key, false)
	if m.IsOn(key) {
		t.Fatal("Set(false) 后应为关闭")
	}
	// 会话间隔离
	m.Set("g:1", true)
	if m.IsOn("g:2") || m.IsOn("f:1") {
		t.Fatal("计划模式应按会话隔离")
	}
}

// TestPlanGateBlocking 计划模式是门禁第一腿：开启时副作用工具被阻断并给出
// 计划提示，只读工具与 todo_write（规划工作流一部分）放行；关闭后恢复。
func TestPlanGateBlocking(t *testing.T) {
	p := &AIChatPlugin{planManager: newPlanManager()}
	const key = "g:1"
	gate := p.buildPreToolGate(key, agenthook.AgentKindMain, message.FromUint64(1), func(string) {})
	call := func(name string) (bool, string) {
		return gate(context.Background(), llmtool.ToolCall{Name: name, Arguments: "{}"})
	}

	if blocked, _ := call("bash"); blocked {
		t.Fatal("未开启计划模式时不应阻断 bash")
	}

	p.planManager.Set(key, true)
	blocked, result := call("bash")
	if !blocked || result == "" {
		t.Fatalf("计划模式下 bash 应被阻断并携带提示, blocked=%v result=%q", blocked, result)
	}
	for _, name := range []string{"time", "webSearch", "memory_search", "config_get", "todo_write"} {
		if blocked, _ := call(name); blocked {
			t.Fatalf("计划模式下只读/中性工具 %s 不应被阻断", name)
		}
	}

	p.planManager.Set(key, false)
	if blocked, _ := call("bash"); blocked {
		t.Fatal("退出计划模式后 bash 应恢复放行")
	}
}

// TestPlanBlockedToolsAreRealToolNames 阻断清单成员必须与真实注册的工具名一致，
// 防止工具改名后清单静默失效（清单里出现幽灵名字=计划模式漏拦）。
func TestPlanBlockedToolsAreRealToolNames(t *testing.T) {
	p := &AIChatPlugin{}
	p.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	p.PersistentStorage = newPFake()

	names := map[string]bool{}
	collect := func(tools []llmtool.Tool) {
		for _, tool := range tools {
			names[tool.Name()] = true
		}
	}

	// functool 基础工具（仅需名字，nil 存储不影响 Name()）
	bashTool, err := functool.NewBashTool(functool.BashConfig{})
	if err != nil {
		t.Fatalf("NewBashTool: %v", err)
	}
	names[bashTool.Name()] = true
	names[functool.NewSendFileTool().Name()] = true
	names[functool.NewConfigSetTool(nil).Name()] = true
	names[functool.NewConfigGetTool(nil).Name()] = true
	names[functool.NewConfigFileSetTool(nil).Name()] = true
	names[functool.NewConfigFileGetTool(nil).Name()] = true

	// 会话级工具（与 getChat/registerScopedTools 的注册来源一致）
	collect(newMemoryTools(newTestMemoryManager(0), "g:1", ""))
	collect(newKnowledgeTools(newTestKnowledgeManager(0), "g:1", ""))
	collect(newClockTools(newClockManager(p, time.Second, 10), clockTargetGroup, "1"))
	collect(newSkillTools(newTestSkillPlugin(t)))
	collect(newMCPTools(newMCPToolPlugin(t)))
	collect(newSubagentTools(p, nil, message.FromUint64(1), true))
	collect(newTeamTools(p, nil, message.FromUint64(1), true))

	for name := range planBlockedTools {
		if !names[name] {
			t.Errorf("planBlockedTools 中的 %q 不是真实注册的工具名", name)
		}
	}
	// todo_write 是规划工作流的一部分，刻意不在阻断清单中
	if _, blocked := planBlockedTools["todo_write"]; blocked {
		t.Error("todo_write 不应被计划模式阻断")
	}
}
