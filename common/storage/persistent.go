package storage

import "context"

// PersistentStorage 持久化存储接口。
//
// 与易失的缓存 [Storage] 不同，持久化存储的数据在进程重启后不丢失，
// 适合保存需要长期留存的状态：插件配置、用户数据、历史记录、积分等。
//
// 语义约定：
//   - 不支持 TTL 与列表语义——这些属于缓存范畴。如需保存有序数据，
//     可将 JSON 数组作为值整体读写（[Set]/[Get]）；数据量大时建议
//     用可排序的键（如 e:<序号>）逐条存储，避免单条记录体积失控。
//   - 命名空间隔离与缓存 [Storage] 一致：[Clone] 创建带前缀的子空间，
//     框架在注入时已用插件名做好基础隔离，插件内部再按需 Clone。
//   - 所有方法返回 (value, bool) 或 bool，错误内部记录日志后以 false 返回，
//     与 [Storage] 的错误处理风格保持一致。
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
