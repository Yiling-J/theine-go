package clients

import (
	"github.com/Yiling-J/theine-go"
	"github.com/dgraph-io/ristretto"
)

type Client interface {
	Init(cap int)
	// Get key, return value and true if exist
	// Set key if not exist and return false
	GetSet(key string) (string, bool)
	Set(key string)
	Name() string
	Close()
}

type Theine struct {
	client *theine.Cache
}

func (c *Theine) Init(cap int) {
	client, err := theine.New(uint(cap))
	if err != nil {
		panic(err)
	}
	c.client = client
}

func (c *Theine) GetSet(key string) (string, bool) {
	v, ok := c.client.Get(key)
	if ok {
		return v.(string), true
	}
	c.client.Set(key, key)
	return key, false
}

func (c *Theine) Set(key string) {
	c.client.Set(key, key)
}
func (c *Theine) Name() string {
	return "theine"
}

func (c *Theine) Close() {
	c.client.Close()
}

type Ristretto struct {
	client *ristretto.Cache
}

func (c *Ristretto) Init(cap int) {
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

func (c *Ristretto) GetSet(key string) (string, bool) {
	v, ok := c.client.Get(key)
	if ok {
		return v.(string), true
	}
	c.client.Set(key, key, 1)
	return key, false
}

func (c *Ristretto) Set(key string) {
	c.client.Set(key, key, 1)
}
func (c *Ristretto) Name() string {
	return "ristretto"
}

func (c *Ristretto) Close() {
	c.client.Close()
}
