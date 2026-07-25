# 定时任务

框架内置基于 [robfig/cron/v3](https://github.com/robfig/cron) 的调度器，插件在 `StartCron` 钩子中注册任务。

## 基本用法

重写 `StartCron`，通过注入的 `CronManager` 添加任务：

```go
func (p *NewsPlugin) StartCron(ctx context.Context, b bot.Bot, c plugin.CronManager) error {
	_, err := c.AddFunc("0 18 * * *", func() {
		// 每天 18:00 执行
		p.sendNews(b)
	})
	return err
}
```

`AddFunc(spec, cmd)` 返回 `(cron.EntryID, error)`，`EntryID` 可用于后续移除（自行持有管理）。

## cron 表达式

标准 5 段格式：`分 时 日 月 周`

| 表达式 | 含义 |
| --- | --- |
| `0 18 * * *` | 每天 18:00 |
| `*/5 * * * *` | 每 5 分钟 |
| `0 9 * * 1-5` | 工作日 9:00 |
| `0 0 1 * *` | 每月 1 号 0:00 |

## 完整示例：整点报时插件

```go
package pluginhourly

import (
	"context"
	"fmt"
	"time"

	"github.com/jeanhua/AniaBot/common/bot"
	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/jeanhua/AniaBot/common/msgchain"
	"github.com/jeanhua/AniaBot/common/plugin"
	"github.com/jeanhua/AniaBot/common/plugininfo"
)

type hourlyConfig struct {
	Groups []int `cfg:"plugin.hourly.groups" label:"报时群列表" group:"整点报时" help:"每行一个群号"`
}

type HourlyPlugin struct {
	plugin.Meta
	cfg hourlyConfig // 由框架在 Start 前自动填充
}

func NewPlugin() *HourlyPlugin {
	return &HourlyPlugin{
		Meta: plugin.Meta{
			Name:      "整点报时插件",
			HelpWords: "整点在群里报时",
			Order:     plugin.LevelNormal,
			ShowFor:   plugininfo.ShowForGroup,
		},
	}
}

// ConfigSchema 声明配置结构体，框架自动注册面板字段并填充
func (p *HourlyPlugin) ConfigSchema() any { return &p.cfg }

// StartCron 注册定时任务
func (p *HourlyPlugin) StartCron(ctx context.Context, b bot.Bot, c plugin.CronManager) error {
	_, err := c.AddFunc("0 * * * *", func() {
		text := fmt.Sprintf("⏰ 现在是 %s", time.Now().Format("15:04"))
		for _, g := range p.cfg.Groups {
			chain := msgchain.Builder().Group()
			chain.Text(text)
			if _, ok := b.SendGroupMsg(message.QID(g), chain.Build()); !ok {
				p.Logger.Error("报时发送失败", "group", g)
			}
		}
	})
	return err
}
```

配置项（`plugin.hourly.groups`）会在 Web 控制面板「配置管理 → 整点报时」分组中自动出现，可视化编辑，改完重启生效。

## 注意事项

::: warning 任务函数中的 panic
cron 回调由框架调度执行。为安全起见，任务内避免可能 panic 的操作，或自行 recover；发送消息等操作建议检查返回值并记录日志。
:::

::: tip 静态 vs 动态任务
`StartCron` 适合**代码中预先定义好**的静态任务。如果你需要**用户/AI 在运行时动态创建**的任务，AI 对话插件的 clock 系统已实现完整方案（持久化 + 命令管理），可直接参考 `bot/plugins/pluginaichat/clock.go`，或在自己的插件中基于 `PersistentStorage` + cron 实现类似机制。
:::
