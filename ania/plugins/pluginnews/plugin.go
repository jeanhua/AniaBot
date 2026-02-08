package pluginnews

import (
	"log"

	"github.com/jeanhua/AniaBot/common/bot"
	"github.com/jeanhua/AniaBot/common/msgchain"
	"github.com/jeanhua/AniaBot/common/plugin"
	"github.com/spf13/viper"
)

type NewsPlugin struct {
	plugin.Meta
	cronExpress string
	api         string
	groups      []uint
}

func NewNewsPlugin() *NewsPlugin {
	return &NewsPlugin{
		Meta: plugin.Meta{
			Name:      "每日新闻插件",
			HelpWords: "每日准点在指定群里新闻播报",
		},
	}
}

func (p *NewsPlugin) Start(cfg *viper.Viper) {
	p.cronExpress = cfg.GetString("plugin.dailyNews.cron")
	if p.cronExpress == "" {
		log.Println("读取daily news cron表达式错误")
	}
	p.api = cfg.GetString("plugin.dailyNews.api")
	if p.api == "" {
		log.Println("读取daily news api错误")
	}
	groups := cfg.GetIntSlice("plugin.dailyNews.groups")
	for _, g := range groups {
		log.Printf("播报群聊注册:%d", g)
		p.groups = append(p.groups, uint(g))
	}
}

func (p *NewsPlugin) StartCron(bot bot.Bot, c plugin.CronManager) {
	c.AddFunc(p.cronExpress, func() {
		for _, group := range p.groups {
			builder := msgchain.Builder.Group()
			builder.ImageUrl(p.api)
			_, ok := bot.SendGroupMsg(group, builder.Build())
			if ok {
				log.Printf("[发->群%d]: [每日新闻]", group)
			} else {
				log.Printf("[发->群%d]: [每日新闻] 发送失败...", group)
			}
		}
	})
}
