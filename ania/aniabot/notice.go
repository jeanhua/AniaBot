package aniabot

import (
	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/jeanhua/AniaBot/common/plugin"
)

func (ania *AniaBot) onGroupUploadEvent(notice message.GroupUploadNotice) {
	for _, p := range ania.plugins {
		safeExecute("群文件上传事件", p, func(p plugin.Plugin) {
			p.OnGroupUpload(ania, notice)
		})
	}
}

func (ania *AniaBot) onGroupAdminEvent(notice message.GroupAdminNotice) {
	for _, p := range ania.plugins {
		safeExecute("群管理员变动事件", p, func(p plugin.Plugin) {
			p.OnGroupAdmin(ania, notice)
		})
	}
}

func (ania *AniaBot) onGroupDecreaseEvent(notice message.GroupDecreaseNotice) {
	for _, p := range ania.plugins {
		safeExecute("群成员减少事件", p, func(p plugin.Plugin) {
			p.OnGroupDecrease(ania, notice)
		})
	}
}

func (ania *AniaBot) onGroupIncreaseEvent(notice message.GroupIncreaseNotice) {
	for _, p := range ania.plugins {
		safeExecute("群成员增加事件", p, func(p plugin.Plugin) {
			p.OnGroupIncrease(ania, notice)
		})
	}
}

func (ania *AniaBot) onGroupBanEvent(notice message.GroupBanNotice) {
	for _, p := range ania.plugins {
		safeExecute("群禁言事件", p, func(p plugin.Plugin) {
			p.OnGroupBan(ania, notice)
		})
	}
}

func (ania *AniaBot) onFriendAddEvent(notice message.FriendAddNotice) {
	for _, p := range ania.plugins {
		safeExecute("新添加好友事件", p, func(p plugin.Plugin) {
			p.OnFriendAdd(ania, notice)
		})
	}
}

func (ania *AniaBot) onGroupRecallEvent(notice message.GroupRecallNotice) {
	for _, p := range ania.plugins {
		safeExecute("群消息撤回事件", p, func(p plugin.Plugin) {
			p.OnGroupRecall(ania, notice)
		})
	}
}

func (ania *AniaBot) onFriendRecallEvent(notice message.FriendRecallNotice) {
	for _, p := range ania.plugins {
		safeExecute("好友消息撤回事件", p, func(p plugin.Plugin) {
			p.OnFriendRecall(ania, notice)
		})
	}
}

func (ania *AniaBot) onPokeEvent(notice message.PokeNotice) {
	for _, p := range ania.plugins {
		safeExecute("戳一戳事件", p, func(p plugin.Plugin) {
			p.OnPoke(ania, notice)
		})
	}
}

func (ania *AniaBot) onLuckyKingEvent(notice message.LuckyKingNotice) {
	for _, p := range ania.plugins {
		safeExecute("运气王事件", p, func(p plugin.Plugin) {
			p.OnLuckyKing(ania, notice)
		})
	}
}

func (ania *AniaBot) onHonorEvent(notice message.HonorNotice) {
	for _, p := range ania.plugins {
		safeExecute("荣誉变更事件", p, func(p plugin.Plugin) {
			p.OnHonor(ania, notice)
		})
	}
}

func (ania *AniaBot) onGroupMsgEmojiLikeEvent(notice message.GroupMsgEmojiLikeNotice) {
	for _, p := range ania.plugins {
		safeExecute("群表情回应事件", p, func(p plugin.Plugin) {
			p.OnGroupMsgEmojiLike(ania, notice)
		})
	}
}

func (ania *AniaBot) onEssenceEvent(notice message.EssenceNotice) {
	for _, p := range ania.plugins {
		safeExecute("群精华事件", p, func(p plugin.Plugin) {
			p.OnEssence(ania, notice)
		})
	}
}

func (ania *AniaBot) onGroupCardEvent(notice message.GroupCardNotice) {
	for _, p := range ania.plugins {
		safeExecute("群名片变更事件", p, func(p plugin.Plugin) {
			p.OnGroupCard(ania, notice)
		})
	}
}
