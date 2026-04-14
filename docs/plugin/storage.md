# 数据存储

AniaBot 提供统一的存储抽象，支持 Redis 和内存两种引擎，插件数据自动按插件名隔离。

## 获取存储实例

插件通过 `plugin.Meta` 内嵌的 `Storage` 字段访问存储：

```go
func (p *MyPlugin) OnGroupMsg(ctx context.Context, b bot.Bot, cmd command.Command, msg message.Message) (bool, error) {
    // 直接使用 p.Storage
    p.Storage.SetString(ctx, "key", "value")
    val, ok := p.Storage.GetString(ctx, "key")
    return true, nil
}
```

## 存储接口

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

## Option 配置

```go
// 设置过期时间
storage.SetString(ctx, "session", "data", storage.WithTTL(24*time.Hour))

// 仅在键不存在时写入
storage.SetString(ctx, "init_flag", "1", storage.WithCheckExist())
```

| Option | 说明 |
|--------|------|
| `WithTTL(d time.Duration)` | 设置键的过期时间 |
| `WithCheckExist()` | 键不存在时才写入（类似 SETNX） |

## 存储任意结构体

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

## 子存储空间

使用 `Clone` 创建带前缀的子空间，方便分类管理：

```go
groupStorage := p.Storage.Clone("group")
groupStorage.SetString(ctx, "123456", "active")  // 实际 key: <plugin_prefix>:group:123456

userStorage := p.Storage.Clone("user")
userStorage.SetString(ctx, "789", "data")
```

## 扫描键

```go
// 扫描所有以 "group:" 开头的键（在插件命名空间内）
keys, err := p.Storage.ScanKeys(ctx, "group:*", 100)
for _, key := range keys {
    val, _ := p.Storage.GetString(ctx, key)
    fmt.Println(key, val)
}
```

## 存储引擎

框架**默认使用 Redis**，启动时读取配置并 Ping 连接，连接失败直接 panic。

```yaml
# config.yaml
bot:
  store:
    redis:
      address: "localhost:6379"
      password: ""
      db: 0
```

如需使用**内存引擎**（开发/测试场景），在创建 Bot 时通过 `WithStorage` 手动传入：

```go
import "github.com/jeanhua/AniaBot/bot/core"

memStorage := core.NewAniaMemoryStorage(logger)
bot := core.NewAniaBot(adapter, core.WithStorage(memStorage))
```

| 引擎 | 配置方式 | 特点 |
|------|---------|------|
| Redis（默认） | `config.yaml` 配置 `bot.store.redis` | 持久化，重启不丢失 |
| 内存 | `WithStorage` 手动传入 | 轻量，重启清空，适合开发测试 |

::: warning 命名空间隔离
框架以插件 `Name` 字段的 base64 编码作为存储前缀，不同插件的数据完全隔离。修改插件 `Name` 后，原有数据将无法访问，请在修改前迁移数据。
:::
