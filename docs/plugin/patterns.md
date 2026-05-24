# 常见模式

本页汇总了 AniaBot 插件开发中最常用的几种设计模式，每种都来自 `custom/plugins/` 中的真实插件。遇到具体场景时，可以对照选用。

## 1. Channel 任务队列

**适用场景**：需要异步处理的任务（HTTP 请求、图片生成、文件处理等），避免阻塞消息处理。

**原理**：用带缓冲的 channel 作为任务队列，消息处理函数只负责往 channel 里塞任务，后台 goroutine 负责消费和执行。

**真实案例**：`acgwallpaper`（二次元壁纸）、`waifupics`（waifu 图片）、`urlparser`（URL 解析）

```go
type MyPlugin struct {
    plugin.Meta
    pending chan work  // 任务队列
}

// work 是任务描述
type work struct {
    target  string  // "group" 或 "friend"
    userId  int64
    groupId int64
}

func NewMyPlugin(queueSize int) *MyPlugin {
    return &MyPlugin{
        Meta: plugin.Meta{
            Name:  "我的插件",
            Order: plugin.LevelNormal,
        },
        pending: make(chan work, queueSize), // 带缓冲的 channel
    }
}

// Awake 在 Bot 启动后开启后台工作 goroutine
func (p *MyPlugin) Awake(ctx context.Context, b bot.Bot) error {
    b.Go("MyPlugin工作线程", func() {
        p.worker(b)
    })
    return nil
}

// OnGroupMsg 只负责投递任务，不阻塞
func (p *MyPlugin) OnGroupMsg(ctx context.Context, b bot.Bot, cmd command.Command, msg message.Message) (bool, error) {
    if cmd.Name != "mycmd" {
        return true, nil
    }

    select {
    case p.pending <- work{target: "group", userId: msg.Sender.UserId, groupId: msg.GroupId}:
        // 任务已入队
    default:
        // 队列满了，告知用户
        builder := msgchain.Builder().Group()
        builder.Text("任务队列已满，请稍后再试")
        b.SendGroupMsg(msg.GroupId, builder.Build())
    }
    return false, nil
}

// worker 是后台消费循环
func (p *MyPlugin) worker(b bot.Bot) {
    for w := range p.pending {
        // 这里执行实际的耗时操作
        result := p.doHeavyWork()

        // 根据目标发送结果
        switch w.target {
        case "group":
            builder := msgchain.Builder().Group()
            builder.Text(result)
            b.SendGroupMsg(w.groupId, builder.Build())
        case "friend":
            builder := msgchain.Builder().Friend()
            builder.Text(result)
            b.SendFriendMsg(w.userId, builder.Build())
        }
    }
}
```

::: tip 为什么用 b.Go 而不是 go func()？
`b.Go()` 会为你的 goroutine 添加 panic 恢复和日志记录。如果 goroutine 崩溃了，框架会捕获异常并通知所有插件的 `OnPanic` 方法。直接用 `go func()` 的话，panic 会导致整个程序崩溃。
:::

## 2. 后台定时循环

**适用场景**：需要定期执行的任务（定时推送、状态检查、数据同步等）。

**原理**：在 `Awake` 中启动一个带 `select` 的无限循环，通过 `ctx.Done()` 控制退出。

**真实案例**：`groupnewsletter`（消息收集 + 定时生成）

```go
type MyPlugin struct {
    plugin.Meta
    pluginCtx context.Context
    cancel    context.CancelFunc
}

func (p *MyPlugin) Start(ctx context.Context, cfg *viper.Viper) error {
    // 创建插件自己的 context，不依赖框架传入的短生命周期 ctx
    p.pluginCtx, p.cancel = context.WithCancel(context.Background())
    return nil
}

func (p *MyPlugin) Stop() {
    if p.cancel != nil {
        p.cancel() // 停止所有后台循环
    }
}

func (p *MyPlugin) Awake(ctx context.Context, b bot.Bot) error {
    b.Go("MyPlugin后台循环", func() {
        p.backgroundLoop(b)
    })
    return nil
}

func (p *MyPlugin) backgroundLoop(b bot.Bot) {
    ticker := time.NewTicker(5 * time.Minute)
    defer ticker.Stop()

    for {
        select {
        case <-p.pluginCtx.Done():
            p.Logger.Info("后台循环退出")
            return
        case <-ticker.C:
            // 每 5 分钟执行一次
            p.doPeriodicTask(b)
        }
    }
}
```

::: warning 注意 context 的选择
框架传入的 `ctx` 在 `Start` 返回后可能就被取消了。如果你的后台任务需要长期运行，一定要创建自己的 `context.WithCancel`，并在 `Stop()` 中取消它。
:::

## 3. 交互式多步会话

**适用场景**：需要与用户进行多轮对话（向导、表单、游戏等）。

**原理**：用 `map[用户ID]*session` 追踪每个用户的对话状态，每条消息根据当前状态执行不同逻辑。

**真实案例**：`chatrecordsmaker`（聊天记录伪造）

```go
type MyPlugin struct {
    plugin.Meta
    sessions map[int64]*session  // 按用户 ID 追踪会话
    mu       sync.RWMutex
}

type session struct {
    step    int       // 当前步骤
    data    any       // 收集的数据
    created time.Time // 会话创建时间（用于超时清理）
}

func NewMyPlugin() *MyPlugin {
    return &MyPlugin{
        Meta: plugin.Meta{
            Name:  "向导插件",
            Order: plugin.LevelNormal,
        },
        sessions: make(map[int64]*session),
    }
}

func (p *MyPlugin) OnFriendMsg(ctx context.Context, b bot.Bot, cmd command.Command, msg message.Message) (bool, error) {
    userId := msg.Sender.UserId

    p.mu.RLock()
    s, exists := p.sessions[userId]
    p.mu.RUnlock()

    if !exists {
        // 没有进行中的会话，检查是否要开始新的
        if cmd.Name == "start" {
            p.mu.Lock()
            p.sessions[userId] = &session{step: 1, created: time.Now()}
            p.mu.Unlock()

            builder := msgchain.Builder().Friend()
            builder.Text("请输入你的名字：")
            b.SendFriendMsg(userId, builder.Build())
            return false, nil
        }
        return true, nil
    }

    // 有进行中的会话，根据步骤处理
    switch s.step {
    case 1:
        // 收集名字
        s.data = cmd.Name
        s.step = 2
        builder := msgchain.Builder().Friend()
        builder.Text("请输入你的年龄：")
        b.SendFriendMsg(userId, builder.Build())

    case 2:
        // 收集年龄，完成
        builder := msgchain.Builder().Friend()
        builder.Text(fmt.Sprintf("你好，%s！你今年 %s 岁。", s.data, cmd.Name))
        b.SendFriendMsg(userId, builder.Build())

        // 清理会话
        p.mu.Lock()
        delete(p.sessions, userId)
        p.mu.Unlock()
    }

    return false, nil
}
```

::: tip 会话超时清理
生产环境中，应该定期清理超时的会话（比如超过 10 分钟没响应就自动结束），避免内存泄漏。可以在 `Awake` 中启动一个定时清理 goroutine。
:::

## 4. LLM 集成

**适用场景**：需要 AI 能力的插件（文本生成、内容分析、智能摘要等）。

**原理**：使用 `openai-go` SDK 调用任意 OpenAI 兼容的 API。

**真实案例**：`githubrepoer`（GitHub 仓库分析）、`groupnewsletter`（群刊生成）、`urlparser`（URL 摘要）

```go
import "github.com/openai/openai-go/v3"

type MyPlugin struct {
    plugin.Meta
    client *openai.Client
    model  string
}

func (p *MyPlugin) Start(ctx context.Context, cfg *viper.Viper) error {
    baseURL := cfg.GetString("plugin.myplugin.base_url")
    apiKey := cfg.GetString("plugin.myplugin.api_key")
    p.model = cfg.GetString("plugin.myplugin.model")

    // 创建 OpenAI 兼容客户端
    p.client = openai.NewClient(
        option.WithBaseURL(baseURL),
        option.WithAPIKey(apiKey),
    )
    return nil
}

func (p *MyPlugin) askAI(prompt string) (string, error) {
    resp, err := p.client.Chat.Completions.New(context.Background(), openai.ChatCompletionNewParams{
        Model: p.model,
        Messages: []openai.ChatCompletionMessageParamUnion{
            openai.SystemMessage("你是一个有帮助的助手。"),
            openai.UserMessage(prompt),
        },
    })
    if err != nil {
        return "", err
    }
    return resp.Choices[0].Message.Content, nil
}
```

**config.yaml**：

```yaml
plugin:
  myplugin:
    base_url: "https://api.openai.com/v1"
    api_key: "sk-your-key"
    model: "gpt-4o-mini"
```

::: tip 兼容性
任何 OpenAI 兼容的 API 都可以使用，包括 DeepSeek、通义千问、本地部署的 Ollama 等。只需修改 `base_url` 和 `model`。
:::

## 5. 速率限制

**适用场景**：防止插件被频繁触发（API 调用限制、防刷等）。

**原理**：用 `atomic.Int64` 记录上次操作时间戳，每次触发时检查是否超过了冷却时间。

**真实案例**：`activeman`（随机活跃，每分钟最多一次操作）

```go
type MyPlugin struct {
    plugin.Meta
    lastActionTime atomic.Int64 // 上次操作的 Unix 时间戳
}

const cooldown = time.Minute // 冷却时间

func (p *MyPlugin) OnGroupMsg(ctx context.Context, b bot.Bot, cmd command.Command, msg message.Message) (bool, error) {
    now := time.Now().UnixNano()
    last := p.lastActionTime.Load()

    // 检查是否还在冷却期
    if now-last < int64(cooldown) {
        return true, nil // 冷却中，跳过
    }

    // CAS 操作，确保只有一个 goroutine 能成功
    if !p.lastActionTime.CompareAndSwap(last, now) {
        return true, nil // 被其他 goroutine 抢先了
    }

    // 执行操作...
    return true, nil
}
```

## 6. 黑白名单过滤

**适用场景**：按群号/用户号过滤消息，实现权限控制。

**原理**：在高优先级 Order 下检查消息来源，根据黑名单或白名单决定是否放行。

**真实案例**：`interceptor`（消息拦截器）

黑白名单是两种互斥的使用模式：

- **黑名单模式**：列出要屏蔽的群/用户，其余全部放行
- **白名单模式**：列出要放行的群/用户，其余全部拦截

一般不同时使用。下面是一个简化的实现：

```go
type MyPlugin struct {
    plugin.Meta
    blacklist []string
    whitelist []string
}

func NewMyPlugin() *MyPlugin {
    return &MyPlugin{
        Meta: plugin.Meta{
            Name:      "过滤器",
            AdminOnly: true,   // 对用户不可见
            ShowFor:   plugininfo.ShowForNone,
            Order:     plugin.LevelLog + 1, // 在日志之后、其他插件之前
        },
    }
}

func (p *MyPlugin) Start(ctx context.Context, cfg *viper.Viper) error {
    p.blacklist = cfg.GetStringSlice("plugin.filter.blacklist")
    p.whitelist = cfg.GetStringSlice("plugin.filter.whitelist")
    return nil
}

func (p *MyPlugin) OnGroupMsg(ctx context.Context, b bot.Bot, cmd command.Command, msg message.Message) (bool, error) {
    groupId := fmt.Sprint(msg.GroupId)

    // 黑名单模式：命中即拦截
    for _, id := range p.blacklist {
        if id == groupId || id == "all" {
            return false, nil // 拦截
        }
    }

    // 白名单模式：只放行白名单内的，其余全部拦截
    if len(p.whitelist) > 0 {
        for _, id := range p.whitelist {
            if id == groupId || id == "all" {
                return true, nil // 放行
            }
        }
        return false, nil // 不在白名单中，拦截
    }

    // 没有白名单 = 黑名单模式，未命中黑名单的全部放行
    return true, nil
}
```

## 7. Per-Group 状态管理

**适用场景**：需要为每个群维护独立状态（游戏进度、缓存、计数器等）。

**原理**：用 `sync.Map` 以群号为 key 存储状态，天然并发安全。

**真实案例**：`gdmusicplugin`（音乐搜索分页缓存）、`activeman`（签到时间管理）

```go
type MyPlugin struct {
    plugin.Meta
    groupState sync.Map // map[int64]*state
}

type state struct {
    mu       sync.Mutex
    count    int
    lastTime time.Time
}

func (p *MyPlugin) OnGroupMsg(ctx context.Context, b bot.Bot, cmd command.Command, msg message.Message) (bool, error) {
    // 获取或创建该群的状态
    val, _ := p.groupState.LoadOrStore(msg.GroupId, &state{})
    s := val.(*state)

    s.mu.Lock()
    defer s.mu.Unlock()

    s.count++
    // 使用 s.count 做逻辑...

    return true, nil
}
```

::: tip sync.Map vs map + RWMutex
- `sync.Map` 适合 key 集合稳定、读多写少的场景
- `map + RWMutex` 适合需要频繁遍历或删除的场景
- 对于大多数插件，`sync.Map` 已经足够
:::

## 8. 嵌入式资源文件

**适用场景**：插件需要附带模板、提示词、配置等文本文件。

**原理**：用 Go 的 `//go:embed` 指令将文件嵌入到二进制中，部署时无需额外携带文件。

**真实案例**：`groupnewsletter`（嵌入 LLM 提示词 prompt.md）

```go
import "embed"

//go:embed prompt.md
var defaultPrompt string

func (p *MyPlugin) Start(ctx context.Context, cfg *viper.Viper) error {
    // 优先使用配置中的 prompt，否则用嵌入的默认值
    prompt := cfg.GetString("plugin.myplugin.prompt")
    if prompt == "" {
        prompt = defaultPrompt
    }
    // 使用 prompt...
    return nil
}
```

**prompt.md**（放在插件同目录下）：

```markdown
你是一个有帮助的助手。
请根据用户的问题给出简洁明了的回答。
```

## 9. Redis 消息队列

**适用场景**：需要持久化消息列表（聊天记录、日志、历史数据等）。

**原理**：用 Storage 的 List 操作（RPush + LTrim）实现固定长度的消息队列。

**真实案例**：`groupnewsletter`（异步持久化群消息）

```go
// 将消息追加到列表末尾
func (p *MyPlugin) saveMessage(ctx context.Context, groupId int64, msg string) {
    key := fmt.Sprintf("msgs:%d", groupId)
    p.Storage.RPush(ctx, key, msg)        // 追加到末尾
    p.Storage.LTrim(ctx, key, -500, -1)   // 只保留最近 500 条
    // 设置 3 天过期（每次写入都刷新）
    p.Storage.SetString(ctx, key+":ts", time.Now().Format(time.RFC3339),
        storage.WithTTL(72*time.Hour))
}

// 读取所有消息
func (p *MyPlugin) loadMessages(ctx context.Context, groupId int64) []string {
    key := fmt.Sprintf("msgs:%d", groupId)
    items, ok := p.Storage.LRange(ctx, key, 0, -1)
    if !ok {
        return nil
    }
    result := make([]string, 0, len(items))
    for _, item := range items {
        if s, ok := item.(string); ok {
            result = append(result, s)
        }
    }
    return result
}
```

## 模式选择速查表

| 你的需求 | 推荐模式 | 参考插件 |
|---------|---------|---------|
| 异步处理耗时任务 | Channel 任务队列 | acgwallpaper |
| 定期执行任务 | 后台定时循环 | groupnewsletter |
| 多轮对话交互 | 交互式多步会话 | chatrecordsmaker |
| AI 文本生成/分析 | LLM 集成 | githubrepoer |
| 防止频繁触发 | 速率限制 | activeman |
| 按群/用户过滤 | 黑白名单过滤 | interceptor |
| 每个群独立状态 | Per-Group 状态管理 | gdmusicplugin |
| 附带模板/提示词 | 嵌入式资源文件 | groupnewsletter |
| 持久化消息列表 | Redis 消息队列 | groupnewsletter |

## 下一步

- [完整示例](./examples) — 更多完整的插件代码
- [数据存储](./storage) — 深入了解存储接口
- [定时任务](./cron) — Cron 表达式和定时任务
- [部署分支插件](../guide/deploy-plugins) — 查看所有真实插件的实现
