package pluginaichat

import (
	"context"
	"strings"
	"testing"

	"github.com/jeanhua/AniaBot/bot/component/llmtool"
	"github.com/jeanhua/AniaBot/common/model/message"
)

// fakeMsgSource 测试用消息来源：实现 GetMsgDetail 与 GetForwardMsg 两个最小能力。
type fakeMsgSource struct {
	details  map[message.QID]*message.Message
	forwards map[message.QID]*[]message.Message
}

func (f *fakeMsgSource) GetMsgDetail(id message.QID) (*message.Message, bool) {
	d, ok := f.details[id]
	return d, ok
}

func (f *fakeMsgSource) GetForwardMsg(id message.QID) (*[]message.Message, bool) {
	d, ok := f.forwards[id]
	return d, ok
}

func imageSegment(url string) message.OB11Segment {
	return message.OB11Segment{Type: message.SegmentImage, Data: map[string]any{"url": url, "file": "img.png"}}
}

func TestImageRegistryResolve(t *testing.T) {
	reg := newImageRegistry()
	const urlA = "https://example.com/a.png"
	const urlB = "https://example.com/b.png"
	reg.register(message.ImageHash(urlA), urlA)
	reg.register(message.ImageHash(urlA), "https://example.com/dup.png") // 先到先得
	reg.register(message.ImageHash(urlB), urlB)

	found, missing := reg.resolve([]string{message.ImageHash(urlA), "deadbeef", message.ImageHash(urlB)})
	if len(found) != 2 {
		t.Fatalf("resolve found = %d, want 2", len(found))
	}
	if found[0].URL != urlA {
		t.Fatalf("首个引用 URL = %q, want %q（重复登记不应覆盖）", found[0].URL, urlA)
	}
	if len(missing) != 1 || missing[0] != "deadbeef" {
		t.Fatalf("missing = %v, want [deadbeef]", missing)
	}
}

func TestRegisterMessageImagesRecursive(t *testing.T) {
	const urlMain = "https://example.com/main.png"
	const urlReply = "https://example.com/reply.png"
	const urlForward = "https://example.com/forward.png"

	replyMsg := &message.Message{
		MessageId: message.FromString("reply-1"),
		Message:   []message.OB11Segment{imageSegment(urlReply)},
	}
	forwardMsg := &message.Message{
		MessageId: message.FromString("fwd-1"),
		Message:   []message.OB11Segment{imageSegment(urlForward)},
	}
	forwards := []message.Message{*forwardMsg}

	src := &fakeMsgSource{
		details: map[message.QID]*message.Message{
			message.FromString("r1"): replyMsg,
		},
		forwards: map[message.QID]*[]message.Message{
			message.FromString("f1"): &forwards,
		},
	}

	main := message.Message{
		MessageId: message.FromString("main-1"),
		Message: []message.OB11Segment{
			imageSegment(urlMain),
			{Type: message.SegmentReply, Data: map[string]any{"id": message.FromString("r1").String()}},
			{Type: message.SegmentForward, Data: map[string]any{"id": message.FromString("f1").String()}},
		},
	}

	reg := newImageRegistry()
	registerMessageImages(reg, src, main)

	found, missing := reg.resolve([]string{
		message.ImageHash(urlMain),
		message.ImageHash(urlReply),
		message.ImageHash(urlForward),
	})
	if len(found) != 3 {
		t.Fatalf("resolve found = %d, want 3 (missing=%v)", len(found), missing)
	}
}

func TestConfigureImageCallbacksLoadByHash(t *testing.T) {
	p := &AIChatPlugin{}
	p.cfg.Multimodal = true

	const urlA = "https://example.com/a.png"
	const urlB = "https://example.com/b.png"
	reg := newImageRegistry()

	// 当前消息中的图片由 configureImageCallbacks 自动登记
	curMsg := message.Message{Message: []message.OB11Segment{imageSegment(urlA), imageSegment(urlB)}}

	var cbs llmtool.CallBackFuncs
	p.configureImageCallbacks(context.Background(), nil, &cbs, reg, nil, curMsg)
	if cbs.LoadImages == nil {
		t.Fatal("LoadImages 回调未挂载")
	}

	// 只加载指定的哈希，不把全部图片塞进上下文
	res, err := cbs.LoadImages([]string{message.ImageHash(urlA)})
	if err != nil {
		t.Fatalf("LoadImages err = %v", err)
	}
	if !strings.Contains(res, "已加载 1 张图片") {
		t.Fatalf("LoadImages 提示异常, got %q", res)
	}
	if imgs := cbs.TakeLoadedImages(); len(imgs) != 1 || imgs[0] != urlA {
		t.Fatalf("待加载队列应为 [%s], got %v", urlA, imgs)
	}

	// 已加载的哈希重复调用不再加载
	res, err = cbs.LoadImages([]string{message.ImageHash(urlA), message.ImageHash(urlB)})
	if err != nil {
		t.Fatalf("LoadImages err = %v", err)
	}
	if !strings.Contains(res, "已加载 1 张图片") {
		t.Fatalf("第二次应只加载未加载过的图片, got %q", res)
	}
	if imgs := cbs.TakeLoadedImages(); len(imgs) != 1 || imgs[0] != urlB {
		t.Fatalf("待加载队列应为 [%s], got %v", urlB, imgs)
	}

	// 未登记的哈希给出引导
	res, err = cbs.LoadImages([]string{"deadbeef"})
	if err != nil {
		t.Fatalf("LoadImages err = %v", err)
	}
	if !strings.Contains(res, "没有找到可加载的图片") {
		t.Fatalf("未找到哈希时应提示, got %q", res)
	}

	// 空哈希返回引导，不加载任何图片
	res, err = cbs.LoadImages(nil)
	if err != nil {
		t.Fatalf("LoadImages err = %v", err)
	}
	if !strings.Contains(res, "hashes 参数") {
		t.Fatalf("空哈希应提示传入 hashes, got %q", res)
	}
}
