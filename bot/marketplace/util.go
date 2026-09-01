package marketplace

import "encoding/json"

// jsonUnmarshal 便捷解析（loadIndex 缓存读取用）。
func jsonUnmarshal(data []byte, v any) error { return json.Unmarshal(data, v) }

// mustJSON 编码为 JSON（loadIndex 缓存写入用，失败返回 nil 由调用方忽略）。
func mustJSON(v any) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return data
}
