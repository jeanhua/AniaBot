package aniabot

import (
	"log"

	"github.com/jeanhua/AniaBot/common/model/message"
)

func (ania *AniaBot) onGroupUploadEvent(notice message.GroupUploadNotice) {
	defer func() {
		if err := recover(); err != nil {
			log.Println("群文件上传消息事件触发错误: ", err)
		}
	}()
	for _, p := range ania.plugins {
		p.OnGroupUpload(ania, notice)
	}
}

func (ania *AniaBot) onGroupAdminEvent(notice message.GroupAdminNotice) {
	defer func() {
		if err := recover(); err != nil {
			log.Println("群管理变更消息事件触发错误: ", err)
		}
	}()
	for _, p := range ania.plugins {
		p.OnGroupAdmin(ania, notice)
	}
}

func (ania *AniaBot) onGroupDecreaseEvent(notice message.GroupDecreaseNotice) {
	defer func() {
		if err := recover(); err != nil {
			log.Println("群成员退群消息事件触发错误: ", err)
		}
	}()
	for _, p := range ania.plugins {
		p.OnGroupDecrease(ania, notice)
	}
}

func (ania *AniaBot) onGroupIncreaseEvent(notice message.GroupIncreaseNotice) {
	defer func() {
		if err := recover(); err != nil {
			log.Println("群成员加入消息事件触发错误: ", err)
		}
	}()
	for _, p := range ania.plugins {
		p.OnGroupIncrease(ania, notice)
	}
}

func (ania *AniaBot) onGroupBanEvent(notice message.GroupBanNotice) {
	defer func() {
		if err := recover(); err != nil {
			log.Println("群禁言消息事件触发错误: ", err)
		}
	}()
	for _, p := range ania.plugins {
		p.OnGroupBan(ania, notice)
	}
}

func (ania *AniaBot) onFriendAddEvent(notice message.FriendAddNotice) {
	defer func() {
		if err := recover(); err != nil {
			log.Println("好友添加消息事件触发错误: ", err)
		}
	}()
	for _, p := range ania.plugins {
		p.OnFriendAdd(ania, notice)
	}
}

func (ania *AniaBot) onGroupRecallEvent(notice message.GroupRecallNotice) {
	defer func() {
		if err := recover(); err != nil {
			log.Println("群消息撤回消息事件触发错误: ", err)
		}
	}()
	for _, p := range ania.plugins {
		p.OnGroupRecall(ania, notice)
	}
}

func (ania *AniaBot) onFriendRecallEvent(notice message.FriendRecallNotice) {
	defer func() {
		if err := recover(); err != nil {
			log.Println("好友消息撤回消息事件触发错误: ", err)
		}
	}()
	for _, p := range ania.plugins {
		p.OnFriendRecall(ania, notice)
	}
}

func (ania *AniaBot) onPokeEvent(notice message.PokeNotice) {
	defer func() {
		if err := recover(); err != nil {
			log.Println("戳一戳消息事件触发错误: ", err)
		}
	}()
	for _, p := range ania.plugins {
		p.OnPoke(ania, notice)
	}
}

func (ania *AniaBot) onLuckyKingEvent(notice message.LuckyKingNotice) {
	defer func() {
		if err := recover(); err != nil {
			log.Println("运气王消息事件触发错误: ", err)
		}
	}()
	for _, p := range ania.plugins {
		p.OnLuckyKing(ania, notice)
	}
}

func (ania *AniaBot) onHonorEvent(notice message.HonorNotice) {
	defer func() {
		if err := recover(); err != nil {
			log.Println("荣誉变更消息事件触发错误: ", err)
		}
	}()
	for _, p := range ania.plugins {
		p.OnHonor(ania, notice)
	}
}

func (ania *AniaBot) onGroupMsgEmojiLikeEvent(notice message.GroupMsgEmojiLikeNotice) {
	defer func() {
		if err := recover(); err != nil {
			log.Println("群表情回应消息事件触发错误: ", err)
		}
	}()
	for _, p := range ania.plugins {
		p.OnGroupMsgEmojiLike(ania, notice)
	}
}

func (ania *AniaBot) onEssenceEvent(notice message.EssenceNotice) {
	defer func() {
		if err := recover(); err != nil {
			log.Println("群精华消息事件触发错误: ", err)
		}
	}()
	for _, p := range ania.plugins {
		p.OnEssence(ania, notice)
	}
}

func (ania *AniaBot) onGroupCardEvent(notice message.GroupCardNotice) {
	defer func() {
		if err := recover(); err != nil {
			log.Println("群名片变更消息事件触发错误: ", err)
		}
	}()
	for _, p := range ania.plugins {
		p.OnGroupCard(ania, notice)
	}
}
