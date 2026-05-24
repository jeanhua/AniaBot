# 存储接口参考

AniaBot 通过 `common/storage` 提供统一的存储抽象，具体实现支持 Redis 和内存两种引擎。插件通过 `p.Storage` 直接访问，无需额外初始化。

## 接口定义

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
}
```

## 方法说明

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

## Option 配置

```go
type Option func(*StorageConfig)

type StorageConfig struct {
    TTL        time.Duration
    CheckExist bool
}
```

### WithTTL

设置键的过期时间（仅 Redis 引擎有效）。

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

## 存储引擎对比

| 特性 | Redis 引擎 | 内存引擎 |
|------|-----------|---------|
| 配置方式 | `config.yaml` 配置 `redis`（默认） | `WithStorage` 手动传入 |
| 数据持久化 | ✅ 重启不丢失 | ❌ 重启清空 |
| TTL 支持 | ✅ | ✅ |
| 分布式 | ✅ | ❌ |
| 适用场景 | 生产环境 | 开发/测试 |

## 命名空间隔离

每个插件的 `Name` 字段经 base64 编码后作为存储 key 的前缀，不同插件数据完全隔离，无需担心 key 冲突。

```
插件 A (Name="积分系统") 的 key "score:123"
  → 实际存储: c2vmlWnpo4rooqvlm777OnNjb3JlOjEyMw==

插件 B (Name="天气查询") 的 key "score:123"
  → 实际存储: dGlhbXF1ZXTmlLnmmI46c2NvcmU6MTIz

两个插件的 key 相同也不会冲突。
```

::: danger 重要
修改插件 `Name` 字段会导致存储前缀变化，原有数据将无法访问。生产环境中请谨慎修改插件名称。如果确实需要改名，需要先迁移旧数据。
:::

## 常见用法总结

| 场景 | 推荐方法 | 示例 |
|------|---------|------|
| 存储用户积分 | `SetString` / `GetString` | `score:123` → `"100"` |
| 存储用户配置 | `Set` / `Get` (JSON) | `user:123` → `{Score:100}` |
| 每日签到防重复 | `SetString` + `WithTTL` + `WithCheckExist` | 24h 过期 + SETNX |
| 缓存 API 结果 | `SetString` + `WithTTL` | `cache:url` → content |
| 消息历史记录 | `RPush` + `LTrim` + `LRange` | 固定长度队列 |
| Per-group 状态 | `Clone("group")` | 独立子空间 |
| 分布式锁 | `SetString` + `WithCheckExist` + `WithTTL` | 短期互斥锁 |
