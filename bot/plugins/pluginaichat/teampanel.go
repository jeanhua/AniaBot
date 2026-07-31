package pluginaichat

import (
	"fmt"
	"strings"

	"github.com/jeanhua/AniaBot/common/plugininfo"
)

// 团队面板管理接口（实现 adminpanel.TeamSource）。
// 改动直接落盘，AI 工具每次调用实时读写存储，因此无需重启即生效。

// errTeamDisabled 团队功能未启用（plugin.ai_chat_bot.team.enable=false）时的统一错误。
var errTeamDisabled = fmt.Errorf("Agent 团队功能未启用")

// TeamScopes 返回已有团队的会话 scope 列表及各自数量（供 Web 面板展示）。
func (p *AIChatPlugin) TeamScopes() []plugininfo.TeamScopeInfo {
	if p.teamManager == nil {
		return nil
	}
	scopes := p.teamManager.scopes()
	infos := make([]plugininfo.TeamScopeInfo, 0, len(scopes))
	for _, scope := range scopes {
		info := plugininfo.TeamScopeInfo{
			Scope: scope,
			Count: len(p.teamManager.list(scope)),
		}
		if kind, target, ok := strings.Cut(scope, ":"); ok {
			info.Target = target
			if kind == "g" {
				info.Kind = "group"
			} else {
				info.Kind = "friend"
			}
		}
		infos = append(infos, info)
	}
	return infos
}

// TeamList 返回指定 scope 的全部团队（按团队名排序）。
func (p *AIChatPlugin) TeamList(scope string) ([]plugininfo.TeamInfo, error) {
	if p.teamManager == nil {
		return nil, errTeamDisabled
	}
	if !validScope(scope) {
		return nil, fmt.Errorf("非法的会话 scope: %s", scope)
	}
	defs := p.teamManager.list(scope)
	infos := make([]plugininfo.TeamInfo, 0, len(defs))
	for _, d := range defs {
		infos = append(infos, teamDefToInfo(d))
	}
	return infos, nil
}

// TeamCreate 在指定 scope 下新增一个团队。
func (p *AIChatPlugin) TeamCreate(up plugininfo.TeamUpsert) error {
	if p.teamManager == nil {
		return errTeamDisabled
	}
	if !validScope(up.Scope) {
		return fmt.Errorf("非法的会话 scope: %s", up.Scope)
	}
	if _, err := p.teamManager.create(up.Scope, up.Name, up.Desc, infoToTeamMembers(up.Members)); err != nil {
		return err
	}
	p.Logger.Info("团队已通过 Web 面板创建", "scope", up.Scope, "name", up.Name)
	return nil
}

// TeamUpdate 替换指定 scope 中一个团队的说明与成员。
func (p *AIChatPlugin) TeamUpdate(up plugininfo.TeamUpsert) error {
	if p.teamManager == nil {
		return errTeamDisabled
	}
	if !validScope(up.Scope) {
		return fmt.Errorf("非法的会话 scope: %s", up.Scope)
	}
	if err := p.teamManager.update(up.Scope, up.Name, up.Desc, infoToTeamMembers(up.Members)); err != nil {
		return err
	}
	p.Logger.Info("团队已通过 Web 面板更新", "scope", up.Scope, "name", up.Name)
	return nil
}

// TeamDelete 删除指定 scope 中的一个团队。
func (p *AIChatPlugin) TeamDelete(scope, name string) error {
	if p.teamManager == nil {
		return errTeamDisabled
	}
	if !validScope(scope) {
		return fmt.Errorf("非法的会话 scope: %s", scope)
	}
	if !p.teamManager.delete(scope, name) {
		return fmt.Errorf("团队不存在: %s", name)
	}
	p.Logger.Info("团队已通过 Web 面板删除", "scope", scope, "name", name)
	return nil
}

// teamDefToInfo 将内部团队定义转换为面板展示结构。
func teamDefToInfo(d teamDefinition) plugininfo.TeamInfo {
	info := plugininfo.TeamInfo{
		Name:      d.Name,
		Desc:      d.Desc,
		CreatedAt: d.CreatedAt,
	}
	info.Members = make([]plugininfo.TeamMemberInfo, 0, len(d.Members))
	for _, mem := range d.Members {
		info.Members = append(info.Members, plugininfo.TeamMemberInfo{Name: mem.Name, Role: mem.Role})
	}
	return info
}

// infoToTeamMembers 将面板传入的成员列表转换为内部定义。
func infoToTeamMembers(members []plugininfo.TeamMemberInfo) []teamMember {
	out := make([]teamMember, 0, len(members))
	for _, mem := range members {
		out = append(out, teamMember{
			Name: strings.TrimSpace(mem.Name),
			Role: strings.TrimSpace(mem.Role),
		})
	}
	return out
}
