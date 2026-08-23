package pluginantiwithdrawal

import (
	"sync"
)

type MessageQueue[T any] struct {
	messages []T
	size     int
	head     int
	tail     int
	full     bool
	mu       sync.Mutex
}

func NewMessageQueue[T any](capacity int) *MessageQueue[T] {
	return &MessageQueue[T]{
		messages: make([]T, capacity),
		size:     capacity,
	}
}

func (q *MessageQueue[T]) Add(msg T) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.messages[q.tail] = msg
	if q.full {
		q.head = (q.head + 1) % q.size
	}
	q.tail = (q.tail + 1) % q.size
	if q.tail == q.head {
		q.full = true
	}
}

func (q *MessageQueue[T]) GetCount() int {
	if q.full {
		return q.size
	}
	return (q.tail - q.head + q.size) % q.size
}

func (q *MessageQueue[T]) GetAll() []T {
	q.mu.Lock()
	defer q.mu.Unlock()

	count := q.size
	if !q.full {
		count = (q.tail - q.head + q.size) % q.size
	}

	result := make([]T, count)
	for i := 0; i < count; i++ {
		idx := (q.head + i) % q.size
		result[i] = q.messages[idx]
	}
	return result
}

func (q *MessageQueue[T]) Get(limit int) []T {
	q.mu.Lock()
	defer q.mu.Unlock()

	currentCount := q.size
	if !q.full {
		currentCount = (q.tail - q.head + q.size) % q.size
	}

	count := min(limit, currentCount)

	if count <= 0 {
		return make([]T, 0)
	}

	result := make([]T, count)
	startIdx := (q.tail - count + q.size) % q.size

	for i := range count {
		idx := (startIdx + i) % q.size
		result[i] = q.messages[idx]
	}

	return result
}
