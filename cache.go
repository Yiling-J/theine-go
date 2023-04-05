package theine

import (
	"errors"
	"time"

	"github.com/Yiling-J/theine-go/internal"
)

const (
	ZERO_TTL = 0 * time.Second
)

type Config struct {
	MaximumSize int64
}

type Cache[K comparable, V any] struct {
	store *internal.Store[K, V]
}

func New[K comparable, V any](config *Config) (*Cache[K, V], error) {
	if config.MaximumSize <= 0 {
		return nil, errors.New("size must be positive")
	}

	return &Cache[K, V]{
		store: internal.NewStore[K, V](uint(config.MaximumSize)),
	}, nil
}

func (c *Cache[K, V]) Get(key K) (V, bool) {
	return c.store.Get(key)
}

func (c *Cache[K, V]) SetWithTTL(key K, value V, ttl time.Duration) {
	c.store.Set(key, value, ttl)
}

func (c *Cache[K, V]) Set(key K, value V) {
	c.SetWithTTL(key, value, ZERO_TTL)
}

func (c *Cache[K, V]) Delete(key K) {
	c.store.Delete(key)
}

func (c *Cache[K, V]) Len() int {
	return c.store.Len()
}

func (c *Cache[K, V]) Close() {
	c.store.Close()
}
