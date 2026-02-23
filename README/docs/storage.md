# 存储（Storage）接口参考

本页概述 AniaBot 中插件可使用的存储接口与常见用法。框架使用 `common/storage` 提供统一的 Storage 抽象（具体实现常见为 Redis），实际方法和签名以 `common/storage/storage.go` 为准。

主要目的：为插件提供持久化读写能力，并对各插件做键前缀隔离。

## 接口签名

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

## Option 配置系统

```go
type Option func(*StorageConfig)

type StorageConfig struct {
    TTL        time.Duration // 键的过期时间
    CheckExist bool          // 写入前检查键是否存在
}

// WithTTL 设置键的过期时间
// 示例: WithTTL(24 * time.Hour) 设置过期时间为24小时
func WithTTL(ttl time.Duration) Option {
    return func(sc *StorageConfig) {
        sc.TTL = ttl
    }
}

// WithCheckExist 检查键是否存在（用于 Set 操作）
// 如果配合 Set 使用，会在写入前检查键是否已存在，不存在则写入
func WithCheckExist() Option {
    return func(sc *StorageConfig) {
        sc.CheckExist = true
    }
}
```

## 方法详细说明

### GetString
```go
GetString(ctx context.Context, key string) (string, bool)
```
- **功能**：获取指定键的字符串值
- **参数**：
  - `ctx`: 上下文，用于超时控制
  - `key`: 键名（自动添加前缀）
- **返回值**：
  - `string`: 键对应的值
  - `bool`: 键是否存在
- **注意**：返回的 bool 值为 false 时，字符串值为零值

### SetString
```go
SetString(ctx context.Context, key, val string, option ...Option) bool
```
- **功能**：设置字符串值
- **参数**：
  - `ctx`: 上下文
  - `key`: 键名
  - `val`: 要存储的字符串值
  - `option`: 可选配置（如 TTL、检查存在等）
- **返回值**：`bool` 操作是否成功
- **错误处理**：操作失败（如连接错误）返回 false

### Get
```go
Get(ctx context.Context, key string, out any) bool
```
- **功能**：获取值并反序列化到 out
- **参数**：
  - `ctx`: 上下文
  - `key`: 键名
  - `out`: 输出参数，必须是**指针类型**
- **返回值**：`bool` 是否存在/成功
- **序列化**：默认使用 JSON 反序列化
- **错误处理**：
  - 键不存在：返回 false
  - 反序列化失败：返回 false（记录错误日志）

### Set
```go
Set(ctx context.Context, key string, val any, option ...Option) bool
```
- **功能**：序列化并存储任意值
- **参数**：
  - `ctx`: 上下文
  - `key`: 键名
  - `val`: 任意类型的值
  - `option`: 可选配置
- **返回值**：`bool` 操作是否成功
- **序列化**：默认使用 JSON 序列化
- **错误处理**：序列化失败或存储失败时返回 false

### ScanKeys
```go
ScanKeys(ctx context.Context, pattern string, count int64) ([]string, error)
```
- **功能**：扫描匹配模式的键
- **参数**：
  - `ctx`: 上下文
  - `pattern`: 匹配模式，支持：
    - `*`: 匹配任意字符（如 "user:*"）
    - `?`: 匹配单个字符（如 "user:?"）
    - `[]`: 匹配范围（如 "user:[1-5]"）
  - `count`: 每次扫描的批次大小，建议 10-100
- **返回值**：
  - `[]string`: 匹配的键列表（**不包含前缀**）
  - `error`: 扫描过程中的错误
- **注意**：返回的键名已去除存储前缀，可直接用于其他操作

### Del
```go
Del(ctx context.Context, key string) bool
```
- **功能**：删除指定键
- **参数**：
  - `ctx`: 上下文
  - `key`: 要删除的键名
- **返回值**：`bool` 删除是否成功（键不存在也返回 true）

### Clear
```go
Clear(ctx context.Context) bool
```
- **功能**：清空当前存储命名空间/前缀下的所有数据
- **参数**：`ctx`: 上下文
- **返回值**：`bool` 清空操作是否成功
- **注意**：
  - 只清除当前前缀下的数据
  - 大量数据时可能耗时较长
  - 操作具有原子性

### Clone
```go
Clone(prefix string) Storage
```
- **功能**：克隆一个带有新的键前缀的 Storage 实例
- **参数**：`prefix`: 新的前缀
- **返回值**：新的 Storage 实例
- **用途**：为子模块创建隔离的存储空间
