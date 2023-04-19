package clients

import (
	"github.com/Yiling-J/theine-go"
)

type Client[K comparable, V any] interface {
	Init(cap int)
	// Get key, return value and true if exist
	// Set key if not exist and return false
	GetSet(key K, value V) (V, bool)
	Set(key K, value V)
	Name() string
	Close()
}

type Theine[K comparable, V any] struct {
	client *theine.Cache[K, V]
}

func (c *Theine[K, V]) Init(cap int) {
	client, err := theine.NewBuilder[K, V](int64(cap)).Build()
	if err != nil {
		panic(err)
	}
	c.client = client
}

func (c *Theine[K, V]) GetSet(key K, value V) (V, bool) {
	v, ok := c.client.Get(key)
	if ok {
		return v, true
	}
	c.client.Set(key, value, 1)
	return value, false
}

func (c *Theine[K, V]) Set(key K, value V) {
	c.client.Set(key, value, 1)
}
func (c *Theine[K, V]) Name() string {
	return "theine"
}

func (c *Theine[K, V]) Close() {
	c.client.Close()
}
