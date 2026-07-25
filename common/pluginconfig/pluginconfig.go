// Package pluginconfig 提供配置字段的注册机制。
//
// 插件通过实现 plugin.ConfigRegistrar 接口声明自己的配置字段
// （键、显示名、类型、分组、描述、默认值等元信息），框架在启动时收集并：
//  1. 将声明了默认值的键写入配置中心（仅当键不存在时，升级新增的键自动补齐）；
//  2. 让 Web 控制面板根据注册的元信息动态渲染表单——新增/移除插件
//     无需改动面板代码。
//
// 注册表是进程内结构，每次启动由框架与插件重新注册，不持久化。
package pluginconfig

import (
	"strings"
	"sync"
)

// Field 描述一个配置键的元信息。
type Field struct {
	Key       string `json:"key"`                 // 点分配置键，如 plugin.xxx.enable（统一小写）
	Label     string `json:"label"`               // 面板表单中的显示名
	Type      string `json:"type"`                // string | password | int | float | bool | text | strings | ints
	Group     string `json:"group"`               // 面板中的分组名
	Help      string `json:"help,omitempty"`      // 字段说明（可选）
	Sensitive bool   `json:"sensitive,omitempty"` // 敏感字段：面板不回显，API 掩码处理
	Default   any    `json:"-"`                   // 默认值：注册时若配置中心无此键则写入；nil 表示无默认值
}

var (
	mu     sync.RWMutex
	fields []Field
	index  = map[string]int{}
)

// Register 注册配置字段。键相同（不区分大小写）时后者原位覆盖前者，保持注册顺序。
func Register(fs ...Field) {
	mu.Lock()
	defer mu.Unlock()
	for _, f := range fs {
		f.Key = strings.ToLower(f.Key)
		if i, ok := index[f.Key]; ok {
			fields[i] = f
			continue
		}
		index[f.Key] = len(fields)
		fields = append(fields, f)
	}
}

// Fields 返回全部已注册字段（按注册顺序）。
func Fields() []Field {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]Field, len(fields))
	copy(out, fields)
	return out
}

// Defaults 返回所有声明了默认值的键值对（Default 为 nil 的键不包含）。
func Defaults() map[string]any {
	mu.RLock()
	defer mu.RUnlock()
	out := make(map[string]any, len(fields))
	for _, f := range fields {
		if f.Default != nil {
			out[f.Key] = f.Default
		}
	}
	return out
}
