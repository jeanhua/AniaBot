package pluginaichat

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/jeanhua/AniaBot/bot/component/llmtool"
	"github.com/jeanhua/AniaBot/common/model/message"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// ---- truncateSubagentResult ----

func TestTruncateSubagentResult(t *testing.T) {
	tests := []struct {
		name      string
		s         string
		maxRunes  int
		wantTrunc bool
	}{
		{"短结果不截断", "你好", 10, false},
		{"恰好等于上限不截断", "12345", 5, false},
		{"超长截断", "123456789", 5, true},
		{"中文按字符截断", "一二三四五六七八九十", 5, true},
		{"上限为0不截断", "任意长度内容", 0, false},
		{"上限为负不截断", "任意长度内容", -1, false},
		{"空字符串", "", 5, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, truncated := truncateSubagentResult(tt.s, tt.maxRunes)
			if truncated != tt.wantTrunc {
				t.Fatalf("truncated = %v, want %v", truncated, tt.wantTrunc)
			}
			if !tt.wantTrunc {
				if got != tt.s {
					t.Fatalf("未截断时结果应原样返回, got %q", got)
				}
				return
			}
			if !strings.Contains(got, "已截断") {
				t.Fatalf("截断结果应包含截断标记, got %q", got)
			}
			if !utf8.ValidString(got) {
				t.Fatalf("截断结果必须是合法 UTF-8（不能在多字节字符中间切断）")
			}
			// 截断正文不超过上限（标记文本除外）
			body := strings.SplitN(got, "\n…", 2)[0]
			if utf8.RuneCountInString(body) > tt.maxRunes {
				t.Fatalf("截断正文长度 %d 超过上限 %d", utf8.RuneCountInString(body), tt.maxRunes)
			}
		})
	}
}

// ---- 配置兜底 ----

func TestSubagentConfigDefaults(t *testing.T) {
	p := &AIChatPlugin{}
	if got := p.subagentTimeout(); got != 300*time.Second {
		t.Fatalf("默认超时 = %v, want 300s", got)
	}
	if got := p.subagentMaxIterations(); got != 10 {
		t.Fatalf("默认最大轮数 = %d, want 10", got)
	}
	if got := p.subagentMaxResultLen(); got != 4000 {
		t.Fatalf("默认结果上限 = %d, want 4000", got)
	}

	p.cfg.Subagent.TimeoutSec = 60
	p.cfg.Subagent.MaxIterations = 5
	p.cfg.Subagent.MaxResultLen = 100
	if got := p.subagentTimeout(); got != 60*time.Second {
		t.Fatalf("超时 = %v, want 60s", got)
	}
	if got := p.subagentMaxIterations(); got != 5 {
		t.Fatalf("最大轮数 = %d, want 5", got)
	}
	if got := p.subagentMaxResultLen(); got != 100 {
		t.Fatalf("结果上限 = %d, want 100", got)
	}
}

// TestSubagentTimeoutConfigClamp 配置值先限幅再乘 time.Second：
// 超大配置（>1800）被钳到 1800s，不会因 int64 溢出变成负 duration。
func TestSubagentTimeoutConfigClamp(t *testing.T) {
	p := &AIChatPlugin{}
	p.cfg.Subagent.TimeoutSec = 10000000000 // 直接乘会 int64 溢出为负值
	if got := p.subagentTimeout(); got != subagentMaxTimeoutSec*time.Second {
		t.Fatalf("超大配置超时应被限幅到 %v, got %v", subagentMaxTimeoutSec*time.Second, got)
	}
}

// ---- resolveSubagentTimeout ----

func TestResolveSubagentTimeout(t *testing.T) {
	def := 300 * time.Second

	t.Run("无父deadline时用默认超时", func(t *testing.T) {
		got, err := resolveSubagentTimeout(def, 0, context.Background())
		if err != nil || got != def {
			t.Fatalf("got %v, err %v; want %v", got, err, def)
		}
	})

	t.Run("单次调用覆盖默认超时", func(t *testing.T) {
		got, err := resolveSubagentTimeout(def, 60, context.Background())
		if err != nil || got != 60*time.Second {
			t.Fatalf("got %v, err %v; want 60s", got, err)
		}
	})

	t.Run("超大timeout_sec先限幅再乘防止溢出", func(t *testing.T) {
		got, err := resolveSubagentTimeout(def, 10000000000, context.Background())
		if err != nil || got != subagentMaxTimeoutSec*time.Second {
			t.Fatalf("got %v, err %v; want %v", got, err, subagentMaxTimeoutSec*time.Second)
		}
	})

	t.Run("父deadline预算不足时压缩超时", func(t *testing.T) {
		// 父请求还剩 60s：预留 30s 收尾后预算 30s，默认 300s 被压缩到 30s
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		got, err := resolveSubagentTimeout(def, 0, ctx)
		if err != nil {
			t.Fatalf("err = %v", err)
		}
		if got <= 0 || got > 30*time.Second {
			t.Fatalf("超时应被压缩到 (0, 30s], got %v", got)
		}
	})

	t.Run("父deadline预算耗尽时直接报错", func(t *testing.T) {
		// 父请求只剩 10s（不足 30s 预留），不应启动子代理
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := resolveSubagentTimeout(def, 0, ctx); err == nil {
			t.Fatal("预算耗尽时应返回错误")
		}
	})
}

// ---- makeSubagentCallbacks ----

func TestMakeSubagentCallbacks(t *testing.T) {
	p := &AIChatPlugin{}
	parentSendTextCalled := false
	parentLoadImagesCalled := false
	parent := llmtool.CallBackFuncs{
		SendText: func(s string) (string, error) {
			parentSendTextCalled = true
			return "发送成功", nil
		},
		SendImage: func(bs64 string) (string, error) {
			return "图片已发送:" + bs64, nil
		},
		LoadImages: func() (string, error) {
			parentLoadImagesCalled = true
			return "已加载 1 张图片", nil
		},
		TakeLoadedImages: func() []string {
			return []string{"parent-image"}
		},
	}
	cbs := p.makeSubagentCallbacks(context.Background(), parent, testLogger(), nil)

	// 中间轮文本被丢弃，不透传给主会话（避免打扰用户）
	res, err := cbs.SendText("中间过程文本")
	if err != nil {
		t.Fatalf("SendText err = %v", err)
	}
	if parentSendTextCalled {
		t.Fatal("子代理中间轮文本不应调用主会话的 SendText")
	}
	if !strings.Contains(res, "未发送") {
		t.Fatalf("SendText 应返回丢弃提示, got %q", res)
	}

	// 图片发送等能力透传主会话回调
	res, err = cbs.SendImage("base64data")
	if err != nil {
		t.Fatalf("SendImage err = %v", err)
	}
	if res != "图片已发送:base64data" {
		t.Fatalf("SendImage 应透传主会话回调, got %q", res)
	}

	// LoadImages 不透传：子代理有独立图片状态，不与主会话互踩
	res, err = cbs.LoadImages()
	if err != nil {
		t.Fatalf("LoadImages err = %v", err)
	}
	if parentLoadImagesCalled {
		t.Fatal("子代理 LoadImages 不应调用主会话回调（共享图片状态会互踩）")
	}
	if !strings.Contains(res, "子代理无法直接加载") {
		t.Fatalf("LoadImages 应返回独立提示, got %q", res)
	}

	// TakeLoadedImages 排空子代理自己的队列，不动主会话的
	if imgs := cbs.TakeLoadedImages(); len(imgs) != 0 {
		t.Fatalf("子代理图片队列初始应为空, got %v", imgs)
	}
}

// TestSubagentCallbacksLocalImageIsolation 多模态开启时，子代理经 LoadLocalImage
// 加载的本地图片进入子代理自己的队列（由 TakeLoadedImages 排空），与主会话隔离。
func TestSubagentCallbacksLocalImageIsolation(t *testing.T) {
	p := &AIChatPlugin{}
	p.cfg.Multimodal = true

	imgPath := filepath.Join(t.TempDir(), "test.png")
	if err := os.WriteFile(imgPath, []byte("fake-png-data"), 0o644); err != nil {
		t.Fatal(err)
	}

	parentTakeCalled := false
	parent := llmtool.CallBackFuncs{
		TakeLoadedImages: func() []string {
			parentTakeCalled = true
			return nil
		},
	}
	cbs := p.makeSubagentCallbacks(context.Background(), parent, testLogger(), nil)
	if cbs.LoadLocalImage == nil {
		t.Fatal("多模态开启时子代理应支持 LoadLocalImage")
	}

	res, err := cbs.LoadLocalImage(imgPath)
	if err != nil {
		t.Fatalf("LoadLocalImage err = %v", err)
	}
	if !strings.Contains(res, "已加载本地图片") {
		t.Fatalf("LoadLocalImage 提示异常, got %q", res)
	}
	imgs := cbs.TakeLoadedImages()
	if len(imgs) != 1 || !strings.HasPrefix(imgs[0], "data:image/png;base64,") {
		t.Fatalf("子代理图片队列应有 1 张 data URI 图片, got %v", imgs)
	}
	if parentTakeCalled {
		t.Fatal("子代理 TakeLoadedImages 不应触碰主会话图片队列")
	}
	// 排空后再次取应为空
	if imgs := cbs.TakeLoadedImages(); len(imgs) != 0 {
		t.Fatalf("排空后应返回空, got %v", imgs)
	}
}

// TestSubagentCallbacksLocalImageDisabled 未启用多模态且无 OCR 模型时，
// 子代理不支持读取本地图片（LoadLocalImage 为 nil，与 clock 回调语义一致）。
func TestSubagentCallbacksLocalImageDisabled(t *testing.T) {
	p := &AIChatPlugin{}
	cbs := p.makeSubagentCallbacks(context.Background(), llmtool.CallBackFuncs{}, testLogger(), nil)
	if cbs.LoadLocalImage != nil {
		t.Fatal("未启用多模态且无 OCR 模型时 LoadLocalImage 应为 nil")
	}
}

// ---- newSubagentTools ----

func TestNewSubagentTools(t *testing.T) {
	p := &AIChatPlugin{}

	groupTools := newSubagentTools(p, nil, message.FromUint64(12345), true)
	if len(groupTools) != 3 {
		t.Fatalf("应有 3 个子代理工具（run/list/cancel）, got %d", len(groupTools))
	}
	tool := groupTools[0]
	if tool.Name() != "subagent_run" {
		t.Fatalf("工具名 = %q, want subagent_run", tool.Name())
	}
	if !strings.Contains(tool.Description(), "群聊（会话 ID qq:12345）") {
		t.Fatalf("群聊会话描述应包含会话 ID, got %q", tool.Description())
	}
	if !strings.Contains(tool.Description(), "300 秒") {
		t.Fatalf("描述应包含默认超时, got %q", tool.Description())
	}

	friendTools := newSubagentTools(p, nil, message.FromUint64(67890), false)
	if !strings.Contains(friendTools[0].Description(), "私聊（对方 ID qq:67890）") {
		t.Fatalf("私聊会话描述应包含对方 ID, got %q", friendTools[0].Description())
	}
}

// ---- subagent_run 参数校验 ----

func TestSubagentRunToolEmptyTask(t *testing.T) {
	p := &AIChatPlugin{}
	tool := newSubagentTools(p, nil, message.FromUint64(1), true)[0]

	if _, err := tool.Execute(context.Background(), &subagentRunParams{Task: "   "}, llmtool.CallBackFuncs{}); err == nil {
		t.Fatal("空 task 应返回错误")
	}
}

// ---- 子代理/压缩器独立模型配置回退 ----

func TestSubagentLLMConfigFallback(t *testing.T) {
	p := &AIChatPlugin{}
	p.cfg.BaseURL = "https://main.example.com"
	p.cfg.APIKey = "main-key"
	p.cfg.Model = "main-model"

	// 全部留空：回退主模型
	base, key, model, format := p.subagentLLMConfig()
	if base != "https://main.example.com" || key != "main-key" || model != "main-model" {
		t.Fatalf("全空应回退主模型, got %q/%q/%q", base, key, model)
	}
	if format != "" {
		t.Fatalf("主格式为空时 format 应为空, got %q", format)
	}

	// 部分填充：只覆盖模型
	p.cfg.Subagent.Model = "cheap-model"
	_, _, model, _ = p.subagentLLMConfig()
	if model != "cheap-model" {
		t.Fatalf("model = %q, want cheap-model", model)
	}
	if base, _, _, _ := p.subagentLLMConfig(); base != "https://main.example.com" {
		t.Fatalf("base_url 未填应回退主模型, got %q", base)
	}

	// 格式回退：主格式 anthropic，子代理留空应继承；显式覆盖后生效
	p.cfg.APIFormat = "anthropic"
	if _, _, _, format = p.subagentLLMConfig(); format != "anthropic" {
		t.Fatalf("format 未填应回退主格式, got %q", format)
	}
	p.cfg.Subagent.APIFormat = "responses"
	if _, _, _, format = p.subagentLLMConfig(); format != "responses" {
		t.Fatalf("独立 format 未生效, got %q", format)
	}

	// 全部填充
	p.cfg.Subagent.BaseURL = "https://sub.example.com"
	p.cfg.Subagent.APIKey = "sub-key"
	base, key, model, _ = p.subagentLLMConfig()
	if base != "https://sub.example.com" || key != "sub-key" || model != "cheap-model" {
		t.Fatalf("独立配置未生效, got %q/%q/%q", base, key, model)
	}
}

func TestCompressorLLMConfigFallback(t *testing.T) {
	p := &AIChatPlugin{}
	p.cfg.BaseURL = "https://main.example.com"
	p.cfg.APIKey = "main-key"
	p.cfg.Model = "main-model"

	base, key, model, _ := p.compressorLLMConfig()
	if base != "https://main.example.com" || key != "main-key" || model != "main-model" {
		t.Fatalf("全空应回退主模型, got %q/%q/%q", base, key, model)
	}

	p.cfg.Compressor.Model = "summary-model"
	if _, _, model, _ := p.compressorLLMConfig(); model != "summary-model" {
		t.Fatalf("model = %q, want summary-model", model)
	}

	// 格式回退与独立覆盖
	p.cfg.APIFormat = "anthropic"
	if _, _, _, format := p.compressorLLMConfig(); format != "anthropic" {
		t.Fatalf("format 未填应回退主格式, got %q", format)
	}
	p.cfg.Compressor.APIFormat = "chat_completions"
	if _, _, _, format := p.compressorLLMConfig(); format != "chat_completions" {
		t.Fatalf("独立 format 未生效, got %q", format)
	}
}

func TestBuildCompressorClientNilWhenEmpty(t *testing.T) {
	p := &AIChatPlugin{}
	p.cfg.BaseURL = "https://main.example.com"
	p.cfg.APIKey = "main-key"
	p.cfg.Model = "main-model"
	// 三字段全空：返回 nil（压缩复用主对话客户端）
	if c := p.buildCompressorClient(); c != nil {
		t.Fatalf("三字段全空时应返回 nil, got %v", c)
	}
	// 配置了模型则构造独立客户端
	p.cfg.Compressor.Model = "summary-model"
	if c := p.buildCompressorClient(); c == nil {
		t.Fatal("配置了模型应返回独立客户端")
	}
}
