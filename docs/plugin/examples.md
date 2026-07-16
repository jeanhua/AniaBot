# 完整示例

本页提供几个完整的插件实现，展示不同场景下的综合用法。每个示例都可以直接运行，也可以作为你开发插件的起点。

## 天气查询插件

一个完整的天气查询插件，展示命令解析、配置读取、错误处理的综合用法。

```go
package pluginweather

import (
    "context"
    "fmt"

    "github.com/jeanhua/AniaBot/common/bot"
    "github.com/jeanhua/AniaBot/common/model/command"
    "github.com/jeanhua/AniaBot/common/model/message"
    "github.com/jeanhua/AniaBot/common/msgchain"
    "github.com/jeanhua/AniaBot/common/plugin"
    "github.com/jeanhua/AniaBot/common/plugininfo"
    "github.com/spf13/viper"
)

type WeatherPlugin struct {
    plugin.Meta
    apiKey string
}

func NewPlugin() *WeatherPlugin {
    return &WeatherPlugin{
        Meta: plugin.Meta{
            Name:      "天气查询",
            HelpWords: "查询天气，用法：/weather <城市>",
            AdminOnly: false,
            ShowFor:   plugininfo.ShowForGroup | plugininfo.ShowForFriend,
            Author:    "jeanhua",
            Version:   "1.0.0",
            Order:     20,
        },
    }
}

func (p *WeatherPlugin) Start(ctx context.Context, cfg *viper.Viper) error {
    p.apiKey = cfg.GetString("plugin.weather.api_key")
    if p.apiKey == "" {
        return fmt.Errorf("plugin.weather.api_key 未配置")
    }
    return nil
}

func (p *WeatherPlugin) OnGroupMsg(ctx context.Context, b bot.Bot, cmd command.Command, msg message.Message) (bool, error) {
    if cmd.Name != "weather" {
        return true, nil
    }

    if len(cmd.Args) == 0 {
        builder := msgchain.Builder().Group()
        builder.Reply(msg.MessageId)
        builder.Text("请指定城市，例如：/weather 北京")
        b.SendGroupMsg(msg.GroupId, builder.Build())
        return false, nil
    }

    city := cmd.Args[0]
    info, err := p.queryWeather(city)
    if err != nil {
        builder := msgchain.Builder().Group()
        builder.Text(fmt.Sprintf("查询失败：%v", err))
        b.SendGroupMsg(msg.GroupId, builder.Build())
        return false, nil
    }

    builder := msgchain.Builder().Group()
    builder.Reply(msg.MessageId)
    builder.Text(fmt.Sprintf("📍 %s 天气\n%s", city, info))
    b.SendGroupMsg(msg.GroupId, builder.Build())

    return false, nil
}

func (p *WeatherPlugin) queryWeather(city string) (string, error) {
    // 调用天气 API...
    return "晴，25°C，东南风 3 级", nil
}
```

**config.yaml**：

```yaml
plugin:
  weather:
    api_key: "your-weather-api-key"
```

**要点**：
- `Start` 中读取配置，并做空值检查
- 参数校验：用户不传参数时给出用法提示
- 错误处理：API 调用失败时给出友好提示

---

## 积分系统插件

展示存储接口的综合用法：读写用户积分、每日签到防重复、排行榜查询。

```go
package pluginscore

import (
    "context"
    "fmt"
    "sort"
    "time"

    "github.com/jeanhua/AniaBot/common/bot"
    "github.com/jeanhua/AniaBot/common/model/command"
    "github.com/jeanhua/AniaBot/common/model/message"
    "github.com/jeanhua/AniaBot/common/msgchain"
    "github.com/jeanhua/AniaBot/common/plugin"
    "github.com/jeanhua/AniaBot/common/plugininfo"
    "github.com/jeanhua/AniaBot/common/storage"
)

type ScorePlugin struct {
    plugin.Meta
}

func NewPlugin() *ScorePlugin {
    return &ScorePlugin{
        Meta: plugin.Meta{
            Name:    "积分系统",
            HelpWords: "/score 查看积分 | /checkin 签到 | /rank 查看排行",
            ShowFor: plugininfo.ShowForGroup,
            Order:   30,
        },
    }
}

func (p *ScorePlugin) OnGroupMsg(ctx context.Context, b bot.Bot, cmd command.Command, msg message.Message) (bool, error) {
    switch cmd.Name {
    case "score":
        return p.handleScore(ctx, b, msg)
    case "checkin":
        return p.handleCheckin(ctx, b, msg)
    case "rank":
        return p.handleRank(ctx, b, msg)
    default:
        return true, nil
    }
}

// 查看积分
func (p *ScorePlugin) handleScore(ctx context.Context, b bot.Bot, msg message.Message) (bool, error) {
    key := fmt.Sprintf("score:%d:%d", msg.GroupId, msg.Sender.UserId)
    val, _ := p.Storage.GetString(ctx, key)
    if val == "" {
        val = "0"
    }
    builder := msgchain.Builder().Group()
    builder.Mention(msg.Sender.UserId)
    builder.Text(fmt.Sprintf(" 你的积分：%s", val))
    b.SendGroupMsg(msg.GroupId, builder.Build())
    return false, nil
}

// 每日签到
func (p *ScorePlugin) handleCheckin(ctx context.Context, b bot.Bot, msg message.Message) (bool, error) {
    dailyKey := fmt.Sprintf("checkin:%d:%d", msg.GroupId, msg.Sender.UserId)

    // WithCheckExist + WithTTL：24 小时内只能签到一次
    if !p.Storage.SetString(ctx, dailyKey, "1",
        storage.WithTTL(24*time.Hour),
        storage.WithCheckExist(),
    ) {
        builder := msgchain.Builder().Group()
        builder.Mention(msg.Sender.UserId)
        builder.Text(" 今天已经签到过了！")
        b.SendGroupMsg(msg.GroupId, builder.Build())
        return false, nil
    }

    // 加 10 分
    scoreKey := fmt.Sprintf("score:%d:%d", msg.GroupId, msg.Sender.UserId)
    // 这里简化处理，生产环境应该用原子操作
    val, _ := p.Storage.GetString(ctx, scoreKey)
    score := 10
    if val != "" {
        fmt.Sscanf(val, "%d", &score)
        score += 10
    }
    p.Storage.SetString(ctx, scoreKey, fmt.Sprintf("%d", score))

    builder := msgchain.Builder().Group()
    builder.Mention(msg.Sender.UserId)
    builder.Text(fmt.Sprintf(" 签到成功！+10 分，当前积分：%d", score))
    b.SendGroupMsg(msg.GroupId, builder.Build())
    return false, nil
}

// 排行榜
func (p *ScorePlugin) handleRank(ctx context.Context, b bot.Bot, msg message.Message) (bool, error) {
    prefix := fmt.Sprintf("score:%d:", msg.GroupId)
    keys, err := p.Storage.ScanKeys(ctx, prefix+"*", 100)
    if err != nil {
        return false, nil
    }

    type entry struct {
        userId string
        score  int
    }
    var entries []entry
    for _, key := range keys {
        val, _ := p.Storage.GetString(ctx, key)
        score := 0
        fmt.Sscanf(val, "%d", &score)
        userId := key[len(prefix):]
        entries = append(entries, entry{userId, score})
    }

    sort.Slice(entries, func(i, j int) bool {
        return entries[i].score > entries[j].score
    })

    strBuilder := &strings.Builder{}
    strBuilder.WriteString("📊 积分排行榜\n")
    for i, e := range entries {
        if i >= 10 {
            break
        }
        strBuilder.WriteString(fmt.Sprintf("%d. %s: %d 分\n", i+1, e.userId, e.score))
    }

    builder := msgchain.Builder().Group()
    builder.Text(strBuilder.String())
    b.SendGroupMsg(msg.GroupId, builder.Build())
    return false, nil
}
```

**要点**：
- `WithTTL` + `WithCheckExist` 组合实现每日签到防重复
- `ScanKeys` 扫描同前缀的键实现排行榜
- 多个子命令用 `switch` 分发

---

## Channel 任务队列插件

展示异步任务处理模式，适合需要调用外部 API 的场景。这个模式来自 `acgwallpaper` 和 `waifupics` 插件。

```go
package pluginfetcher

import (
    "context"
    "fmt"

    "github.com/jeanhua/AniaBot/common/bot"
    "github.com/jeanhua/AniaBot/common/model/command"
    "github.com/jeanhua/AniaBot/common/model/message"
    "github.com/jeanhua/AniaBot/common/msgchain"
    "github.com/jeanhua/AniaBot/common/plugin"
    "github.com/jeanhua/AniaBot/common/plugininfo"
)

type work struct {
    target  string
    userId  int64
    groupId int64
}

type FetcherPlugin struct {
    plugin.Meta
    pending chan work
}

func NewFetcherPlugin(queueSize int) *FetcherPlugin {
    return &FetcherPlugin{
        Meta: plugin.Meta{
            Name:      "数据获取",
            HelpWords: "发送 /fetch 获取数据",
            ShowFor:   plugininfo.ShowForGroup | plugininfo.ShowForFriend,
            Order:     plugin.LevelNormal,
        },
        pending: make(chan work, queueSize),
    }
}

// Awake 启动后台工作 goroutine
func (p *FetcherPlugin) Awake(ctx context.Context, b bot.Bot) error {
    b.Go("FetcherPlugin工作线程", func() {
        p.worker(b)
    })
    return nil
}

func (p *FetcherPlugin) OnGroupMsg(ctx context.Context, b bot.Bot, cmd command.Command, msg message.Message) (bool, error) {
    if cmd.Name != "fetch" {
        return true, nil
    }

    select {
    case p.pending <- work{target: "group", userId: msg.Sender.UserId, groupId: msg.GroupId}:
        // 任务已入队
        builder := msgchain.Builder().Group()
        builder.Reply(msg.MessageId)
        builder.Text("正在获取，请稍候...")
        b.SendGroupMsg(msg.GroupId, builder.Build())
    default:
        builder := msgchain.Builder().Group()
        builder.Reply(msg.MessageId)
        builder.Mention(msg.Sender.UserId)
        builder.Text(" 任务队列满了，请稍后再试")
        b.SendGroupMsg(msg.GroupId, builder.Build())
    }
    return false, nil
}

func (p *FetcherPlugin) OnFriendMsg(ctx context.Context, b bot.Bot, cmd command.Command, msg message.Message) (bool, error) {
    if cmd.Name != "fetch" {
        return true, nil
    }

    select {
    case p.pending <- work{target: "friend", userId: msg.Sender.UserId}:
        builder := msgchain.Builder().Friend()
        builder.Text("正在获取，请稍候...")
        b.SendFriendMsg(msg.Sender.UserId, builder.Build())
    default:
        builder := msgchain.Builder().Friend()
        builder.Text("任务队列满了，请稍后再试")
        b.SendFriendMsg(msg.Sender.UserId, builder.Build())
    }
    return false, nil
}

// worker 是后台消费循环
func (p *FetcherPlugin) worker(b bot.Bot) {
    for w := range p.pending {
        // 模拟耗时操作
        result, err := p.fetchData()
        if err != nil {
            p.Logger.Error("获取数据失败", "error", err)
            continue
        }

        builder := msgchain.Builder()
        switch w.target {
        case "group":
            builder.Group()
            builder.Text(result)
            b.SendGroupMsg(w.groupId, builder.Build())
        case "friend":
            builder.Friend()
            builder.Text(result)
            b.SendFriendMsg(w.userId, builder.Build())
        }
    }
}

func (p *FetcherPlugin) fetchData() (string, error) {
    // 调用外部 API...
    return "获取到的数据", nil
}
```

**要点**：
- 用带缓冲的 channel 作为任务队列，`Awake` 中启动消费 goroutine
- `select` + `default` 实现非阻塞投递，队列满时给用户提示
- `b.Go()` 启动的 goroutine 有 panic 恢复
- 群聊和私聊共用同一个 worker

---

## 交互式会话插件

展示多步交互模式，适合向导、表单、游戏等场景。这个模式来自 `chatrecordsmaker` 插件。

```go
package pluginwizard

import (
    "context"
    "fmt"
    "sync"
    "time"

    "github.com/jeanhua/AniaBot/common/bot"
    "github.com/jeanhua/AniaBot/common/model/command"
    "github.com/jeanhua/AniaBot/common/model/message"
    "github.com/jeanhua/AniaBot/common/msgchain"
    "github.com/jeanhua/AniaBot/common/plugin"
    "github.com/jeanhua/AniaBot/common/plugininfo"
)

type session struct {
    step     int
    name     string
    age      string
    created  time.Time
}

type WizardPlugin struct {
    plugin.Meta
    sessions map[int64]*session
    mu       sync.RWMutex
}

func NewWizardPlugin() *WizardPlugin {
    return &WizardPlugin{
        Meta: plugin.Meta{
            Name:      "信息收集",
            HelpWords: "发送 /start 开始填写信息",
            ShowFor:   plugininfo.ShowForFriend,
            Order:     plugin.LevelNormal,
        },
        sessions: make(map[int64]*session),
    }
}

func (p *WizardPlugin) Awake(ctx context.Context, b bot.Bot) error {
    // 定期清理超时会话
    b.Go("WizardPlugin会话清理", func() {
        ticker := time.NewTicker(5 * time.Minute)
        defer ticker.Stop()
        for range ticker.C {
            p.cleanExpiredSessions()
        }
    })
    return nil
}

func (p *WizardPlugin) OnFriendMsg(ctx context.Context, b bot.Bot, cmd command.Command, msg message.Message) (bool, error) {
    userId := msg.Sender.UserId

    p.mu.RLock()
    s, exists := p.sessions[userId]
    p.mu.RUnlock()

    // 没有进行中的会话，检查是否要开始
    if !exists {
        if cmd.Name != "start" {
            return true, nil
        }
        p.mu.Lock()
        p.sessions[userId] = &session{step: 1, created: time.Now()}
        p.mu.Unlock()

        builder := msgchain.Builder().Friend()
        builder.Text("欢迎！请输入你的名字：")
        b.SendFriendMsg(userId, builder.Build())
        return false, nil
    }

    // 处理取消命令
    if cmd.Name == "cancel" {
        p.mu.Lock()
        delete(p.sessions, userId)
        p.mu.Unlock()
        builder := msgchain.Builder().Friend()
        builder.Text("已取消。")
        b.SendFriendMsg(userId, builder.Build())
        return false, nil
    }

    // 根据当前步骤处理
    switch s.step {
    case 1:
        s.name = cmd.Name
        s.step = 2
        builder := msgchain.Builder().Friend()
        builder.Text(fmt.Sprintf("你好，%s！请输入你的年龄：", s.name))
        b.SendFriendMsg(userId, builder.Build())

    case 2:
        s.age = cmd.Name
        // 完成，输出结果
        builder := msgchain.Builder().Friend()
        builder.Text(fmt.Sprintf("收集完成！\n名字：%s\n年龄：%s", s.name, s.age))
        b.SendFriendMsg(userId, builder.Build())

        p.mu.Lock()
        delete(p.sessions, userId)
        p.mu.Unlock()
    }

    return false, nil
}

func (p *WizardPlugin) cleanExpiredSessions() {
    p.mu.Lock()
    defer p.mu.Unlock()
    deadline := time.Now().Add(-10 * time.Minute)
    for uid, s := range p.sessions {
        if s.created.Before(deadline) {
            delete(p.sessions, uid)
        }
    }
}
```

**要点**：
- `map[用户ID]*session` 追踪每个用户的对话状态
- `sync.RWMutex` 保护并发访问
- 定期清理超时会话，防止内存泄漏
- 支持 `/cancel` 取消当前会话

---

## 更多示例

查看 [dev/deploy 分支](https://github.com/jeanhua/AniaBot/tree/dev/deploy) 获取更多由社区开发的插件示例，包括：

- 音乐点歌（分页交互 + 文件发送）
- GitHub 仓库分析（LLM 集成 + Markdown 转图片）
- 群刊生成（消息收集 + 缓存持久化 + AI 生成）
- 消息拦截器（黑白名单 + 高优先级 Order）

也可以查看 [常见模式](./patterns) 了解更多设计模式。
