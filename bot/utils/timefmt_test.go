package utils

import (
	"testing"
	"time"
)

func TestGetFormattedTime_Parseable(t *testing.T) {
	s := GetFormattedTime()
	if _, err := time.Parse(time.RFC3339, s); err != nil {
		t.Fatalf("GetFormattedTime returned non-RFC3339 string: %v (value: %s)", err, s)
	}
}
