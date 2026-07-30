package message

import (
	"crypto/sha256"
	"encoding/hex"
)

// imageHashLen 图片短哈希长度（hex 字符数）
const imageHashLen = 8

// ImageHash 根据图片引用（URL 或 data URI）计算短哈希，作为图片在 AI 上下文中的
// 稳定标识（如 [图片 a1b2c3d4]），便于 AI 区分和引用同一会话中的多张图片。
// 同一引用必得同一哈希；QQ 图片 URL 含唯一文件标识，不同图片哈希不同。
func ImageHash(ref string) string {
	sum := sha256.Sum256([]byte(ref))
	return hex.EncodeToString(sum[:])[:imageHashLen]
}

// Hash 返回该图片消息的短哈希（优先 URL，为空时退化为文件名）。
func (msg ImageMessage) Hash() string {
	ref := msg.Url
	if ref == "" {
		ref = msg.File
	}
	return ImageHash(ref)
}
