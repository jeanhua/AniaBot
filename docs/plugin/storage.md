# 数据存储

AniaBot 提供统一的存储抽象，支持 Redis 和内存两种引擎，插件数据自动按插件名隔离。你不需要关心底层用的是什么数据库，只需要调用 `p.Storage` 的方法即可。

## 获取存储实例

插件通过 `plugin.Meta` 内嵌的 `Storage` 字段访问存储，无需额外初始化：

```go
func (p *MyPlugin) OnGroupMsg(ctx context.Context, b bot.Bot, cmd command.Command, msg message.Message) (bool, error) {
    // 直接使用 p.Storage
    p.Storage.SetString(ctx, "key", "value")
    val, ok := p.Storage.GetString(ctx, "key")
    return true, nil
}
```

::: tip 什么时候用 ctx？
存储操作的 `ctx` 参数用于超时控制和取消。在消息处理方法中直接传入框架给的 `ctx` 即可。在后台 goroutine 中，可以用 `context.WithTimeout` 创建带超时的 context。
:::

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

## 字符串操作：GetString / SetString

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

## JSON 操作：Get / Set

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

## Option 配置

### WithTTL — 设置过期时间

```go
// 设置 24 小时过期
storage.SetString(ctx, "session", "data", storage.WithTTL(24*time.Hour))
```

**适用场景**：缓存、临时会话、每日签到标记等。

### WithCheckExist — 防止重复写入

```go
// 仅在键不存在时写入（类似 SETNX）
ok := storage.SetString(ctx, "init_flag", "1", storage.WithCheckExist())
if !ok {
    // 键已存在，写入失败
}
```

**适用场景**：每日签到、防止重复操作、分布式锁。

### 组合使用

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

## 子存储空间

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

## 扫描键

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

## 列表操作

Redis List 语义，适合消息队列、历史记录等场景。

### 基本操作

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

### 实战：固定长度的消息队列

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

### 列表操作速查

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

## 存储引擎

框架**默认使用 Redis**，启动时读取配置并 Ping 连接，连接失败直接 panic。

```yaml
# config.yaml
redis:
  addr: "localhost:6379"
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
| Redis（默认） | `config.yaml` 配置 `redis` | 持久化，重启不丢失，支持 TTL |
| 内存 | `WithStorage` 手动传入 | 轻量，重启清空，适合开发测试 |

::: warning 命名空间隔离
框架以插件 `Name` 字段的 base64 编码作为存储前缀，不同插件的数据完全隔离。修改插件 `Name` 后，原有数据将无法访问，请在修改前迁移数据。
:::

## 常见用法总结

| 场景 | 推荐方法 | 参考插件 |
|------|---------|---------|
| 存储用户积分/计数 | `SetString` / `GetString` | 积分系统 |
| 存储用户配置/状态 | `Set` / `Get` (JSON) | — |
| 每日签到防重复 | `SetString` + `WithTTL` + `WithCheckExist` | 积分系统 |
| 缓存 API 结果 | `SetString` + `WithTTL` | urlparser |
| 消息历史记录 | `RPush` + `LTrim` + `LRange` | groupnewsletter |
| Per-group 状态 | `Clone("group")` 或 `sync.Map` | gdmusicplugin |

## 下一步

- [存储接口参考](../api/storage) — 完整的 API 文档
- [定时任务](./cron) — 定时清理过期数据
- [常见模式](./patterns) — 更多存储相关的设计模式
