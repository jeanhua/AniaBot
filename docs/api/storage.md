# 存储接口参考

两层存储均通过插件 `Meta` 注入，按插件名自动命名空间隔离。

```go
p.Storage            // storage.Storage —— 缓存层
p.PersistentStorage  // storage.PersistentStorage —— 持久层
```

::: info 通用约定
- 所有方法返回 `(value, bool)` 或 `bool`，内部错误记录日志后以 `false` 返回
- `Get(ctx, key, out)` 的 `out` 必须是**指针**，内部 JSON 反序列化
- `Clone(prefix)` 创建子命名空间，可多级嵌套
:::

## Storage —— 缓存层

后端：进程内内存（默认）/ Redis。支持 TTL 与列表语义。

### KV

```go
GetString(ctx context.Context, key string) (string, bool)
SetString(ctx context.Context, key, val string, option ...Option) bool

Get(ctx context.Context, key string, out any) bool
Set(ctx context.Context, key string, val any, option ...Option) bool

Del(ctx context.Context, key string) bool
Clear(ctx context.Context) bool                       // 清空本插件命名空间

ScanKeys(ctx context.Context, pattern string, count int64) ([]string, error)

Clone(prefix string) Storage

Expire(ctx context.Context, key string, ttl time.Duration) bool
```

### 选项（Option）

```go
storage.WithTTL(ttl)        // 写入时设置过期时间
storage.WithCheckExist()    // 写入时检查键是否已存在
```

### 列表（Redis List 语义）

```go
LPush(ctx context.Context, key string, values ...any) int64      // 左侧插入，返回长度
RPush(ctx context.Context, key string, values ...any) int64      // 右侧插入，返回长度
LPop(ctx context.Context, key string) (any, bool)                // 左侧弹出
RPop(ctx context.Context, key string) (any, bool)                // 右侧弹出
LRange(ctx context.Context, key string, start, stop int64) ([]any, bool)
LLen(ctx context.Context, key string) int64
LRem(ctx context.Context, key string, count int64, value any) int64  // 移除指定值
LSet(ctx context.Context, key string, index int64, value any) bool   // 按索引设置
LIndex(ctx context.Context, key string, index int64) (any, bool)
LTrim(ctx context.Context, key string, start, stop int64) bool       // 保留范围内元素
```

## PersistentStorage —— 持久层

后端：SQLite（默认，纯 Go）/ MySQL。无 TTL、无列表语义。

```go
GetString(ctx context.Context, key string) (string, bool)
SetString(ctx context.Context, key, val string) bool      // 覆盖写

Get(ctx context.Context, key string, out any) bool        // JSON 反序列化到 out（指针）
Set(ctx context.Context, key string, val any) bool        // JSON 序列化

Has(ctx context.Context, key string) bool
Del(ctx context.Context, key string) bool                 // 键不存在也返回 true
Keys(ctx context.Context, prefix string) ([]string, error) // 按前缀列出相对键
Clear(ctx context.Context) bool                           // 清空本插件全部数据，谨慎！

Clone(prefix string) PersistentStorage
```

::: tip 有序数据怎么存？
持久层没有列表语义。需要保存数组时，把切片作为 JSON 值整体读写：

```go
var list []string
p.PersistentStorage.Get(ctx, "list", &list)
list = append(list, "new item")
p.PersistentStorage.Set(ctx, "list", list)
```
:::

## 后端配置

```yaml
bot:
  store:
    cache:
      driver: redis   # redis | memory
      redis:
        address: "localhost:6379"
        password: ""
        db: 0
    persistent:
      driver: sqlite  # sqlite | mysql
      sqlite:
        path: "./data/aniabot.db"
      mysql:
        dsn: "root:password@tcp(localhost:3306)/aniabot?charset=utf8mb4&parseTime=true&loc=Local"
        max_open_conns: 20
        max_idle_conns: 5
        conn_max_lifetime_sec: 300
```

使用示例与实战模式见 [插件开发 · 数据存储](/plugin/storage)。
