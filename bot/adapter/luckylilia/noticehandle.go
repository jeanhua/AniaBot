package luckylilia

import (
	"encoding/json"
	"log"

	"github.com/jeanhua/AniaBot/common/adapter"
	"github.com/jeanhua/AniaBot/common/model/message"
)

type noticeParser func(data []byte) any
type noticeHandler func(notice any, trigger adapter.TriggerWrapper)

type noticeTypeRegistry struct {
	parser  noticeParser
	handler noticeHandler
}

// noticeRegistry 通知事件类型 → 解析/分发注册表。
// 各 parser 里逐字段调用 toLil：QID 反序列化对纯数字统一补 qq: 前缀，
// 解析后需替换为 lil:，保证跨适配器路由与自消息过滤一致。
var noticeRegistry = map[string]noticeTypeRegistry{
	"group_upload": {
		parser: func(data []byte) any {
			var notice message.GroupUploadNotice
			if err := json.Unmarshal(data, &notice); err != nil {
				return nil
			}
			notice.SelfId = toLil(notice.SelfId)
			notice.GroupId = toLil(notice.GroupId)
			notice.UserId = toLil(notice.UserId)
			notice.File.Id = toLil(notice.File.Id)
			return notice
		},
		handler: func(notice any, trigger adapter.TriggerWrapper) {
			if trigger.OnGroupUpload != nil {
				trigger.OnGroupUpload(notice.(message.GroupUploadNotice))
			}
		},
	},
	"group_admin": {
		parser: func(data []byte) any {
			var notice message.GroupAdminNotice
			if err := json.Unmarshal(data, &notice); err != nil {
				return nil
			}
			notice.SelfId = toLil(notice.SelfId)
			notice.GroupId = toLil(notice.GroupId)
			notice.UserId = toLil(notice.UserId)
			return notice
		},
		handler: func(notice any, trigger adapter.TriggerWrapper) {
			if trigger.OnGroupAdmin != nil {
				trigger.OnGroupAdmin(notice.(message.GroupAdminNotice))
			}
		},
	},
	"group_decrease": {
		parser: func(data []byte) any {
			var notice message.GroupDecreaseNotice
			if err := json.Unmarshal(data, &notice); err != nil {
				return nil
			}
			notice.SelfId = toLil(notice.SelfId)
			notice.GroupId = toLil(notice.GroupId)
			notice.OperatorId = toLil(notice.OperatorId)
			notice.UserId = toLil(notice.UserId)
			return notice
		},
		handler: func(notice any, trigger adapter.TriggerWrapper) {
			if trigger.OnGroupDecrease != nil {
				trigger.OnGroupDecrease(notice.(message.GroupDecreaseNotice))
			}
		},
	},
	"group_increase": {
		parser: func(data []byte) any {
			var notice message.GroupIncreaseNotice
			if err := json.Unmarshal(data, &notice); err != nil {
				return nil
			}
			notice.SelfId = toLil(notice.SelfId)
			notice.GroupId = toLil(notice.GroupId)
			notice.OperatorId = toLil(notice.OperatorId)
			notice.UserId = toLil(notice.UserId)
			return notice
		},
		handler: func(notice any, trigger adapter.TriggerWrapper) {
			if trigger.OnGroupIncrease != nil {
				trigger.OnGroupIncrease(notice.(message.GroupIncreaseNotice))
			}
		},
	},
	"group_ban": {
		parser: func(data []byte) any {
			var notice message.GroupBanNotice
			if err := json.Unmarshal(data, &notice); err != nil {
				return nil
			}
			notice.SelfId = toLil(notice.SelfId)
			notice.GroupId = toLil(notice.GroupId)
			notice.OperatorId = toLil(notice.OperatorId)
			notice.UserId = toLil(notice.UserId)
			return notice
		},
		handler: func(notice any, trigger adapter.TriggerWrapper) {
			if trigger.OnGroupBan != nil {
				trigger.OnGroupBan(notice.(message.GroupBanNotice))
			}
		},
	},
	"friend_add": {
		parser: func(data []byte) any {
			var notice message.FriendAddNotice
			if err := json.Unmarshal(data, &notice); err != nil {
				return nil
			}
			notice.SelfId = toLil(notice.SelfId)
			notice.UserId = toLil(notice.UserId)
			return notice
		},
		handler: func(notice any, trigger adapter.TriggerWrapper) {
			if trigger.OnFriendAdd != nil {
				trigger.OnFriendAdd(notice.(message.FriendAddNotice))
			}
		},
	},
	"group_recall": {
		parser: func(data []byte) any {
			var notice message.GroupRecallNotice
			if err := json.Unmarshal(data, &notice); err != nil {
				return nil
			}
			notice.SelfId = toLil(notice.SelfId)
			notice.GroupId = toLil(notice.GroupId)
			notice.UserId = toLil(notice.UserId)
			notice.OperatorId = toLil(notice.OperatorId)
			notice.MessageId = toLil(notice.MessageId)
			return notice
		},
		handler: func(notice any, trigger adapter.TriggerWrapper) {
			if trigger.OnGroupRecall != nil {
				trigger.OnGroupRecall(notice.(message.GroupRecallNotice))
			}
		},
	},
	"friend_recall": {
		parser: func(data []byte) any {
			var notice message.FriendRecallNotice
			if err := json.Unmarshal(data, &notice); err != nil {
				return nil
			}
			notice.SelfId = toLil(notice.SelfId)
			notice.UserId = toLil(notice.UserId)
			notice.MessageId = toLil(notice.MessageId)
			return notice
		},
		handler: func(notice any, trigger adapter.TriggerWrapper) {
			if trigger.OnFriendRecall != nil {
				trigger.OnFriendRecall(notice.(message.FriendRecallNotice))
			}
		},
	},
	"poke": {
		parser: func(data []byte) any {
			var notice message.PokeNotice
			if err := json.Unmarshal(data, &notice); err != nil {
				return nil
			}
			notice.SelfId = toLil(notice.SelfId)
			if notice.GroupId != nil {
				gid := toLil(*notice.GroupId)
				notice.GroupId = &gid
			}
			notice.UserId = toLil(notice.UserId)
			notice.TargetId = toLil(notice.TargetId)
			return notice
		},
		handler: func(notice any, trigger adapter.TriggerWrapper) {
			if trigger.OnPoke != nil {
				trigger.OnPoke(notice.(message.PokeNotice))
			}
		},
	},
	"lucky_king": {
		parser: func(data []byte) any {
			var notice message.LuckyKingNotice
			if err := json.Unmarshal(data, &notice); err != nil {
				return nil
			}
			notice.SelfId = toLil(notice.SelfId)
			notice.GroupId = toLil(notice.GroupId)
			notice.UserId = toLil(notice.UserId)
			notice.TargetId = toLil(notice.TargetId)
			return notice
		},
		handler: func(notice any, trigger adapter.TriggerWrapper) {
			if trigger.OnLuckyKing != nil {
				trigger.OnLuckyKing(notice.(message.LuckyKingNotice))
			}
		},
	},
	"honor": {
		parser: func(data []byte) any {
			var notice message.HonorNotice
			if err := json.Unmarshal(data, &notice); err != nil {
				return nil
			}
			notice.SelfId = toLil(notice.SelfId)
			notice.GroupId = toLil(notice.GroupId)
			notice.UserId = toLil(notice.UserId)
			return notice
		},
		handler: func(notice any, trigger adapter.TriggerWrapper) {
			if trigger.OnHonor != nil {
				trigger.OnHonor(notice.(message.HonorNotice))
			}
		},
	},
	"group_msg_emoji_like": {
		parser: func(data []byte) any {
			var notice message.GroupMsgEmojiLikeNotice
			if err := json.Unmarshal(data, &notice); err != nil {
				return nil
			}
			notice.SelfId = toLil(notice.SelfId)
			notice.GroupId = toLil(notice.GroupId)
			notice.UserId = toLil(notice.UserId)
			notice.MessageId = toLil(notice.MessageId)
			return notice
		},
		handler: func(notice any, trigger adapter.TriggerWrapper) {
			if trigger.OnGroupMsgEmojiLike != nil {
				trigger.OnGroupMsgEmojiLike(notice.(message.GroupMsgEmojiLikeNotice))
			}
		},
	},
	"essence": {
		parser: func(data []byte) any {
			var notice message.EssenceNotice
			if err := json.Unmarshal(data, &notice); err != nil {
				return nil
			}
			notice.SelfId = toLil(notice.SelfId)
			notice.GroupId = toLil(notice.GroupId)
			notice.MessageId = toLil(notice.MessageId)
			notice.SenderId = toLil(notice.SenderId)
			notice.OperatorId = toLil(notice.OperatorId)
			return notice
		},
		handler: func(notice any, trigger adapter.TriggerWrapper) {
			if trigger.OnEssence != nil {
				trigger.OnEssence(notice.(message.EssenceNotice))
			}
		},
	},
	"group_card": {
		parser: func(data []byte) any {
			var notice message.GroupCardNotice
			if err := json.Unmarshal(data, &notice); err != nil {
				return nil
			}
			notice.SelfId = toLil(notice.SelfId)
			notice.GroupId = toLil(notice.GroupId)
			notice.UserId = toLil(notice.UserId)
			return notice
		},
		handler: func(notice any, trigger adapter.TriggerWrapper) {
			if trigger.OnGroupCard != nil {
				trigger.OnGroupCard(notice.(message.GroupCardNotice))
			}
		},
	},
}

func handleNotice(trigger adapter.TriggerWrapper, noticeType string, data []byte) {
	registry, ok := noticeRegistry[noticeType]
	if !ok {
		log.Println("未知的通知类型: ", noticeType)
		return
	}

	notice := registry.parser(data)
	if notice == nil {
		log.Printf("解析通知事件[%s]错误\n", noticeType)
		return
	}

	registry.handler(notice, trigger)
}
