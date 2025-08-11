package storage

import (
	"sync"
)

var (
	instance *InMemoryStorage
	once     sync.Once
)

type InMemoryStorage struct {
	mu   sync.RWMutex
	data map[string][]any
}

func NewInMemoryStorage() *InMemoryStorage {
	once.Do(func() {
		instance = &InMemoryStorage{
			data: make(map[string][]any),
		}
	})
	return instance
}

func (s *InMemoryStorage) InsertOne(key string, value any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = append(s.data[key], value)
}

func (s *InMemoryStorage) Get(key string) []any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]any(nil), s.data[key]...)
}
