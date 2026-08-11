package message

import (
	"strconv"
	"strings"
)

// QQIDPrefix QQ（NapCat）平台的框架统一 ID 前缀。
// 旧版本以裸数字表示 QQ ID，升级后会自动迁移为 qq:<数字>。
const QQIDPrefix = "qq:"

type QID string

func (q QID) MarshalJSON() ([]byte, error) {
	return []byte(`"` + string(q) + `"`), nil
}

func (q *QID) UnmarshalJSON(data []byte) error {
	s := string(data)

	if s == "null" {
		*q = ""
		return nil
	}

	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		unquoted, err := strconv.Unquote(s)
		if err != nil {
			return err
		}
		s = unquoted
	}

	// 空串为零值（如定时任务创建者未设置时落盘的 ""）
	if s == "" {
		*q = ""
		return nil
	}

	// 数字 ID（QQ）统一为带 qq: 前缀的无前导零十进制串；
	// 非数字 ID（其他平台，如飞书 fs:ou_xxx）原样保留。
	if val, err := strconv.ParseUint(s, 10, 64); err == nil {
		*q = QID(QQIDPrefix + strconv.FormatUint(val, 10))
	} else {
		*q = FromString(s)
	}
	return nil
}

func (q QID) String() string {
	return string(q)
}

func (q QID) Uint64() uint64 {
	val, _ := strconv.ParseUint(q.TrimQQPrefix(), 10, 64)
	return val
}

// NormalizeQQID 把纯数字 QQ ID 规范化为 qq: 前缀，其余 ID 原样保留。
// 已带 qq: 前缀或非数字 ID（fs:、tg:、dc: 等）不会被重复处理。
func NormalizeQQID(s string) string {
	if s == "" || strings.HasPrefix(s, QQIDPrefix) {
		return s
	}
	raw := strings.TrimSpace(s)
	if val, err := strconv.ParseUint(raw, 10, 64); err == nil {
		return QQIDPrefix + strconv.FormatUint(val, 10)
	}
	return s
}

// WithQQPrefix 为 QQ 原始数字 ID 添加 qq: 前缀；已带前缀或非数字 ID 原样返回。
func (q QID) WithQQPrefix() QID {
	return FromString(string(q))
}

// TrimQQPrefix 去掉 qq: 前缀，还原 QQ 平台原始 ID。
func (q QID) TrimQQPrefix() string {
	return strings.TrimPrefix(string(q), QQIDPrefix)
}

// FromString 从字符串构造 QID。纯数字会规范化为 QQ 的 qq: 前缀格式。
func FromString(s string) QID {
	return QID(NormalizeQQID(s))
}

// FromUint64 由数值构造 QQ 的 QID（统一带 qq: 前缀）。
func FromUint64(v uint64) QID {
	return QID(QQIDPrefix + strconv.FormatUint(v, 10))
}

// AddPrefix 为平台原始 ID 添加框架统一前缀（如飞书 "fs:"），
// 已带该前缀时原样返回。多平台共存时用于在适配器边界标记 ID 来源。
func (q QID) AddPrefix(prefix string) QID {
	if prefix == "" || strings.HasPrefix(string(q), prefix) {
		return q
	}
	return QID(prefix + string(q))
}

// TrimPrefix 去掉框架统一前缀，还原平台原始 ID（适配器调用平台 API 前使用）。
func (q QID) TrimPrefix(prefix string) string {
	if prefix == "" {
		return string(q)
	}
	return strings.TrimPrefix(string(q), prefix)
}
