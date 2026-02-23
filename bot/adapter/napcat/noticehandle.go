package napcat

import (
	"encoding/json"
	"log"

	"github.com/jeanhua/AniaBot/common/adapter"
	"github.com/jeanhua/AniaBot/common/model/message"
)

func handleNotice(trigger adapter.TriggerWrapper, noticeType string, data []byte) {
	switch noticeType {
	case "group_upload":
		var notice message.GroupUploadNotice
		if err := json.Unmarshal(data, &notice); err != nil {
			log.Println("解析消息通知事件[group_upload]错误: ", err.Error())
			return
		}
		if trigger.OnGroupUpload != nil {
			trigger.OnGroupUpload(notice)
		}

	case "group_admin":
		var notice message.GroupAdminNotice
		if err := json.Unmarshal(data, &notice); err != nil {
			log.Println("解析消息通知事件[group_admin]错误: ", err.Error())
			return
		}
		if trigger.OnGroupAdmin != nil {
			trigger.OnGroupAdmin(notice)
		}

	case "group_decrease":
		var notice message.GroupDecreaseNotice
		if err := json.Unmarshal(data, &notice); err != nil {
			log.Println("解析消息通知事件[group_decrease]错误: ", err.Error())
			return
		}
		if trigger.OnGroupDecrease != nil {
			trigger.OnGroupDecrease(notice)
		}

	case "group_increase":
		var notice message.GroupIncreaseNotice
		if err := json.Unmarshal(data, &notice); err != nil {
			log.Println("解析消息通知事件[group_increase]错误: ", err.Error())
			return
		}
		if trigger.OnGroupIncrease != nil {
			trigger.OnGroupIncrease(notice)
		}

	case "group_ban":
		var notice message.GroupBanNotice
		if err := json.Unmarshal(data, &notice); err != nil {
			log.Println("解析消息通知事件[group_ban]错误: ", err.Error())
			return
		}
		if trigger.OnGroupBan != nil {
			trigger.OnGroupBan(notice)
		}

	case "friend_add":
		var notice message.FriendAddNotice
		if err := json.Unmarshal(data, &notice); err != nil {
			log.Println("解析消息通知事件[friend_add]错误: ", err.Error())
			return
		}
		if trigger.OnFriendAdd != nil {
			trigger.OnFriendAdd(notice)
		}

	case "group_recall":
		var notice message.GroupRecallNotice
		if err := json.Unmarshal(data, &notice); err != nil {
			log.Println("解析消息通知事件[group_recall]错误: ", err.Error())
			return
		}
		if trigger.OnGroupRecall != nil {
			trigger.OnGroupRecall(notice)
		}

	case "friend_recall":
		var notice message.FriendRecallNotice
		if err := json.Unmarshal(data, &notice); err != nil {
			log.Println("解析消息通知事件[friend_recall]错误: ", err.Error())
			return
		}
		if trigger.OnFriendRecall != nil {
			trigger.OnFriendRecall(notice)
		}

	case "poke":
		var notice message.PokeNotice
		if err := json.Unmarshal(data, &notice); err != nil {
			log.Println("解析消息通知事件[poke]错误: ", err.Error())
			return
		}
		if trigger.OnPoke != nil {
			trigger.OnPoke(notice)
		}

	case "lucky_king":
		var notice message.LuckyKingNotice
		if err := json.Unmarshal(data, &notice); err != nil {
			log.Println("解析消息通知事件[lucky_king]错误: ", err.Error())
			return
		}
		if trigger.OnLuckyKing != nil {
			trigger.OnLuckyKing(notice)
		}

	case "honor":
		var notice message.HonorNotice
		if err := json.Unmarshal(data, &notice); err != nil {
			log.Println("解析消息通知事件[honor]错误: ", err.Error())
			return
		}
		if trigger.OnHonor != nil {
			trigger.OnHonor(notice)
		}

	case "group_msg_emoji_like":
		var notice message.GroupMsgEmojiLikeNotice
		if err := json.Unmarshal(data, &notice); err != nil {
			log.Println("解析消息通知事件[group_msg_emoji_like]错误: ", err.Error())
			return
		}
		if trigger.OnGroupMsgEmojiLike != nil {
			trigger.OnGroupMsgEmojiLike(notice)
		}

	case "essence":
		var notice message.EssenceNotice
		if err := json.Unmarshal(data, &notice); err != nil {
			log.Println("解析消息通知事件[essence]错误: ", err.Error())
			return
		}
		if trigger.OnEssence != nil {
			trigger.OnEssence(notice)
		}

	case "group_card":
		var notice message.GroupCardNotice
		if err := json.Unmarshal(data, &notice); err != nil {
			log.Println("解析消息通知事件[group_card]错误: ", err.Error())
			return
		}
		if trigger.OnGroupCard != nil {
			trigger.OnGroupCard(notice)
		}

	default:
		log.Println("未知的通知类型: ", noticeType)
		return
	}
}
