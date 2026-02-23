package core

import (
	"context"

	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/jeanhua/AniaBot/common/plugin"
)

func (ania *AniaBot) onGroupUploadEvent(notice message.GroupUploadNotice) {
	ctx := context.Background()
	for _, p := range ania.plugins {
		safeExecute("群文件上传事件", p, func(p plugin.Plugin) {
			noticeCtx, cancel := context.WithTimeout(ctx, NoticeEventTimeout)
			err := p.OnGroupUpload(noticeCtx, ania, notice)
			logError(err, p, "群文件上传事件")
			cancel()
		})
	}
}

func (ania *AniaBot) onGroupAdminEvent(notice message.GroupAdminNotice) {
	ctx := context.Background()
	for _, p := range ania.plugins {
		safeExecute("群管理员变动事件", p, func(p plugin.Plugin) {
			noticeCtx, cancel := context.WithTimeout(ctx, NoticeEventTimeout)
			err := p.OnGroupAdmin(noticeCtx, ania, notice)
			logError(err, p, "群管理员变动事件")
			cancel()
		})
	}
}

func (ania *AniaBot) onGroupDecreaseEvent(notice message.GroupDecreaseNotice) {
	ctx := context.Background()
	for _, p := range ania.plugins {
		safeExecute("群成员减少事件", p, func(p plugin.Plugin) {
			noticeCtx, cancel := context.WithTimeout(ctx, NoticeEventTimeout)
			err := p.OnGroupDecrease(noticeCtx, ania, notice)
			logError(err, p, "群成员减少事件")
			cancel()
		})
	}
}

func (ania *AniaBot) onGroupIncreaseEvent(notice message.GroupIncreaseNotice) {
	ctx := context.Background()
	for _, p := range ania.plugins {
		safeExecute("群成员增加事件", p, func(p plugin.Plugin) {
			noticeCtx, cancel := context.WithTimeout(ctx, NoticeEventTimeout)
			err := p.OnGroupIncrease(noticeCtx, ania, notice)
			logError(err, p, "群成员增加事件")
			cancel()
		})
	}
}

func (ania *AniaBot) onGroupBanEvent(notice message.GroupBanNotice) {
	ctx := context.Background()
	for _, p := range ania.plugins {
		safeExecute("群禁言事件", p, func(p plugin.Plugin) {
			noticeCtx, cancel := context.WithTimeout(ctx, NoticeEventTimeout)
			err := p.OnGroupBan(noticeCtx, ania, notice)
			logError(err, p, "群禁言事件")
			cancel()
		})
	}
}

func (ania *AniaBot) onFriendAddEvent(notice message.FriendAddNotice) {
	ctx := context.Background()
	for _, p := range ania.plugins {
		safeExecute("新添加好友事件", p, func(p plugin.Plugin) {
			noticeCtx, cancel := context.WithTimeout(ctx, NoticeEventTimeout)
			err := p.OnFriendAdd(noticeCtx, ania, notice)
			logError(err, p, "新添加好友事件")
			cancel()
		})
	}
}

func (ania *AniaBot) onGroupRecallEvent(notice message.GroupRecallNotice) {
	ctx := context.Background()
	for _, p := range ania.plugins {
		safeExecute("群消息撤回事件", p, func(p plugin.Plugin) {
			noticeCtx, cancel := context.WithTimeout(ctx, NoticeEventTimeout)
			err := p.OnGroupRecall(noticeCtx, ania, notice)
			logError(err, p, "群消息撤回事件")
			cancel()
		})
	}
}

func (ania *AniaBot) onFriendRecallEvent(notice message.FriendRecallNotice) {
	ctx := context.Background()
	for _, p := range ania.plugins {
		safeExecute("好友消息撤回事件", p, func(p plugin.Plugin) {
			noticeCtx, cancel := context.WithTimeout(ctx, NoticeEventTimeout)
			err := p.OnFriendRecall(noticeCtx, ania, notice)
			logError(err, p, "好友消息撤回事件")
			cancel()
		})
	}
}

func (ania *AniaBot) onPokeEvent(notice message.PokeNotice) {
	ctx := context.Background()
	for _, p := range ania.plugins {
		safeExecute("戳一戳事件", p, func(p plugin.Plugin) {
			noticeCtx, cancel := context.WithTimeout(ctx, NoticeEventTimeout)
			err := p.OnPoke(noticeCtx, ania, notice)
			logError(err, p, "戳一戳事件")
			cancel()
		})
	}
}

func (ania *AniaBot) onLuckyKingEvent(notice message.LuckyKingNotice) {
	ctx := context.Background()
	for _, p := range ania.plugins {
		safeExecute("运气王事件", p, func(p plugin.Plugin) {
			noticeCtx, cancel := context.WithTimeout(ctx, NoticeEventTimeout)
			err := p.OnLuckyKing(noticeCtx, ania, notice)
			logError(err, p, "运气王事件")
			cancel()
		})
	}
}

func (ania *AniaBot) onHonorEvent(notice message.HonorNotice) {
	ctx := context.Background()
	for _, p := range ania.plugins {
		safeExecute("荣誉变更事件", p, func(p plugin.Plugin) {
			noticeCtx, cancel := context.WithTimeout(ctx, NoticeEventTimeout)
			err := p.OnHonor(noticeCtx, ania, notice)
			logError(err, p, "荣誉变更事件")
			cancel()
		})
	}
}

func (ania *AniaBot) onGroupMsgEmojiLikeEvent(notice message.GroupMsgEmojiLikeNotice) {
	ctx := context.Background()
	for _, p := range ania.plugins {
		safeExecute("群表情回应事件", p, func(p plugin.Plugin) {
			noticeCtx, cancel := context.WithTimeout(ctx, NoticeEventTimeout)
			err := p.OnGroupMsgEmojiLike(noticeCtx, ania, notice)
			logError(err, p, "群表情回应事件")
			cancel()
		})
	}
}

func (ania *AniaBot) onEssenceEvent(notice message.EssenceNotice) {
	ctx := context.Background()
	for _, p := range ania.plugins {
		safeExecute("群精华事件", p, func(p plugin.Plugin) {
			noticeCtx, cancel := context.WithTimeout(ctx, NoticeEventTimeout)
			err := p.OnEssence(noticeCtx, ania, notice)
			logError(err, p, "群精华事件")
			cancel()
		})
	}
}

func (ania *AniaBot) onGroupCardEvent(notice message.GroupCardNotice) {
	ctx := context.Background()
	for _, p := range ania.plugins {
		safeExecute("群名片变更事件", p, func(p plugin.Plugin) {
			noticeCtx, cancel := context.WithTimeout(ctx, NoticeEventTimeout)
			err := p.OnGroupCard(noticeCtx, ania, notice)
			logError(err, p, "群名片变更事件")
			cancel()
		})
	}
}
