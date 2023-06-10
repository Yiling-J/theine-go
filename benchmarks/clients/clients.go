package clients

import (
	"encoding/gob"
	"io"
	"os"

	"github.com/Yiling-J/theine-go"
	"github.com/Yiling-J/theine-go/internal"
	"github.com/Yiling-J/theine-go/internal/clock"
	"github.com/cockroachdb/pebble"
)

type Client[K comparable, V any] interface {
	Init(cap int)
	Get(key K) (V, bool)
	Set(key K, value V)
	Name() string
	Close()
	Metrics() string
}

type TheineMem[K comparable, V any] struct {
	client *theine.Cache[K, V]
}

func (c *TheineMem[K, V]) Init(cap int) {
	client, err := theine.NewBuilder[K, V](int64(cap)).Build()
	if err != nil {
		panic(err)
	}
	c.client = client
}

func (c *TheineMem[K, V]) Get(key K) (V, bool) {
	return c.client.Get(key)
}

func (c *TheineMem[K, V]) Set(key K, value V) {
	c.client.Set(key, value, 1)
}

func (c *TheineMem[K, V]) Name() string {
	return "theine"
}

func (c *TheineMem[K, V]) Close() {
	c.client.Close()
}

type TheineNvm[K comparable, V any] struct {
	client          *theine.HybridCache[K, V]
	KeySerializer   theine.Serializer[K]
	ValueSerializer theine.Serializer[V]
}

func (c *TheineNvm[K, V]) Init(cap int) {
	nvm, err := theine.NewNvmBuilder[K, V]("foo", 4000<<20).BigHashPct(20).
		KeySerializer(c.KeySerializer).ValueSerializer(c.ValueSerializer).ErrorHandler(func(err error) {
		panic(err)
	}).Build()
	if err != nil {
		panic(err)
	}
	client, err := theine.NewBuilder[K, V](int64(cap)).Hybrid(nvm).Workers(8).Build()
	if err != nil {
		panic(err)
	}
	c.client = client
}

func (c *TheineNvm[K, V]) Get(key K) (V, bool) {
	v, ok, err := c.client.Get(key)
	if err != nil {
		panic(err)
	}
	return v, ok
}

func (c *TheineNvm[K, V]) Set(key K, value V) {
	_ = c.client.Set(key, value, 1)
}
func (c *TheineNvm[K, V]) Name() string {
	return "theine"
}

func (c *TheineNvm[K, V]) Close() {
	c.client.Close()
	_ = os.Remove("foobar")
}

func (c *TheineNvm[K, V]) Metrics() string {
	return ""
}

type PebbleCache[K comparable, V any] struct {
	db              *pebble.DB
	keySerializer   theine.Serializer[K]
	valueSerializer theine.Serializer[V]
}

func (n *PebbleCache[K, V]) Get(key K) (value V, cost int64, expire int64, ok bool, err error) {
	kb, err := n.keySerializer.Marshal(key)
	if err != nil {
		return value, cost, expire, false, err
	}
	vb, cost, expire, ok, closer, err := n.get(kb)
	defer func() {
		if closer != nil {
			_ = closer.Close()
		}
	}()
	if err != nil {
		return value, cost, expire, false, err
	}
	if !ok {
		return value, cost, expire, false, &internal.NotFound{}
	}
	err = n.valueSerializer.Unmarshal(vb, &value)
	if err != nil {
		return value, cost, expire, false, err
	}
	return value, cost, expire, true, nil
}

func (c *PebbleCache[K, V]) get(key []byte) (value []byte, cost int64, expire int64, ok bool, closer io.Closer, err error) {
	v, closer, err := c.db.Get(key)
	if err == pebble.ErrNotFound {
		return v, 1, 0, false, closer, nil
	}
	return v, 1, 0, true, closer, err
}

func (n *PebbleCache[K, V]) Set(key K, value V, cost int64, expire int64) error {
	kb, err := n.keySerializer.Marshal(key)
	if err != nil {
		return err
	}
	vb, err := n.valueSerializer.Marshal(value)
	if err != nil {
		return err
	}
	return n.set(kb, vb, cost, expire)
}

func (c *PebbleCache[K, V]) set(key []byte, value []byte, cost int64, expire int64) error {
	return c.db.Set(key, value, pebble.NoSync)
}

func (n *PebbleCache[K, V]) Delete(key K) error {
	kb, err := n.keySerializer.Marshal(key)
	if err != nil {
		return err
	}
	return n.delete(kb)
}

func (c *PebbleCache[K, V]) delete(key []byte) error {
	return c.db.Delete(key, nil)
}

func (c *PebbleCache[K, V]) Load(block *theine.DataBlock) error {
	return nil
}

func (c *PebbleCache[K, V]) SaveMeta(blockEncoder *gob.Encoder) error {
	return nil
}

func (c *PebbleCache[K, V]) SaveData(blockEncoder *gob.Encoder) error {
	return nil
}

func (c *PebbleCache[K, V]) HandleAsyncError(err error) {
	panic(err)
}

func (c *PebbleCache[K, V]) SetClock(clock *clock.Clock) {}

type TheinePebble[K comparable, V any] struct {
	client          *theine.HybridCache[K, V]
	KeySerializer   theine.Serializer[K]
	ValueSerializer theine.Serializer[V]
	db              *pebble.DB
}

func (c *TheinePebble[K, V]) Init(cap int) {
	db, err := pebble.Open("demo", &pebble.Options{
		DisableWAL:               true,
		MaxConcurrentCompactions: func() int { return 8 },
	})
	if err != nil {
		panic(err)
	}
	pb := &PebbleCache[K, V]{
		db:              db,
		keySerializer:   c.KeySerializer,
		valueSerializer: c.ValueSerializer,
	}
	client, err := theine.NewBuilder[K, V](int64(cap)).Hybrid(pb).Workers(8).Build()
	if err != nil {
		panic(err)
	}
	c.client = client
	c.db = db
}

func (c *TheinePebble[K, V]) Get(key K) (V, bool) {
	v, ok, err := c.client.Get(key)
	if err != nil {
		panic(err)
	}
	return v, ok
}

func (c *TheinePebble[K, V]) Set(key K, value V) {
	success := c.client.Set(key, value, 1)
	if !success {
		panic("set failed")
	}
}
func (c *TheinePebble[K, V]) Name() string {
	return "theine"
}

func (c *TheinePebble[K, V]) Close() {
	c.client.Close()
	_ = os.Remove("foobar")
}

func (c *TheinePebble[K, V]) Metrics() string {
	return c.db.Metrics().String()
}
