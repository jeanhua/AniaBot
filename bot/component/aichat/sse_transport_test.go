package aichat

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestSSECommentFilterReader(t *testing.T) {
	input := ": OPENROUTER PROCESSING\n\n" +
		": keep-alive\n\n" +
		"\n\n" +
		"data: {\"id\":\"1\",\"choices\":[{\"delta\":{\"content\":\"hello\"}}]}\n\n" +
		": middle comment\n\n" +
		"data: {\"id\":\"2\",\"choices\":[{\"delta\":{\"content\":\" world\"}}]}\n\n" +
		"data: [DONE]\n\n"

	reader := newSSECommentFilterReader(strings.NewReader(input))
	gotBytes, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll error: %v", err)
	}
	got := string(gotBytes)

	want := "data: {\"id\":\"1\",\"choices\":[{\"delta\":{\"content\":\"hello\"}}]}\n\n" +
		"data: {\"id\":\"2\",\"choices\":[{\"delta\":{\"content\":\" world\"}}]}\n\n" +
		"data: [DONE]\n\n"

	if got != want {
		t.Fatalf("output mismatch:\ngot:\n%q\nwant:\n%q", got, want)
	}
}

func TestSSECommentFilterReaderMultiLineData(t *testing.T) {
	input := ": comment\n\n" +
		"data: line1\n" +
		"data: line2\n\n"

	reader := newSSECommentFilterReader(strings.NewReader(input))
	gotBytes, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll error: %v", err)
	}
	got := string(gotBytes)

	want := "data: line1\n" +
		"data: line2\n\n"

	if got != want {
		t.Fatalf("output mismatch:\ngot:\n%q\nwant:\n%q", got, want)
	}
}

type dummyCloser struct {
	closed bool
}

func (d *dummyCloser) Close() error {
	d.closed = true
	return nil
}

func TestSSEFilterBodyClose(t *testing.T) {
	d := &dummyCloser{}
	body := &sseFilterBody{
		Reader: bytes.NewReader([]byte("test")),
		closer: d,
	}
	if err := body.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
	if !d.closed {
		t.Fatal("expected closer to be closed")
	}
}

