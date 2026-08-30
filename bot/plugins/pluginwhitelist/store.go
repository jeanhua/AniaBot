package pluginwhitelist

import (
	"fmt"

	"github.com/jeanhua/AniaBot/bot/plugins/plugininterceptor"
)

// readList 从配置中心读一条字符串列表。
// 配置中心里的列表可能是 []string，也可能是 []any（JSON 解码后的形态），
// 两种都要能读；读不到或类型不符时返回空列表（等同「名单为空」）。
func (p *WhitelistPlugin) readList(key string) []string {
	if p.ConfigEditor == nil {
		return nil
	}
	v, ok := p.ConfigEditor.Get(key)
	if !ok || v == nil {
		return nil
	}
	switch items := v.(type) {
	case []string:
		return append([]string(nil), items...)
	case []any:
		out := make([]string, 0, len(items))
		for _, it := range items {
			if s, ok := it.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		p.Logger.Warn("名单配置类型异常，按空名单处理", "key", key, "type", fmt.Sprintf("%T", v))
		return nil
	}
}

// saveList 写回一条名单并立即刷新拦截判定用的共享状态。
// 先写配置（失败则不刷新，保持内存与持久层一致），再 reload。
func (p *WhitelistPlugin) saveList(key string, list []string) error {
	if p.ConfigEditor == nil {
		return fmt.Errorf("配置中心不可用（持久化存储异常？）")
	}
	if err := p.ConfigEditor.Set(key, list); err != nil {
		return err
	}
	p.reloadStore()
	return nil
}

// reloadStore 用配置中心的当前值重建 interceptor 的名单状态，使改动即时生效。
// 这是「无需 /reboot」的关键：interceptor 原先只在 Start 时读一次名单。
func (p *WhitelistPlugin) reloadStore() {
	if p.ConfigEditor == nil {
		return
	}
	enable := false
	if v, ok := p.ConfigEditor.Get(keyIntEnable); ok {
		if b, ok := v.(bool); ok {
			enable = b
		}
	}
	mode := ""
	if v, ok := p.ConfigEditor.Get(keyIntMode); ok {
		if s, ok := v.(string); ok {
			mode = s
		}
	}

	p.store.Load(enable, mode,
		p.readList(keyIntGroups),
		p.readList(keyIntFriends),
		p.readList(keyIntGroupUsers),
		func(rule string) { p.Logger.Warn("忽略非法的群内屏蔽成员规则", "rule", rule) })

	groups, friends, groupUsers := p.store.Counts()
	p.Logger.Info("名单已热重载",
		"enable", enable, "mode", p.store.Mode(),
		"groups", groups, "friends", friends, "groupUsers", groupUsers)
}

// 编译期断言：本插件依赖 interceptor 导出的共享名单存储
var _ = plugininterceptor.Store
