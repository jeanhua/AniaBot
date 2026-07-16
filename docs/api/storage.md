# 存储接口参考

AniaBot 通过 `common/storage` 提供两层独立的存储抽象：

- **缓存层（CACHE）**：`storage.Storage` 接口，易失、支持 TTL 与 Redis 列表语义，适合会话、队列、短期缓存。引擎为 `redis`（默认）或 `memory`。
- **持久化层（PERSISTENT）**：`storage.PersistentStorage` 接口，数据重启不丢失、**不支持** TTL/列表，适合长期留存的状态（插件配置、用户数据、积分、历史记录等）。引擎为 `sqlite`（默认）或 `mysql`。

两层互不干扰：写缓存不会落到持久化层，反之亦然。插件按需选择——只读写的、要跨重启保留的，用 `p.PersistentStorage`；要 TTL、要队列、可丢失的，用 `p.Storage`。

## 两层存储模型与依赖注入

| 层 | 接口 | 字段 | 语义 | 默认引擎 | 可选引擎 |
|----|------|------|------|---------|---------|
| 缓存 | `storage.Storage` | `p.Storage` | 易失、支持 TTL + 列表 | redis | memory |
| 持久化 | `storage.PersistentStorage` | `p.PersistentStorage` | 持久、无 TTL/列表 | sqlite | mysql |

框架在 `bot/core/core.go` 的 `Run()` 中完成两层初始化与注入：

```go
// 缓存层（默认 redis，可配置 memory）
cacheStore, err := newCacheStorage(ctx, cfg, logger)   // bot.store.cache.*

// 持久化层（默认 sqlite，可配置 mysql）
persistentStore, err := newPersistentStorage(ctx, cfg, logger)  // bot.store.persistent.*

// 注入：两层各自以 base64(插件名) 为基础命名空间 Clone 出独立子空间
for _, p := range ania.plugins {
    encodeName := base64.StdEncoding.EncodeToString([]byte(p.GetMeta().Name))
    p.SetStorage(ania.storage.Clone(encodeName))            // 缓存
    p.SetPersistentStorage(ania.persistent.Clone(encodeName))  // 持久化
    // ...其余 DI
}
```

因此插件内可直接使用，无需额外初始化：

```go
// 缓存（易失）
p.Storage.SetString(ctx, "session", token, storage.WithTTL(time.Hour))

// 持久化（重启不丢）
p.PersistentStorage.Set(ctx, "user:123", profile)
```

::: tip 两层各自独立
`p.Storage` 与 `p.PersistentStorage` 是两套完全独立的存储，键空间也不共享。需要跨重启保留的数据**必须**写入 `p.PersistentStorage`；写入 `p.Storage` 的数据在进程重启或缓存过期后即丢失。
:::

## 缓存存储接口 Storage

```go
type Storage interface {
    GetString(ctx context.Context, key string) (string, bool)
    SetString(ctx context.Context, key, val string, option ...Option) bool

    Get(ctx context.Context, key string, out any) bool
    Set(ctx context.Context, key string, val any, option ...Option) bool

    ScanKeys(ctx context.Context, pattern string, count int64) ([]string, error)

    Del(ctx context.Context, key string) bool
    Clear(ctx context.Context) bool

    Clone(prefix string) Storage

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

    Expire(ctx context.Context, key string, ttl time.Duration) bool
}
```

## 方法说明（缓存）

### GetString / SetString

字符串类型的读写操作。

```go
// 写入
ok := p.Storage.SetString(ctx, "key", "value")

// 读取，第二个返回值表示键是否存在
val, exists := p.Storage.GetString(ctx, "key")
if exists {
    fmt.Println(val)
}
```

**带选项写入**：

```go
// 设置 24 小时过期
p.Storage.SetString(ctx, "session", "data", storage.WithTTL(24*time.Hour))

// 仅在键不存在时写入（防重复）
p.Storage.SetString(ctx, "lock", "1", storage.WithCheckExist())
```

---

### Get / Set

任意类型的读写（内部通过 JSON 序列化）。

```go
type Config struct {
    Enabled bool
    Limit   int
}

// 写入
p.Storage.Set(ctx, "config", Config{Enabled: true, Limit: 100})

// 读取（out 必须是指针）
var cfg Config
if p.Storage.Get(ctx, "config", &cfg) {
    fmt.Println(cfg.Limit) // 100
}
```

**真实案例**：`groupnewsletter` 插件用 `Set` 存储消息列表，用 `Get` 恢复：

```go
// 保存
p.Storage.Set(ctx, "group:"+groupId, messages)

// 恢复
var msgs []GroupMessage
if p.Storage.Get(ctx, "group:"+groupId, &msgs) {
    p.groupMsgs[groupId] = msgs
}
```

::: warning Get 的 out 参数
`Get` 的第三个参数必须是指针类型，否则 JSON 反序列化会失败。传 `var out MyStruct` 是错误的，要传 `&out`。
:::

---

### ScanKeys

扫描匹配 pattern 的键（在插件命名空间内）。

```go
// 扫描所有 group: 前缀的键，最多返回 100 个
keys, err := p.Storage.ScanKeys(ctx, "group:*", 100)
if err != nil {
    p.Logger.Error("扫描失败", "error", err)
    return
}
for _, key := range keys {
    val, _ := p.Storage.GetString(ctx, key)
    fmt.Println(key, val)
}
```

::: warning
`ScanKeys` 返回的 key 是插件命名空间内的相对路径，可直接传给 `GetString` 等方法使用。
:::

**真实案例**：排行榜功能，扫描所有用户积分：

```go
prefix := fmt.Sprintf("score:%d:", msg.GroupId)
keys, _ := p.Storage.ScanKeys(ctx, prefix+"*", 100)
for _, key := range keys {
    val, _ := p.Storage.GetString(ctx, key)
    userId := strings.TrimPrefix(key, prefix)
    // 处理排行榜...
}
```

---

### Del / Clear

```go
// 删除单个键
p.Storage.Del(ctx, "key")

// 清空插件所有数据（谨慎使用！）
p.Storage.Clear(ctx)
```

::: danger Clear 的危险性
`Clear()` 会删除当前插件的**所有**存储数据，且不可恢复。生产环境中请谨慎使用。
:::

---

### 列表操作

Redis List 语义，适合消息队列、历史记录等场景。

```go
// 从右侧追加（最常用）
p.Storage.RPush(ctx, "history", "msg1", "msg2", "msg3")

// 从左侧弹出（先进先出）
val, ok := p.Storage.LPop(ctx, "queue")

// 范围读取（0 到 -1 表示全部）
items, ok := p.Storage.LRange(ctx, "history", 0, -1)

// 长度
n := p.Storage.LLen(ctx, "history")

// 修剪，只保留最近 100 条
p.Storage.LTrim(ctx, "history", -100, -1)
```

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

**实战：固定长度消息队列**（来自 `groupnewsletter` 插件）：

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

---

### Clone

创建带前缀的子存储空间，用于分类管理数据。

```go
// 为不同群创建独立的存储空间
groupStore := p.Storage.Clone(fmt.Sprintf("group:%d", msg.GroupId))
groupStore.SetString(ctx, "status", "active")
// 实际存储 key: <plugin_prefix>:group:123456:status

// 为不同用户创建独立的存储空间
userStore := p.Storage.Clone(fmt.Sprintf("user:%d", msg.Sender.UserId))
userStore.Set(ctx, "profile", userData)
// 实际存储 key: <plugin_prefix>:user:789:profile
```

::: tip Clone vs 手动拼接 key
`Clone` 创建的子空间可以复用，代码更清晰。如果你需要频繁操作同一类别的多个 key，用 `Clone` 比每次手动拼接 key 字符串更好。
:::

## Option 配置（缓存）

```go
type Option func(*StorageConfig)

type StorageConfig struct {
    TTL        time.Duration
    CheckExist bool
}
```

### WithTTL

设置键的过期时间（仅缓存引擎有效——TTL 属于缓存语义，持久化存储不支持 TTL）。

```go
// 设置 24 小时过期
p.Storage.SetString(ctx, "session", token, storage.WithTTL(24*time.Hour))

// 设置 3 天过期
p.Storage.SetString(ctx, "cache:"+url, content, storage.WithTTL(72*time.Hour))
```

### WithCheckExist

仅在键不存在时写入（类似 Redis `SETNX`）。

```go
// 防止重复签到
ok := p.Storage.SetString(ctx, "checkin:"+uid, "1",
    storage.WithTTL(24*time.Hour),
    storage.WithCheckExist(),
)
if !ok {
    // 键已存在，今天已签到
}
```

### 组合使用

`WithTTL` 和 `WithCheckExist` 可以组合使用：

```go
// 分布式锁：仅在不存在时写入，10 秒后自动过期
locked := p.Storage.SetString(ctx, "lock:task", "1",
    storage.WithTTL(10*time.Second),
    storage.WithCheckExist(),
)
if !locked {
    // 其他实例正在处理
    return
}
defer p.Storage.Del(ctx, "lock:task")
// 执行任务...
```

## 持久化存储接口 PersistentStorage

`storage.PersistentStorage` 是 AniaBot 的持久化存储抽象，数据在进程重启后不丢失，适合保存需要长期留存的状态：插件配置、用户数据、历史记录、积分等。

语义约定（与缓存 `Storage` 的关键差异）：

- **持久**：写入的数据在进程重启后依然存在。
- **不支持 TTL 与列表语义**——这些属于缓存范畴。如需保存有序数据，建议将 JSON 数组作为值整体读写（`Set` / `Get`）。
- **命名空间隔离与缓存一致**：`Clone` 创建带前缀的子空间；框架在注入时已用插件名做好基础隔离，插件内部再按需 `Clone`。
- **错误处理风格与缓存一致**：所有方法返回 `(value, bool)` 或 `bool`，错误内部记录日志后以 `false` 返回。

### 接口定义

```go
type PersistentStorage interface {
    // GetString 读取原始字符串值，第二个返回值表示键是否存在。
    GetString(ctx context.Context, key string) (string, bool)
    // SetString 写入原始字符串值（覆盖写）。
    SetString(ctx context.Context, key, val string) bool

    // Get 读取任意类型值（内部以 JSON 反序列化到 out，out 必须是指针）。
    Get(ctx context.Context, key string, out any) bool
    // Set 写入任意类型值（内部以 JSON 序列化）。
    Set(ctx context.Context, key string, val any) bool

    // Has 判断键是否存在。
    Has(ctx context.Context, key string) bool
    // Del 删除键。键不存在时仍返回 true（仅在发生错误时返回 false）。
    Del(ctx context.Context, key string) bool
    // Keys 列出当前命名空间下指定前缀的所有键（相对键，可直接回传给 Get 等方法）。
    Keys(ctx context.Context, prefix string) ([]string, error)
    // Clear 清空当前命名空间及其所有子命名空间的数据（谨慎使用）。
    Clear(ctx context.Context) bool

    // Clone 创建带前缀的子存储空间，用于分类管理数据。
    Clone(prefix string) PersistentStorage
}
```

### 方法说明（持久化）

#### GetString / SetString

原始字符串的读写（覆盖写，**无 TTL、无 `Option`**）。

```go
// 写入
ok := p.PersistentStorage.SetString(ctx, "bot:version", "1.0.0")

// 读取，第二个返回值表示键是否存在
val, exists := p.PersistentStorage.GetString(ctx, "bot:version")
if exists {
    fmt.Println(val) // 1.0.0
}
```

#### Get / Set（JSON）

任意类型的读写（内部以 JSON 序列化/反序列化），适合存结构化数据。

```go
type UserProfile struct {
    Name  string
    Score int
}

// 写入（内部 JSON 序列化）
ok := p.PersistentStorage.Set(ctx, "user:123", UserProfile{Name: "Alice", Score: 100})

// 读取（out 必须是指针）
var profile UserProfile
if p.PersistentStorage.Get(ctx, "user:123", &profile) {
    fmt.Println(profile.Name, profile.Score) // Alice 100
}
```

::: warning Get 的 out 参数
`Get` 的第三个参数必须是指针类型，否则 JSON 反序列化会失败。传 `var out MyStruct` 是错误的，要传 `&out`。这与缓存接口 `Storage.Get` 的要求一致。
:::

#### Has

判断键是否存在，比 `Get` 更轻量（不读取值、不反序列化）。

```go
if p.PersistentStorage.Has(ctx, "user:123") {
    // 已存在，走更新逻辑
} else {
    // 不存在，走初始化逻辑
}
```

#### Del

删除单个键。**键不存在时仍返回 `true`**，仅在发生错误（如数据库故障）时返回 `false`——因此不要用 `Del` 的返回值判断“键原本是否存在”，要判断存在性请用 `Has`。

```go
ok := p.PersistentStorage.Del(ctx, "user:123")
// ok=false 仅表示删除过程出错；键原本不存在时 ok 仍为 true
```

#### Keys

列出当前命名空间下匹配指定前缀的所有键，返回**相对键**，可直接回传给 `Get` / `GetString` / `Del` 等方法（与缓存 `ScanKeys` 的相对键语义一致，但 `Keys` 用前缀匹配而非 glob pattern，且不限制返回数量）。

```go
// 列出当前命名空间下所有 "user:" 前缀的键
keys, err := p.PersistentStorage.Keys(ctx, "user:")
if err != nil {
    p.Logger.Error("扫描持久化键失败", "error", err)
    return
}
for _, key := range keys {
    // keys 返回的是相对键，可直接回传给 Get
    var profile UserProfile
    if p.PersistentStorage.Get(ctx, key, &profile) {
        fmt.Println(key, profile.Name)
    }
}
```

::: tip 无需 pattern，传前缀即可
`Keys` 做的是字符串前缀匹配，不要像 `ScanKeys` 那样加 `*` 通配符。传 `"user:"` 就能列出所有以 `user:` 开头的键；传空串 `""` 列出当前命名空间下的全部键。
:::

#### Clear

清空**当前命名空间及其所有子命名空间**的数据（谨慎使用）。语义与缓存 `Clear` 一致，实现上对 SQL 后端做范围删除：`namespace >= lo AND namespace < prefixRangeEnd(lo)`，恰好覆盖当前命名空间及其所有以当前前缀为前缀的子命名空间。

::: danger Clear 的危险性
`Clear()` 会删除当前命名空间**及其所有子命名空间**的全部数据，且不可恢复。它不会只删当前这一层，而是连同 `Clone` 出去的所有子空间一起清掉。生产环境中请谨慎使用。
:::

#### Clone

创建带前缀的子存储空间，命名空间语义与缓存 `Storage.Clone` **完全一致**，便于在两层之间迁移代码。

```go
// 为不同群创建独立的持久化空间
groupStore := p.PersistentStorage.Clone(fmt.Sprintf("group:%d", msg.GroupId))
groupStore.Set(ctx, "config", groupCfg)
// 实际存储 key: <plugin_prefix>:group:123456:config

// 为不同用户创建独立的持久化空间
userStore := p.PersistentStorage.Clone(fmt.Sprintf("user:%d", msg.Sender.UserId))
userStore.Set(ctx, "profile", profile)
// 实际存储 key: <plugin_prefix>:user:789:profile
```

::: tip 两层 Clone 语义一致
缓存 `Storage.Clone` 与持久化 `PersistentStorage.Clone` 采用相同的命名空间拼接规则（`prefix + ":"`）。同一段按分类组织 key 的逻辑可以原样套用到任一层。
:::

## 存储引擎对比

两层各自有独立的引擎选择，下表分别说明。

### 缓存引擎（`storage.Storage`）

| 特性 | Redis 引擎 | 内存引擎 |
|------|-----------|---------|
| 配置方式 | `bot.store.cache.driver: redis`（默认） | `bot.store.cache.driver: memory` |
| 数据持久化 | 取决于 Redis 自身配置（默认视为易失；可开启 RDB/AOF 让 Redis 自身持久化，但作为缓存层仍按易失使用） | ❌ 进程退出即清空 |
| TTL 支持 | ✅ | ✅ |
| 列表语义 | ✅ | ✅ |
| 分布式共享 | ✅ 多实例共享 | ❌ 仅进程内 |
| 适用场景 | 生产环境 | 开发/测试、单机无 Redis |

### 持久化引擎（`storage.PersistentStorage`）

| 特性 | SQLite 引擎 | MySQL 引擎 |
|------|------------|-----------|
| 配置方式 | `bot.store.persistent.driver: sqlite`（默认） | `bot.store.persistent.driver: mysql` |
| 数据持久化 | ✅ 单文件持久化（WAL 模式） | ✅ 服务器持久化 |
| TTL 支持 | ❌（持久化层不支持 TTL） | ❌（持久化层不支持 TTL） |
| 列表语义 | ❌（持久化层不支持列表） | ❌（持久化层不支持列表） |
| 分布式共享 | ❌ 单文件，不跨实例 | ✅ 多实例共享 |
| 驱动 | `modernc.org/sqlite`（纯 Go，无 CGO） | `github.com/go-sql-driver/mysql`（纯 Go，无 CGO） |
| CGO 依赖 | ❌ 无 | ❌ 无 |
| 连接模式 | `MaxOpenConns(1)`，PRAGMA `WAL`/`busy_timeout=5000`/`synchronous=NORMAL`/`foreign_keys=ON` | 可配置连接池（`max_open_conns` 等） |
| 适用场景 | 单机/默认、跨平台交叉编译 | 多实例/生产、需共享存储 |

::: tip TTL 只属于缓存层
TTL 与列表语义是缓存范畴的能力，**仅缓存引擎有效（TTL 属于缓存语义，持久化存储不支持 TTL）**。需要 TTL 的数据写 `p.Storage`；需要重启不丢的数据写 `p.PersistentStorage`。
:::

## 配置示例

`config.yaml` 中 `bot.store` 下分 `cache` 与 `persistent` 两段配置（这是相对旧版 `bot.store.redis` 字符串的破坏性变更）：

```yaml
bot:
  store:
    # 缓存层（易失，支持 TTL/列表）
    cache:
      driver: redis            # redis（默认） | memory
      redis:
        address: "localhost:6379"
        password: ""
        db: 0
      # memory 引擎无需任何配置，进程内 map + RWMutex 实现

    # 持久化层（重启不丢失，无 TTL/列表）
    persistent:
      driver: sqlite           # sqlite（默认） | mysql
      sqlite:
        path: "./data/aniabot.db"   # 父目录不存在会自动创建；":memory:" 用于测试
      mysql:
        dsn: "user:password@tcp(localhost:3306)/aniabot?charset=utf8mb4&parseTime=true&loc=Local"
        max_open_conns: 20
        max_idle_conns: 5
        conn_max_lifetime_sec: 3600
```

::: warning 破坏性变更
旧版的 `bot.store.redis`（一个字符串地址）已废弃。升级后需改为 `bot.store.cache.redis.address`，并把需要持久化的数据迁移到 `p.PersistentStorage`。两个 SQL 后端共用同一张表 `ania_kv(namespace, key_name, val, updated_at)`，主键为 `(namespace, key_name)`，并在 `namespace` 上建索引。
:::

## 命名空间隔离

每层存储都按插件 `Name` 字段经 base64 编码后作为存储 key 的前缀，不同插件数据完全隔离，无需担心 key 冲突。缓存层与持久化层各自独立命名空间（互不共享），但隔离规则一致。

```
插件 A (Name="积分系统") 的 key "score:123"
  → 实际存储: c2vmlWnpo4rooqvlm777OnNjb3JlOjEyMw==

插件 B (Name="天气查询") 的 key "score:123"
  → 实际存储: dGlhbXF1ZXTmlLnmmI46c2NvcmU6MTIz

两个插件的 key 相同也不会冲突。
```

::: danger 重要
修改插件 `Name` 字段会导致存储前缀变化，原有数据将无法访问。生产环境中请谨慎修改插件名称。如果确实需要改名，需要先迁移旧数据（缓存层与持久化层都要迁）。
:::

## 常见用法总结

| 场景 | 推荐层 | 推荐方法 | 示例 |
|------|-------|---------|------|
| 存储用户积分（重启需保留） | 持久化 | `PersistentStorage.SetString` / `GetString` | `score:123` → `"100"` |
| 存储用户配置（重启需保留） | 持久化 | `PersistentStorage.Set` / `Get` (JSON) | `user:123` → `{Score:100}` |
| 枚举某类持久化键 | 持久化 | `PersistentStorage.Keys(prefix)` | 列出 `user:` 下所有键 |
| Per-group 持久化状态 | 持久化 | `PersistentStorage.Clone("group")` | 独立子空间 |
| 缓存 API 结果（可过期） | 缓存 | `SetString` + `WithTTL` | `cache:url` → content |
| 每日签到防重复 | 缓存 | `SetString` + `WithTTL` + `WithCheckExist` | 24h 过期 + SETNX |
| 消息历史记录（队列） | 缓存 | `RPush` + `LTrim` + `LRange` | 固定长度队列 |
| Per-group 缓存状态 | 缓存 | `Clone("group")` | 独立子空间 |
| 分布式锁 | 缓存 | `SetString` + `WithCheckExist` + `WithTTL` | 短期互斥锁 |
