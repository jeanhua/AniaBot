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

    Del(ctx context.Context, key string) bool
    Clear(ctx context.Context) bool

    Clone(prefix string) Storage
}
```

## 方法说明

- `GetString(ctx, key) (string, bool)`
  - 返回键对应的字符串值与是否存在的布尔标识。

- `SetString(ctx, key, val, option ...Option) bool`
  - 写入字符串值，接受可选的 `Option`（例如 `WithTTL` 指定过期时间）。返回操作成功与否。

- `Get(ctx, key, out any) bool`
  - 将存储的值反序列化到 `out`（实现侧可用 JSON 或其他序列化），返回是否存在/成功。

- `Set(ctx, key, val any, option ...Option) bool`
  - 保存任意值（实现侧会序列化），可传可选 `Option` 控制行为（如 TTL）。返回是否成功。

- `Del(ctx, key) bool`
  - 删除键，返回是否成功。

- `Clear(ctx) bool`
  - 清空当前存储命名空间/前缀下的数据，返回是否成功。

- `Clone(prefix string) Storage`
  - 克隆一个带有新的键前缀的 `Storage` 实例。插件框架会为每个插件分配独立前缀以避免键冲突。

## Option 与 TTL

- `StorageConfig` 包含 `TTL time.Duration`，可通过 `WithTTL(d)` 构造 `Option` 并传入 `Set`/`SetString` 来设置键过期时间。

## 使用要点

- 框架通常会在插件初始化阶段将具体的 `Storage` 实例注入到插件的 `Meta.Storage` 字段，插件直接使用该接口进行读写。
- `Clone` 可用于在同一实现下为子模块创建带前缀的存储实例。

## 示例

```go
func (p *LogPlugin) Start(cfg *viper.Viper) {
	lastStartTime, ok := p.Storage.GetString(context.Background(), "last_start_time")
	if !ok {
		lastStartTime = "未保存"
	}
	p.Storage.Set(context.Background(), "last_start_time", utils.GetFormattedTime())
	log.Println("日志打印插件初始化完成, 上次重启时间: ", lastStartTime)
}
```

