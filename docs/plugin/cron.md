# 定时任务

AniaBot 基于 [robfig/cron](https://github.com/robfig/cron) 提供定时任务支持，通过实现 `StartCron` 方法注册。

## 注册定时任务

```go
func (p *MyPlugin) StartCron(ctx context.Context, b bot.Bot, c plugin.CronManager) error {
    // 每天 8:00 执行
    c.AddFunc("0 8 * * *", func() {
        builder := msgchain.Builder().Group()
        builder.Text("早上好！今天也要加油哦～")
        b.SendGroupMsg(123456789, builder.Build())
    })
    return nil
}
```

## Cron 表达式

AniaBot 使用标准 5 字段 Cron 表达式：

```
┌───── 分钟 (0-59)
│ ┌───── 小时 (0-23)
│ │ ┌───── 日 (1-31)
│ │ │ ┌───── 月 (1-12)
│ │ │ │ ┌───── 星期 (0-6, 0=周日)
│ │ │ │ │
* * * * *
```

| 表达式 | 说明 |
|--------|------|
| `0 8 * * *` | 每天 8:00 |
| `0 12 * * 1-5` | 工作日 12:00 |
| `*/30 * * * *` | 每 30 分钟 |
| `0 9 1 * *` | 每月 1 日 9:00 |

## 完整示例：每日推送

```go
type DailyPlugin struct {
    plugin.Meta
    groups []int64
}

func NewPlugin() *DailyPlugin {
    return &DailyPlugin{
        Meta: plugin.Meta{
            Name:  "每日推送",
            Order: 50,
        },
    }
}

func (p *DailyPlugin) Start(ctx context.Context, cfg *viper.Viper) error {
    // 从配置读取目标群列表
    p.groups = cfg.GetIntSlice("plugin.daily.groups")
    return nil
}

func (p *DailyPlugin) StartCron(ctx context.Context, b bot.Bot, c plugin.CronManager) error {
    c.AddFunc("0 8 * * *", func() {
        msg := p.getDailyMessage()
        for _, gid := range p.groups {
            builder := msgchain.Builder().Group()
            builder.Text(msg)
            b.SendGroupMsg(gid, builder.Build())
        }
    })
    return nil
}

func (p *DailyPlugin) getDailyMessage() string {
    return "今日份早报已送达！"
}
```

## 注意事项

- `StartCron` 在所有插件 `Start` 完成后调用
- 定时任务在独立 goroutine 中执行，注意并发安全
- 使用 `ctx` 判断是否需要提前退出：`ctx.Done()`
