package feishu

import (
	"context"
	"time"

	"github.com/jeanhua/AniaBot/common/model/message"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

// GetGroupList 实现 adapter.ContactsExt：分页拉取机器人所在的群聊列表。
// ListChat 条目不含成员数（MemberCount 留 0，面板显示「—」）；
// 生产环境该接口只返回群聊（p2p 单聊仅 UAT 可拉），仍按 ChatMode 防御性过滤。
func (a *feishuAdapter) GetGroupList() (*[]message.GroupInfo, bool) {
	if a.client == nil {
		return nil, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	out := make([]message.GroupInfo, 0, 32)
	pageToken := ""
	for {
		builder := larkim.NewListChatReqBuilder().
			UserIdType("open_id").
			SortType("ByCreateTimeAsc").
			PageSize(100)
		if pageToken != "" {
			builder.PageToken(pageToken)
		}
		resp, err := a.client.Im.V1.Chat.List(ctx, builder.Build())
		if err != nil || resp == nil || !resp.Success() || resp.Data == nil {
			a.logger.Warn("飞书获取群列表失败", "error", err)
			return nil, false
		}
		for _, c := range resp.Data.Items {
			if c == nil || c.ChatId == nil {
				continue
			}
			if c.ChatMode != nil && *c.ChatMode == "p2p" {
				continue
			}
			info := message.GroupInfo{GroupID: message.QID(idPrefix + *c.ChatId)}
			if c.Name != nil {
				info.GroupName = *c.Name
			}
			out = append(out, info)
		}
		if resp.Data.HasMore == nil || !*resp.Data.HasMore || resp.Data.PageToken == nil {
			break
		}
		pageToken = *resp.Data.PageToken
	}
	return &out, true
}

// GetFriendList 实现 adapter.ContactsExt：飞书机器人无好友概念——
// p2p 单聊列表仅 UAT 环境可拉取、企业通讯录 API 需租户级权限，均不适用，
// 按接口约定返回空列表。
func (a *feishuAdapter) GetFriendList() (*[]message.Friend, bool) {
	return &[]message.Friend{}, true
}
