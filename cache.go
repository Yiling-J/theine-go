package theine

import (
	"errors"
	"time"

	"github.com/Yiling-J/theine-go/internal"
)

const (
	ZERO_TTL = 0 * time.Second
)

type Cache struct {
	store *internal.Store
}

func New(size uint) (*Cache, error) {
	if size == 0 {
		return nil, errors.New("size must be positive")
	}

	return &Cache{
		store: internal.NewStore(size),
	}, nil
}

func (c *Cache) Get(key string) (interface{}, bool) {
	return c.store.Get(key)
}

func (c *Cache) SetWithTTL(key string, value interface{}, ttl time.Duration) {
	c.store.Set(key, value, ttl)
}

func (c *Cache) Set(key string, value interface{}) {
	c.SetWithTTL(key, value, ZERO_TTL)
}

func (c *Cache) Delete(key string) {
	c.store.Delete(key)
}

func (c *Cache) Len() int {
	return c.store.Len()
}

func (c *Cache) Close() {
	c.store.Close()
}
