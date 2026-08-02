package message

import (
	"strconv"
	"strings"
)

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

	// 数字 ID（QQ）规范化为无前导零的十进制串；
	// 非数字 ID（其他平台，如飞书 fs:ou_xxx）原样保留。
	if val, err := strconv.ParseUint(s, 10, 64); err == nil {
		*q = QID(strconv.FormatUint(val, 10))
	} else {
		*q = QID(s)
	}
	return nil
}

func (q QID) String() string {
	return string(q)
}

func (q QID) Uint64() uint64 {
	val, _ := strconv.ParseUint(string(q), 10, 64)
	return val
}

func FromString(s string) QID {
	return QID(s)
}

// FromUint64 由数值构造 QID（QID 底层为十进制数字字符串）。
func FromUint64(v uint64) QID {
	return QID(strconv.FormatUint(v, 10))
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
