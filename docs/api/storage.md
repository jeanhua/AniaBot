# 存储接口参考

AniaBot 通过 `common/storage` 提供统一的存储抽象，具体实现支持 Redis 和内存两种引擎。

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
```

### Get / Set

任意类型的读写（内部通过 JSON 序列化）。

```go
type Config struct {
    Enabled bool
    Limit   int
}

// 写入
p.Storage.Set(ctx, "config", Config{Enabled: true, Limit: 100})

// 读取
var cfg Config
if p.Storage.Get(ctx, "config", &cfg) {
    fmt.Println(cfg.Limit)
}
```

### ScanKeys

扫描匹配 pattern 的键（在插件命名空间内）。

```go
// 扫描所有 group: 前缀的键，最多返回 100 个
keys, err := p.Storage.ScanKeys(ctx, "group:*", 100)
```

::: warning
`ScanKeys` 返回的 key 是插件命名空间内的相对路径，可直接传给 `GetString` 等方法使用。
:::

### Del / Clear

```go
// 删除单个键
p.Storage.Del(ctx, "key")

// 清空插件所有数据
p.Storage.Clear(ctx)
```

### 列表操作

Redis List 语义，适合消息队列、历史记录等场景。

```go
// 写入
p.Storage.LPush(ctx, "history", "msg1", "msg2")
p.Storage.RPush(ctx, "queue", "task1")

// 弹出
val, ok := p.Storage.LPop(ctx, "queue")

// 范围读取（0 到 -1 表示全部）
items, ok := p.Storage.LRange(ctx, "history", 0, -1)

// 长度
n := p.Storage.LLen(ctx, "history")

// 修剪，只保留最近 100 条
p.Storage.LTrim(ctx, "history", -100, -1)
```

| 方法 | 说明 |
|------|------|
| `LPush` | 从左侧插入 |
| `RPush` | 从右侧插入 |
| `LPop` / `RPop` | 从左/右弹出 |
| `LRange` | 获取范围内元素 |
| `LLen` | 获取列表长度 |
| `LRem` | 移除指定值 |
| `LSet` | 设置指定索引的值 |
| `LIndex` | 获取指定索引的值 |
| `LTrim` | 修剪列表，保留指定范围 |

### Clone

创建带前缀的子存储空间，用于分类管理数据。

```go
// 为不同群创建独立的存储空间
groupStore := p.Storage.Clone(fmt.Sprintf("group:%d", msg.GroupId))
groupStore.SetString(ctx, "status", "active")
// 实际存储 key: <plugin_prefix>:group:123456:status
```

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

## 存储引擎对比

| 特性 | Redis 引擎 | 内存引擎 |
|------|-----------|---------|
| 配置方式 | `config.yaml` 配置 `bot.store.redis`（默认） | `WithStorage` 手动传入 |
| 数据持久化 | ✅ 重启不丢失 | ❌ 重启清空 |
| TTL 支持 | ✅ | ✅ |
| 分布式 | ✅ | ❌ |
| 适用场景 | 生产环境 | 开发/测试 |

## 命名空间隔离

每个插件的 `Name` 字段经 base64 编码后作为存储 key 的前缀，不同插件数据完全隔离，无需担心 key 冲突。

::: danger 重要
修改插件 `Name` 字段会导致存储前缀变化，原有数据将无法访问。生产环境中请谨慎修改插件名称。
:::
