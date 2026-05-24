# 定时任务

AniaBot 基于 [robfig/cron](https://github.com/robfig/cron) 提供定时任务支持，通过实现 `StartCron` 方法注册。定时任务在独立 goroutine 中执行，适合定时推送、状态检查、数据清理等场景。

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

AniaBot 使用标准 5 字段 Cron 表达式，格式如下：

| 位置 | 字段 | 范围 | 说明 |
|:----:|------|------|------|
| 第 1 位 | 分钟 | 0-59 | 几分执行 |
| 第 2 位 | 小时 | 0-23 | 几点执行 |
| 第 3 位 | 日 | 1-31 | 几号执行 |
| 第 4 位 | 月 | 1-12 | 几月执行 |
| 第 5 位 | 星期 | 0-6 | 周几执行（0=周日） |

例如 `0 8 * * 1-5` 表示「工作日每天 8:00」。

常用表达式速查：

| 表达式 | 说明 |
|--------|------|
| `0 8 * * *` | 每天 8:00 |
| `0 12 * * 1-5` | 工作日 12:00 |
| `*/30 * * * *` | 每 30 分钟 |
| `0 9 1 * *` | 每月 1 日 9:00 |
| `0 0 * * 0` | 每周日 0:00 |
| `0 8,12,18 * * *` | 每天 8:00、12:00、18:00 |
| `@every 1h` | 每隔 1 小时（非标准语法，robfig/cron 扩展） |
| `@every 5m` | 每隔 5 分钟 |

::: tip @every 语法
`@every 1h` 表示从 Bot 启动开始，每隔 1 小时执行一次。这比标准 Cron 表达式更直观，适合间隔执行的场景。
:::

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

## Cron + 存储：签到提醒

定时任务常与存储配合使用。比如每天检查哪些群需要签到提醒：

```go
func (p *MyPlugin) StartCron(ctx context.Context, b bot.Bot, c plugin.CronManager) error {
    c.AddFunc("0 9 * * *", func() {
        // 扫描所有需要提醒的群
        keys, err := p.Storage.ScanKeys(ctx, "remind:*", 100)
        if err != nil {
            p.Logger.Error("扫描提醒键失败", "error", err)
            return
        }
        for _, key := range keys {
            val, _ := p.Storage.GetString(ctx, key)
            // key 格式: remind:<groupId>
            // val 格式: 提醒内容
            groupIdStr := strings.TrimPrefix(key, "remind:")
            // 发送提醒...
        }
    })
    return nil
}
```

## 随机定时任务

有些场景需要「每天随机时间执行」。`activeman` 插件使用了一种巧妙的模式：

```go
// 来自 custom/plugins/activeman/plugin.go

// 每分钟检查一次，但只在预定时间执行
func (p *ActiveMan) StartCron(ctx context.Context, b bot.Bot, c plugin.CronManager) error {
    c.AddFunc("@every 1m", func() {
        now := time.Now()
        p.groupSignState.Range(func(key, value any) bool {
            groupId := key.(int64)
            state := value.(*groupSignInfo)

            state.mu.Lock()
            defer state.mu.Unlock()

            // 检查是否到了预定的签到时间
            if now.After(state.nextSignTime) {
                // 执行签到...
                b.SendGroupMsg(groupId, buildMsg)

                // 设置明天的随机签到时间（1~25 小时后）
                hours := time.Duration(1+rand.Intn(25)) * time.Hour
                state.nextSignTime = now.Add(hours)
            }
            return true
        })
    })
    return nil
}
```

::: tip 设计思路
与其为每个群创建独立的定时任务，不如用一个统一的 `@every 1m` 循环，配合内存中的状态表来判断是否该执行。这样更灵活，也更容易管理。
:::

## 定时任务 + 后台循环

有些插件同时使用 `StartCron` 和 `Awake` 中的后台循环：

- `StartCron`：用于按固定时间触发的任务（如每天 8:00 推送）
- `Awake` + 后台循环：用于持续运行的任务（如消息收集、状态监控）

```go
func (p *MyPlugin) StartCron(ctx context.Context, b bot.Bot, c plugin.CronManager) error {
    // 定时任务：每天 8:00 推送
    c.AddFunc("0 8 * * *", func() {
        p.doDailyPush(b)
    })
    return nil
}

func (p *MyPlugin) Awake(ctx context.Context, b bot.Bot) error {
    // 后台循环：持续监控
    b.Go("MyPlugin监控线程", func() {
        p.monitorLoop(b)
    })
    return nil
}
```

## 注意事项

### 执行顺序

`StartCron` 在**所有**插件的 `Start` 完成后才调用。这意味着你在 `Start` 中初始化的资源，在 `StartCron` 中一定可以安全使用。

### 并发安全

定时任务在独立 goroutine 中执行。如果你的任务会修改共享状态（如 `map`），需要加锁或使用 `sync.Map`。

```go
// ❌ 不安全：直接操作 map
func (p *MyPlugin) StartCron(ctx context.Context, b bot.Bot, c plugin.CronManager) error {
    c.AddFunc("@every 1m", func() {
        p.data["key"] = "value" // 可能有并发问题
    })
    return nil
}

// ✅ 安全：使用 sync.Map 或加锁
func (p *MyPlugin) StartCron(ctx context.Context, b bot.Bot, c plugin.CronManager) error {
    c.AddFunc("@every 1m", func() {
        p.data.Store("key", "value") // sync.Map，天然并发安全
    })
    return nil
}
```

### 错误处理

定时任务中的 panic 会被框架捕获并通知所有插件的 `OnPanic` 方法，不会导致整个程序崩溃。但最好还是自己处理错误：

```go
c.AddFunc("0 8 * * *", func() {
    if err := p.doWork(); err != nil {
        p.Logger.Error("定时任务执行失败", "error", err)
        // 可以选择通知管理员
    }
})
```

### 上下文取消

如果插件实现了 `Stop()` 方法，可以在其中取消正在运行的定时任务。但通常不需要，因为 cron 库会在 Bot 关闭时自动停止所有任务。

## 下一步

- [数据存储](./storage) — 定时任务中常用存储来持久化状态
- [常见模式](./patterns) — 更多设计模式
- [内置插件](../guide/builtin-plugins) — 查看 pluginnews 的定时推送实现
