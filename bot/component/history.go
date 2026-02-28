package component

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jeanhua/AniaBot/common/storage"
)

type Message struct {
	Role    string    `json:"role"`
	Content string    `json:"content"`
	Time    time.Time `json:"time"`
}

type MessageStore interface {
	SaveMessage(ctx context.Context, conversationId string, msg Message) error
	GetMessages(ctx context.Context, conversationId string, limit int) ([]Message, error)
	ClearHistory(ctx context.Context, conversationId string) error
}

type InMemoryMessageStore struct {
	messages map[string][]Message
}

func NewInMemoryMessageStore() *InMemoryMessageStore {
	return &InMemoryMessageStore{
		messages: make(map[string][]Message),
	}
}

func (s *InMemoryMessageStore) SaveMessage(ctx context.Context, conversationId string, msg Message) error {
	s.messages[conversationId] = append(s.messages[conversationId], msg)
	return nil
}

func (s *InMemoryMessageStore) GetMessages(ctx context.Context, conversationId string, limit int) ([]Message, error) {
	msgs := s.messages[conversationId]
	if limit > 0 && len(msgs) > limit {
		return msgs[len(msgs)-limit:], nil
	}
	return msgs, nil
}

func (s *InMemoryMessageStore) ClearHistory(ctx context.Context, conversationId string) error {
	s.messages[conversationId] = nil
	return nil
}

type RedisMessageStore struct {
	storage storage.Storage
	ttl     time.Duration
}

func NewRedisMessageStore(storage storage.Storage, ttl time.Duration) *RedisMessageStore {
	return &RedisMessageStore{
		storage: storage,
		ttl:     ttl,
	}
}

func (s *RedisMessageStore) SaveMessage(ctx context.Context, conversationId string, msg Message) error {
	msgs, _ := s.GetMessages(ctx, conversationId, 0)
	msgs = append(msgs, msg)
	data, err := json.Marshal(msgs)
	if err != nil {
		return err
	}
	s.storage.SetString(ctx, conversationId, string(data), storage.WithTTL(s.ttl))
	return nil
}

func (s *RedisMessageStore) GetMessages(ctx context.Context, conversationId string, limit int) ([]Message, error) {
	data, ok := s.storage.GetString(ctx, conversationId)
	if !ok {
		return nil, nil
	}
	var msgs []Message
	if err := json.Unmarshal([]byte(data), &msgs); err != nil {
		return nil, err
	}
	if limit > 0 && len(msgs) > limit {
		return msgs[len(msgs)-limit:], nil
	}
	return msgs, nil
}

func (s *RedisMessageStore) ClearHistory(ctx context.Context, conversationId string) error {
	s.storage.Del(ctx, conversationId)
	return nil
}
