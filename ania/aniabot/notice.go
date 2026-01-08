package aniabot

import "github.com/jeanhua/AniaBot/common/model/message"

func (ania *AniaBot) onGroupUploadEvent(notice message.GroupUploadNotice) {
	for _, p := range ania.plugins {
		p.OnGroupUpload(ania, notice)
	}
}

func (ania *AniaBot) onGroupAdminEvent(notice message.GroupAdminNotice) {
	for _, p := range ania.plugins {
		p.OnGroupAdmin(ania, notice)
	}
}

func (ania *AniaBot) onGroupDecreaseEvent(notice message.GroupDecreaseNotice) {
	for _, p := range ania.plugins {
		p.OnGroupDecrease(ania, notice)
	}
}

func (ania *AniaBot) onGroupIncreaseEvent(notice message.GroupIncreaseNotice) {
	for _, p := range ania.plugins {
		p.OnGroupIncrease(ania, notice)
	}
}

func (ania *AniaBot) onGroupBanEvent(notice message.GroupBanNotice) {
	for _, p := range ania.plugins {
		p.OnGroupBan(ania, notice)
	}
}

func (ania *AniaBot) onFriendAddEvent(notice message.FriendAddNotice) {
	for _, p := range ania.plugins {
		p.OnFriendAdd(ania, notice)
	}
}

func (ania *AniaBot) onGroupRecallEvent(notice message.GroupRecallNotice) {
	for _, p := range ania.plugins {
		p.OnGroupRecall(ania, notice)
	}
}

func (ania *AniaBot) onFriendRecallEvent(notice message.FriendRecallNotice) {
	for _, p := range ania.plugins {
		p.OnFriendRecall(ania, notice)
	}
}

func (ania *AniaBot) onPokeEvent(notice message.PokeNotice) {
	for _, p := range ania.plugins {
		p.OnPoke(ania, notice)
	}
}

func (ania *AniaBot) onLuckyKingEvent(notice message.LuckyKingNotice) {
	for _, p := range ania.plugins {
		p.OnLuckyKing(ania, notice)
	}
}

func (ania *AniaBot) onHonorEvent(notice message.HonorNotice) {
	for _, p := range ania.plugins {
		p.OnHonor(ania, notice)
	}
}

func (ania *AniaBot) onGroupMsgEmojiLikeEvent(notice message.GroupMsgEmojiLikeNotice) {
	for _, p := range ania.plugins {
		p.OnGroupMsgEmojiLike(ania, notice)
	}
}

func (ania *AniaBot) onEssenceEvent(notice message.EssenceNotice) {
	for _, p := range ania.plugins {
		p.OnEssence(ania, notice)
	}
}

func (ania *AniaBot) onGroupCardEvent(notice message.GroupCardNotice) {
	for _, p := range ania.plugins {
		p.OnGroupCard(ania, notice)
	}
}
