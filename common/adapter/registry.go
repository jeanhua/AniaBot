package adapter

import (
	"fmt"
	"sort"
	"sync"

	"github.com/jeanhua/AniaBot/common/bot"
	"github.com/jeanhua/AniaBot/common/pluginconfig"
	"github.com/spf13/viper"
)

// Definition 描述一个平台适配器：如何创建、平台标识、统一 ID 前缀与面板配置字段。
//
// 新增平台（如 Telegram）时无需改动框架核心：在新包中实现 Adapter，
// 提供 Definition 并在 cmd/main.go 调用 adapter.Register 注册即可；
// 平台专属能力以实现可选能力接口（参照 QQExt）+ RegisterBotWrapper 的方式扩展。
type Definition struct {
	// Name 适配器名（唯一），启用配置键约定为 bot.platform.<name>.enable
	Name string
	// Platform 平台标识（如 "qq"、"feishu"），写入事件 Platform，供插件 Meta.Platforms 过滤
	Platform string
	// IDPrefix 该平台 ID 的框架统一前缀（如 QQ "qq:"、飞书 "fs:"）；
	// 为空表示无前缀的默认平台（仅兼容旧版自定义适配器）
	IDPrefix string
	// ConfigFields 平台配置字段（面板动态渲染），应包含 bot.platform.<name>.enable
	ConfigFields []pluginconfig.Field
	// New 工厂：配置加载完成后由 core 调用；返回 error 时记日志并跳过该平台
	New func(cfg *viper.Viper) (Adapter, error)
}

var (
	registryMu sync.RWMutex
	registry   = map[string]Definition{}
)

// Register 注册平台适配器定义。同名或同非空 ID 前缀重复注册会 panic（启动期编程错误）。
func Register(d Definition) {
	if d.Name == "" || d.Platform == "" || d.New == nil {
		panic("adapter.Register: Name/Platform/New 不能为空")
	}
	registryMu.Lock()
	defer registryMu.Unlock()
	if _, ok := registry[d.Name]; ok {
		panic(fmt.Sprintf("adapter.Register: 适配器 %q 重复注册", d.Name))
	}
	if d.IDPrefix != "" {
		for _, e := range registry {
			if e.IDPrefix == d.IDPrefix {
				panic(fmt.Sprintf("adapter.Register: ID 前缀 %q 被 %q 与 %q 重复占用", d.IDPrefix, e.Name, d.Name))
			}
		}
	}
	registry[d.Name] = d
}

// Definitions 返回全部已注册定义（按 Name 排序，保证遍历稳定）。
func Definitions() []Definition {
	registryMu.RLock()
	defer registryMu.RUnlock()
	out := make([]Definition, 0, len(registry))
	for _, d := range registry {
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// BotWrapper 按事件来源适配器的能力包装 bot.Bot 外观：
// 包装器检查 src 实现的能力接口（如 QQExt），命中则返回带平台专属方法的扩展外观，
// 否则原样返回 base。由平台适配器包在 init() 中通过 RegisterBotWrapper 注册。
//
// 这样插件在事件回调里可以用类型断言探测平台能力：
//
//	if qb, ok := b.(bot.QQ); ok { qb.SendPokeMsg(...) }
//
// 断言仅当事件来自实现了对应能力接口的适配器时成功。
type BotWrapper func(base bot.Bot, src Adapter) bot.Bot

var (
	botWrapperMu sync.RWMutex
	botWrappers  []BotWrapper
)

// RegisterBotWrapper 注册 bot 外观包装器（平台适配器包的 init() 中调用）。
func RegisterBotWrapper(w BotWrapper) {
	botWrapperMu.Lock()
	defer botWrapperMu.Unlock()
	botWrappers = append(botWrappers, w)
}

// WrapBot 依次应用全部已注册的 BotWrapper，生成事件来源平台的 bot 外观。
func WrapBot(base bot.Bot, src Adapter) bot.Bot {
	botWrapperMu.RLock()
	defer botWrapperMu.RUnlock()
	for _, w := range botWrappers {
		base = w(base, src)
	}
	return base
}
