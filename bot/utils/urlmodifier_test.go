package utils

import (
	"strings"
	"testing"
)

func TestURLModifier_SetDelQuery(t *testing.T) {
	m, err := NewURLModifier("https://example.com/path?foo=1")
	if err != nil {
		t.Fatalf("NewURLModifier error: %v", err)
	}

	s := m.String()
	if !strings.Contains(s, "foo=1") {
		t.Fatalf("expected foo=1 in %s", s)
	}

	m.SetQuery("bar", "2")
	s = m.String()
	if !strings.Contains(s, "bar=2") || !strings.Contains(s, "foo=1") {
		t.Fatalf("expected both foo and bar in %s", s)
	}

	m.DelQuery("foo")
	s = m.String()
	if strings.Contains(s, "foo=1") {
		t.Fatalf("expected foo removed, got %s", s)
	}
}
