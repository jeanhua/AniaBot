package pluginwhitelist

import (
	"fmt"
	"strings"

	"github.com/jeanhua/AniaBot/common/model/command"
	"github.com/jeanhua/AniaBot/common/model/message"
)

const helpText = `白名单管理 /wl 用法：
/wl status                查看当前名单状态
/wl list                  列出群名单与用户名单
/wl add [群号|用户ID]      加入名单（群里不带参数=加入本群）
/wl del [群号|用户ID]      移出名单（群里不带参数=移出本群）
/wl mode whitelist|blacklist   切换名单模式
/wl on | /wl off          启用 / 关闭名单功能
改动立即生效，无需 /reboot。ID 可写 123456（默认 QQ）或带前缀如 tg:-100123`

// handleCommand 处理 /wl 子命令，返回要回复的文本。
// curID 为当前会话 ID（群号或对方用户 ID），用于 add/del 不带参数时的默认目标。
func (p *WhitelistPlugin) handleCommand(cmd command.Command, curID message.QID, isGroup bool) string {
	if len(cmd.Args) == 0 {
		return p.statusText() + "\n\n发送 /wl help 查看用法"
	}

	switch strings.ToLower(cmd.Args[0]) {
	case "help", "h", "?":
		return helpText
	case "status", "st":
		return p.statusText()
	case "list", "ls":
		return p.listText()
	case "add":
		return p.mutate(cmd.Args[1:], curID, isGroup, true)
	case "del", "rm", "remove":
		return p.mutate(cmd.Args[1:], curID, isGroup, false)
	case "mode":
		if len(cmd.Args) < 2 {
			return "用法：/wl mode whitelist|blacklist"
		}
		return p.setMode(strings.ToLower(cmd.Args[1]))
	case "on", "enable":
		return p.setEnabled(true)
	case "off", "disable":
		return p.setEnabled(false)
	default:
		return "未知的子命令：" + cmd.Args[0] + "\n\n" + helpText
	}
}

// mutate 增删名单。target 为空时用当前会话（群里=本群，私聊=对方）。
// 群聊 ID 与用户 ID 分表存放，据 isGroup 与显式参数判断落到哪张表：
// 显式给了 ID 时无法从 ID 本身判断是群还是人，因此沿用「命令发出的场景」
// ——在群里执行就操作群名单，私聊里执行就操作用户名单。
func (p *WhitelistPlugin) mutate(args []string, curID message.QID, isGroup bool, add bool) string {
	target := curID
	if len(args) > 0 && strings.TrimSpace(args[0]) != "" {
		target = message.FromString(strings.TrimSpace(args[0]))
	}

	key := keyIntFriends
	kind := "用户"
	if isGroup {
		key = keyIntGroups
		kind = "群"
	}

	list := p.readList(key)
	idx := indexOfID(list, target)
	switch {
	case add && idx >= 0:
		return fmt.Sprintf("%s %s 已经在名单里了", kind, target)
	case !add && idx < 0:
		return fmt.Sprintf("%s %s 不在名单里", kind, target)
	case add:
		list = append(list, target.String())
	default:
		list = append(list[:idx], list[idx+1:]...)
	}

	if err := p.saveList(key, list); err != nil {
		return "写入配置失败：" + err.Error()
	}
	action := "已加入"
	if !add {
		action = "已移出"
	}
	return fmt.Sprintf("%s %s %s名单（当前共 %d 条）", action, kind, target, len(list))
}

// setMode 切换名单模式
func (p *WhitelistPlugin) setMode(mode string) string {
	if mode != "whitelist" && mode != "blacklist" {
		return "模式只能是 whitelist 或 blacklist"
	}
	if p.ConfigEditor == nil {
		return "配置中心不可用，无法修改模式"
	}
	if err := p.ConfigEditor.Set(keyIntMode, mode); err != nil {
		return "写入配置失败：" + err.Error()
	}
	p.reloadStore()
	return "名单模式已切换为 " + mode + "\n\n" + p.statusText()
}

// setEnabled 启用/关闭名单功能
func (p *WhitelistPlugin) setEnabled(on bool) string {
	if p.ConfigEditor == nil {
		return "配置中心不可用，无法修改开关"
	}
	if err := p.ConfigEditor.Set(keyIntEnable, on); err != nil {
		return "写入配置失败：" + err.Error()
	}
	p.reloadStore()
	if on {
		return "名单功能已启用\n\n" + p.statusText()
	}
	return "名单功能已关闭，当前放行全部会话"
}

// listText 列出两张名单的内容
func (p *WhitelistPlugin) listText() string {
	groups := p.readList(keyIntGroups)
	friends := p.readList(keyIntFriends)
	rules := p.readList(keyIntGroupUsers)

	var sb strings.Builder
	sb.WriteString(p.statusText())
	sb.WriteString("\n\n群名单：")
	sb.WriteString(joinOrNone(groups))
	sb.WriteString("\n用户名单：")
	sb.WriteString(joinOrNone(friends))
	if len(rules) > 0 {
		sb.WriteString("\n群内屏蔽规则：")
		sb.WriteString(joinOrNone(rules))
	}
	return sb.String()
}

func joinOrNone(items []string) string {
	if len(items) == 0 {
		return "（空）"
	}
	return "\n  " + strings.Join(items, "\n  ")
}

// indexOfID 在名单中查找 ID（按规范化后的 QID 比较，容忍写法差异
// 如 123456 与 qq:123456）；未找到返回 -1。
func indexOfID(list []string, target message.QID) int {
	for i, item := range list {
		if message.FromString(strings.TrimSpace(item)) == target {
			return i
		}
	}
	return -1
}
