package message

import (
	"fmt"
	"strconv"
)

type QID uint64

func (q QID) MarshalJSON() ([]byte, error) {
	return []byte(`"` + strconv.FormatUint(uint64(q), 10) + `"`), nil
}

func (q *QID) UnmarshalJSON(data []byte) error {
	s := string(data)

	if s == "null" {
		*q = 0
		return nil
	}

	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		unquoted, err := strconv.Unquote(s)
		if err != nil {
			return err
		}
		s = unquoted
	}

	val, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid QID: %w", err)
	}
	*q = QID(val)
	return nil
}

func (q QID) String() string {
	return strconv.FormatUint(uint64(q), 10)
}

func (q QID) Uint64() uint64 {
	return uint64(q)
}

func FromString(s string) (QID, error) {
	val, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid QID: %w", err)
	}
	return QID(val), nil
}
