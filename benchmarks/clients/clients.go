package clients

import (
	"github.com/Yiling-J/theine-go"
	"github.com/dgraph-io/ristretto"
	lru "github.com/hashicorp/golang-lru/v2"
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
	client, err := theine.New[K, V](int64(cap))
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

type Ristretto[K comparable, V any] struct {
	client *ristretto.Cache
}

func (c *Ristretto[K, V]) Init(cap int) {
	client, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: int64(cap * 10),
		MaxCost:     int64(cap),
		BufferItems: 64,
	})
	if err != nil {
		panic(err)
	}
	c.client = client
}

func (c *Ristretto[K, V]) GetSet(key K, value V) (V, bool) {
	v, ok := c.client.Get(key)
	if ok {
		return v.(V), true
	}
	c.client.Set(key, value, 1)
	return value, false
}

func (c *Ristretto[K, V]) Set(key K, value V) {
	c.client.Set(key, value, 1)
}
func (c *Ristretto[K, V]) Name() string {
	return "ristretto"
}

func (c *Ristretto[K, V]) Close() {
	c.client.Close()
}

type LRU[K comparable, V any] struct {
	client *lru.Cache[K, V]
}

func (c *LRU[K, V]) Init(cap int) {
	client, err := lru.New[K, V](cap)
	if err != nil {
		panic(err)
	}
	c.client = client

}

func (c *LRU[K, V]) GetSet(key K, value V) (V, bool) {
	v, ok := c.client.Get(key)
	if ok {
		return v, true
	}
	c.client.Add(key, value)
	return value, false
}

func (c *LRU[K, V]) Set(key K, value V) {
	c.client.Add(key, value)
}
func (c *LRU[K, V]) Name() string {
	return "lru"
}

func (c *LRU[K, V]) Close() {
}
