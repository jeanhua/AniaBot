# 数据存储

AniaBot 提供两层存储抽象，插件数据自动按插件名隔离，你不需要关心底层用的是什么数据库。

- **缓存层 `p.Storage`**：易失（volatile），支持 TTL 与列表语义。默认 Redis，可切换为进程内内存。适合热数据、临时会话、分布式锁、限流计数等"丢了能重建"的数据。
- **持久化层 `p.PersistentStorage`**：重启不丢失，无 TTL/列表语义。默认 SQLite，可切换为 MySQL。适合插件配置、用户积分、历史记录、长期状态等"必须留存"的数据。

两层独立配置、互不影响，命名空间隔离方式完全一致（框架以插件 `Name` 的 base64 编码作为前缀注入，详见[存储引擎](#存储引擎)一节的命名空间隔离说明）。

## 何时用哪一层

| 特征 | 缓存 `p.Storage` | 持久化 `p.PersistentStorage` |
|------|------------------|------------------------------|
| 数据性质 | 热数据 / 临时数据 | 需长期保存的数据 |
| 重启后 | 视为易失（Redis 自身的持久化不改变本层语义） | 不丢失 |
| TTL / 过期 | 支持 `WithTTL` | 不支持 |
| 列表语义 | 支持（`RPush`/`LRange` 等） | 不支持（用 JSON 数组整体读写代替） |
| 典型场景 | 缓存 API 结果、临时会话、分布式锁、限流计数器、每日签到标记 | 插件配置、用户积分、历史记录、长期状态 |
| 默认后端 | Redis | SQLite |

::: tip 选型原则
问自己一句："进程重启后这份数据必须还在吗？" 是 → 持久化层；否（或可重建）→ 缓存层。
:::

## 获取存储实例

插件通过 `plugin.Meta` 内嵌的 `Storage`（缓存）和 `PersistentStorage`（持久化）两个字段访问存储，框架在启动时已注入，无需额外初始化：

```go
func (p *MyPlugin) OnGroupMsg(ctx context.Context, b bot.Bot, cmd command.Command, msg message.Message) (bool, error) {
    // 缓存层
    p.Storage.SetString(ctx, "key", "value")
    val, ok := p.Storage.GetString(ctx, "key")

    // 持久化层
    p.PersistentStorage.SetString(ctx, "key", "value")
    pval, pok := p.PersistentStorage.GetString(ctx, "key")
    return true, nil
}
```

::: tip 什么时候用 ctx？
存储操作的 `ctx` 参数用于超时控制和取消。在消息处理方法中直接传入框架给的 `ctx` 即可。在后台 goroutine 中，可以用 `context.WithTimeout` 创建带超时的 context。
:::

## 缓存存储 p.Storage

下面各节均为缓存层 `p.Storage` 的用法，支持字符串、JSON、TTL、列表等语义。

### 缓存接口

```go
type Storage interface {
    // 字符串操作
    GetString(ctx context.Context, key string) (string, bool)
    SetString(ctx context.Context, key, val string, option ...Option) bool

    // 任意类型（JSON 序列化）
    Get(ctx context.Context, key string, out any) bool
    Set(ctx context.Context, key string, val any, option ...Option) bool

    // 键扫描
    ScanKeys(ctx context.Context, pattern string, count int64) ([]string, error)

    // 删除
    Del(ctx context.Context, key string) bool
    Clear(ctx context.Context) bool

    // 创建子存储空间
    Clone(prefix string) Storage

    // 列表操作
    LPush(ctx context.Context, key string, values ...any) int64
    RPush(ctx context.Context, key string, values ...any) int64
    LPop(ctx context.Context, key string) (any, bool)
    RPop(ctx context.Context, key string) (any, bool)
    LRange(ctx context.Context, key string, start, stop int64) ([]any, bool)
    LLen(ctx context.Context, key string) int64
    LRem(ctx context.Context, key string, count int64, value any) int64
    LSet(ctx context.Context, key string, index int64, value any) bool
    LIndex(ctx context.Context, key string, index int64) (any, bool)
    LTrim(ctx context.Context, key string, start, stop int64) bool
}
```

### 字符串操作：GetString / SetString

最基础的读写操作，适合存储简单的键值对：

```go
// 写入
p.Storage.SetString(ctx, "greeting", "hello world")

// 读取，第二个返回值表示键是否存在
val, exists := p.Storage.GetString(ctx, "greeting")
if exists {
    fmt.Println(val) // "hello world"
}
```

**真实案例**：积分系统存储用户分数：

```go
key := fmt.Sprintf("score:%d:%d", msg.GroupId, msg.Sender.UserId)
val, _ := p.Storage.GetString(ctx, key)
if val == "" {
    val = "0"
}
```

### JSON 操作：Get / Set

存储任意 Go 结构体，内部自动 JSON 序列化/反序列化：

```go
type UserData struct {
    Score int
    Level string
}

// 写入
data := UserData{Score: 100, Level: "gold"}
p.Storage.Set(ctx, "user:12345", data)

// 读取
var out UserData
if p.Storage.Get(ctx, "user:12345", &out) {
    fmt.Println(out.Score) // 100
}
```

::: tip 选择 String 还是 JSON？
- 只存一个值（计数器、开关、状态标记）→ 用 `SetString`
- 存储多个相关字段（用户信息、配置对象）→ 用 `Set`（JSON）
- 存储消息列表 → 用列表操作（`RPush`/`LRange`）
:::

### Option 配置

#### WithTTL — 设置过期时间

```go
// 设置 24 小时过期
storage.SetString(ctx, "session", "data", storage.WithTTL(24*time.Hour))
```

**适用场景**：缓存、临时会话、每日签到标记等。

#### WithCheckExist — 防止重复写入

```go
// 仅在键不存在时写入（类似 SETNX）
ok := storage.SetString(ctx, "init_flag", "1", storage.WithCheckExist())
if !ok {
    // 键已存在，写入失败
}
```

**适用场景**：每日签到、防止重复操作、分布式锁。

#### 组合使用

```go
// 每日签到：24 小时过期 + 防重复
dailyKey := fmt.Sprintf("checkin:%d:%d", msg.GroupId, msg.Sender.UserId)
if !p.Storage.SetString(ctx, dailyKey, "1",
    storage.WithTTL(24*time.Hour),
    storage.WithCheckExist(),
) {
    // 今天已经签到过了
    builder := msgchain.Builder().Group()
    builder.Text("今天已经签到过了！")
    b.SendGroupMsg(msg.GroupId, builder.Build())
    return false, nil
}
// 签到成功，加积分...
```

### 子存储空间

使用 `Clone` 创建带前缀的子空间，方便分类管理：

```go
groupStorage := p.Storage.Clone("group")
groupStorage.SetString(ctx, "123456", "active")  // 实际 key: <plugin_prefix>:group:123456

userStorage := p.Storage.Clone("user")
userStorage.SetString(ctx, "789", "data")         // 实际 key: <plugin_prefix>:user:789
```

::: tip 什么时候用 Clone？
当你需要按类别组织数据时（按群、按用户、按类型），Clone 比手动拼接 key 字符串更清晰，也更不容易出错。
:::

### 扫描键

```go
// 扫描所有以 "group:" 开头的键（在插件命名空间内）
keys, err := p.Storage.ScanKeys(ctx, "group:*", 100)
for _, key := range keys {
    val, _ := p.Storage.GetString(ctx, key)
    fmt.Println(key, val)
}
```

::: warning
`ScanKeys` 返回的 key 是插件命名空间内的相对路径，可直接传给 `GetString` 等方法使用。第二个参数 `count` 控制每次扫描的数量，不是返回结果的数量。
:::

### 列表操作

Redis List 语义，适合消息队列、历史记录等场景。

#### 基本操作

```go
// 从右侧追加
p.Storage.RPush(ctx, "history", "msg1", "msg2", "msg3")

// 从左侧弹出（先进先出）
val, ok := p.Storage.LPop(ctx, "queue")

// 读取全部（0 到 -1 表示全部）
items, ok := p.Storage.LRange(ctx, "history", 0, -1)

// 长度
n := p.Storage.LLen(ctx, "history")
```

#### 实战：固定长度的消息队列

这是 `groupnewsletter` 插件使用的模式——用 `RPush` 追加消息，用 `LTrim` 保持列表长度：

```go
// 保存消息，最多保留 500 条
func (p *MyPlugin) saveMessage(ctx context.Context, groupId int64, msg string) {
    key := fmt.Sprintf("msgs:%d", groupId)
    p.Storage.RPush(ctx, key, msg)        // 追加到末尾
    p.Storage.LTrim(ctx, key, -500, -1)   // 只保留最近 500 条
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

#### 列表操作速查

| 方法 | 说明 | 典型用途 |
|------|------|---------|
| `LPush` | 从左侧插入 | 栈（后进先出） |
| `RPush` | 从右侧插入 | 队列（先进先出） |
| `LPop` / `RPop` | 从左/右弹出 | 取出待处理任务 |
| `LRange` | 获取范围内元素 | 读取历史记录 |
| `LLen` | 获取列表长度 | 检查队列深度 |
| `LRem` | 移除指定值 | 删除特定消息 |
| `LSet` | 设置指定索引的值 | 修改历史记录 |
| `LIndex` | 获取指定索引的值 | 随机访问 |
| `LTrim` | 修剪列表 | 保持固定长度 |

## 持久化存储 p.PersistentStorage

持久化层用于保存重启后必须留存的数据。与缓存层不同，它**没有 TTL、没有列表语义**——所有方法都是普通键值读写；如需保存有序数据，建议把 JSON 数组作为值整体读写（`Set`/`Get`）。命名空间隔离与缓存层完全一致：框架已按插件名做好基础隔离，插件内部可再用 `Clone` 分类。

### 持久化接口

```go
type PersistentStorage interface {
    // 字符串操作
    GetString(ctx context.Context, key string) (string, bool)
    SetString(ctx context.Context, key, val string) bool

    // 任意类型（JSON 序列化）
    Get(ctx context.Context, key string, out any) bool   // out 必须是指针
    Set(ctx context.Context, key string, val any) bool

    // 键存在性 / 删除
    Has(ctx context.Context, key string) bool
    Del(ctx context.Context, key string) bool            // 键不存在时仍返回 true

    // 键扫描（返回相对键，可直接回传给 Get）
    Keys(ctx context.Context, prefix string) ([]string, error)

    // 清空当前命名空间及其所有子命名空间（谨慎使用）
    Clear(ctx context.Context) bool

    // 创建带前缀的子存储空间
    Clone(prefix string) PersistentStorage
}
```

### 实战：保存 per-group 插件配置

把每个群的自定义配置以 JSON 整体读写，key 用 `group:<群号>` 归类：

```go
type GroupConfig struct {
    Enabled     bool   `json:"enabled"`
    Prefix      string `json:"prefix"`
    WelcomeText string `json:"welcome_text"`
}

// 读取某群配置（不存在时返回默认值）
func (p *MyPlugin) loadGroupConfig(ctx context.Context, groupId int64) GroupConfig {
    var cfg GroupConfig
    key := fmt.Sprintf("group:%d", groupId)
    if p.PersistentStorage.Get(ctx, key, &cfg) {
        return cfg
    }
    // 未配置过，返回默认值
    return GroupConfig{Enabled: true, Prefix: "bot"}
}

// 写入 / 更新某群配置
func (p *MyPlugin) saveGroupConfig(ctx context.Context, groupId int64, cfg GroupConfig) {
    p.PersistentStorage.Set(ctx, fmt.Sprintf("group:%d", groupId), cfg)
}
```

### 扫描键：Keys

`Keys` 返回当前命名空间内匹配前缀的相对键，可直接回传给 `Get`。例如遍历所有群的配置：

```go
keys, err := p.PersistentStorage.Keys(ctx, "group:")
if err != nil {
    p.Logger.Error("扫描群配置失败", "error", err)
    return
}
for _, k := range keys {
    var cfg GroupConfig
    if p.PersistentStorage.Get(ctx, k, &cfg) {
        fmt.Println(k, cfg) // k 形如 "group:123456"
    }
}
```

::: tip 与缓存层 ScanKeys 的区别
缓存层用 `ScanKeys(ctx, pattern, count)`，pattern 支持 glob（如 `group:*`）且需指定 `count`；持久化层用 `Keys(ctx, prefix)`，prefix 是普通字符串前缀（如 `group:`），无需 count。两者返回的都是命名空间内的相对键。
:::

### 子存储空间：Clone

与缓存层用法一致，`Clone` 创建带前缀的子空间：

```go
groupStore := p.PersistentStorage.Clone("group")
groupStore.SetString(ctx, "123456", "active")  // 实际 key: <plugin_prefix>:group:123456
```

## 存储引擎

两层存储各自独立选择驱动，均通过 `bot.store` 配置，启动时由 `bot/core` 根据配置创建并注入。缓存默认 Redis，持久化默认 SQLite；两层均使用纯 Go 驱动，**无需 CGO**。初始化失败会记录日志后 `os.Exit(1)`（不再 panic）。

### config.yaml

```yaml
# config.yaml
bot:
  store:
    # 缓存存储：易失，支持 TTL 与列表语义，适合热数据/临时会话/分布式锁
    cache:
      driver: redis            # redis（默认，需 Redis 服务） | memory（进程内内存，零依赖，重启清空）
      redis:
        address: "localhost:6379"
        password: ""
        db: 0
      # memory 引擎无需任何配置

    # 持久化存储：重启不丢失，适合需要长期保存的数据（插件配置/用户数据/历史记录等）
    persistent:
      driver: sqlite           # sqlite（默认，纯 Go 无 CGO） | mysql
      sqlite:
        path: "./data/aniabot.db"   # 数据库文件路径，目录不存在会自动创建；":memory:" 表示纯内存（重启清空）
      mysql:
        dsn: "root:password@tcp(localhost:3306)/aniabot?charset=utf8mb4&parseTime=true&loc=Local"
        max_open_conns: 20          # 最大连接数，0 表示不限
        max_idle_conns: 5           # 最大空闲连接数
        conn_max_lifetime_sec: 300  # 连接最长存活时间（秒）
```

### 引擎对照

| 层 | 引擎 | 配置项 | 特点 |
|----|------|--------|------|
| 缓存 `p.Storage` | Redis（默认） | `bot.store.cache.driver: redis` + `bot.store.cache.redis.*` | 支持 TTL/列表，多实例共享，依赖外部 Redis 服务 |
| 缓存 `p.Storage` | 内存 | `bot.store.cache.driver: memory` | 轻量零依赖，进程内不共享，重启清空，适合开发测试 |
| 持久化 `p.PersistentStorage` | SQLite（默认） | `bot.store.persistent.driver: sqlite` + `bot.store.persistent.sqlite.path` | 单文件，纯 Go 无 CGO，WAL 模式，适合单机部署 |
| 持久化 `p.PersistentStorage` | MySQL | `bot.store.persistent.driver: mysql` + `bot.store.persistent.mysql.*` | 多实例共享，适合多机/高并发部署 |

::: warning 命名空间隔离
两层都以插件 `Name` 字段的 base64 编码作为存储前缀，不同插件的数据完全隔离。修改插件 `Name` 后，原有数据将无法访问，请在修改前迁移数据。该隔离机制对缓存层与持久化层完全一致。
:::

### 手动注入（escape hatch）

默认走配置即可。如需在代码中手动指定后端（例如测试、特殊部署），可用 `WithStorage` / `WithPersistentStorage` 选项覆盖配置——框架检测到已注入就不再读配置：

```go
import "github.com/jeanhua/AniaBot/bot/core"

// 缓存层：内存引擎
memCache := core.NewAniaMemoryStorage(logger)
// 持久化层：SQLite 纯内存（仅用于测试，重启清空）
memPersistent, err := core.NewAniaSqliteStorage(context.Background(), ":memory:", logger)
if err != nil {
    log.Fatal(err)
}

bot := core.NewAniaBot(adapter,
    core.WithStorage(memCache),
    core.WithPersistentStorage(memPersistent),
)
```

也可只覆盖其中一层，另一层仍走配置。相关构造器：`NewAniaMemoryStorage`（缓存/内存）、`NewAniaRedisStorage`（缓存/Redis）、`NewAniaSqliteStorage`（持久化/SQLite）、`NewAniaMysqlStorage`（持久化/MySQL）。

## 常见用法总结

| 场景 | 推荐层 / 方法 | 参考插件 |
|------|---------------|---------|
| 缓存 API 结果（带过期） | 缓存 `SetString` + `WithTTL` | urlparser |
| 每日签到防重复 | 缓存 `SetString` + `WithTTL` + `WithCheckExist` | 积分系统 |
| 临时会话 / 分布式锁 | 缓存 `SetString` + `WithCheckExist` + `WithTTL` | — |
| 消息历史（固定长度队列） | 缓存 `RPush` + `LTrim` + `LRange` | groupnewsletter |
| Per-group 易失状态 | 缓存 `Clone("group")` | gdmusicplugin |
| 用户积分（需重启留存） | 持久化 `SetString` / `GetString` | — |
| 插件配置 / 长期状态 | 持久化 `Set` / `Get`（JSON） | — |
| 长期历史记录 | 持久化 `Set`（JSON 数组整体读写） | — |

## 下一步

- [存储接口参考](../api/storage) — 完整的 API 文档
- [定时任务](./cron) — 定时清理过期数据
- [常见模式](./patterns) — 更多存储相关的设计模式
