package message

import (
	"fmt"
	"strconv"
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

	val, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid QID: %w", err)
	}
	*q = QID(strconv.FormatUint(val, 10))
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
