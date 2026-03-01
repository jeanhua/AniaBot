// 随机活跃插件，本插件将随机在群里面活跃

package activeman

import (
	"context"
	"math/rand"
	"sync"
	"time"

	"github.com/jeanhua/AniaBot/common/bot"
	"github.com/jeanhua/AniaBot/common/model/command"
	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/jeanhua/AniaBot/common/plugin"
)

type ActiveMan struct {
	plugin.Meta

	// 点赞概率
	LikeProb float64
	// 戳一戳概率
	PokeProb float64
	// 签到概率
	SignProb float64
	// 每个群的签到状态
	groupSignState sync.Map // map[uint]*groupSignInfo
}

type groupSignInfo struct {
	signHour     int        // 签到小时 (0-23)
	signMinute   int        // 签到分钟 (0-59)
	lastSignDate string     // 上次签到日期 YYYY-MM-DD
	mu           sync.Mutex // 保护 lastSignDate
}

func (info *groupSignInfo) shouldSignToday() bool {
	info.mu.Lock()
	defer info.mu.Unlock()

	today := time.Now().Format("2006-01-02")
	if info.lastSignDate == today {
		return false
	}
	info.lastSignDate = today
	return true
}

func (info *groupSignInfo) isSignTime() bool {
	now := time.Now()
	return now.Hour() == info.signHour && now.Minute() == info.signMinute
}

func NewActiveMan(likeProb, pokeProb, signProb float64) *ActiveMan {
	return &ActiveMan{
		Meta: plugin.Meta{
			Name:      "随机活跃插件",
			HelpWords: "本插件将随机在群里面活跃，点赞、戳一戳和签到",
		},
		LikeProb: likeProb,
		PokeProb: pokeProb,
		SignProb: signProb,
	}
}

func (p *ActiveMan) StartCron(ctx context.Context, b bot.Bot, c plugin.CronManager) error {
	c.AddFunc("@every 1m", func() {
		p.checkAndSign(b)
	})
	return nil
}

func (p *ActiveMan) checkAndSign(b bot.Bot) {
	p.groupSignState.Range(func(key, value any) bool {
		groupId := key.(uint)
		info := value.(*groupSignInfo)

		if info.isSignTime() && info.shouldSignToday() {
			r := rand.New(rand.NewSource(time.Now().UnixNano()))
			if r.Float64() < p.SignProb {
				b.SendGroupSign(groupId)
				p.Logger.Printf("群签到 群:%d", groupId)
			}
		}
		return true
	})
}

func (p *ActiveMan) getOrCreateGroupSignInfo(groupId uint) *groupSignInfo {
	val, ok := p.groupSignState.Load(groupId)
	if ok {
		return val.(*groupSignInfo)
	}
	info := &groupSignInfo{
		signHour:   rand.Intn(24),
		signMinute: rand.Intn(60),
	}
	p.groupSignState.Store(groupId, info)
	return info
}

func (p *ActiveMan) OnGroupMsg(ctx context.Context, b bot.Bot, cmd command.Command, msg message.Message) (bool, error) {
	p.getOrCreateGroupSignInfo(msg.GroupId)

	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	if r.Float64() < p.LikeProb {
		emojiId := rand.Intn(200)
		b.SetMsgEmojiLike(msg.MessageId, emojiId, true)
		p.Logger.Printf("点赞消息 %d 表情 %d", msg.MessageId, emojiId)
	}
	if r.Float64() < p.PokeProb {
		b.SendPokeMsg(msg.Sender.UserId, &msg.GroupId)
		p.Logger.Printf("戳一戳消息 群:%d 戳:%d", msg.GroupId, msg.Sender.UserId)
	}

	return true, nil
}
