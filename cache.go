package theine

import (
	"context"
	"errors"
	"time"

	"github.com/Yiling-J/theine-go/internal"
)

const (
	ZERO_TTL = 0 * time.Second
)

type RemoveReason = internal.RemoveReason

type Loaded[V any] struct {
	Value V
	Cost  int64
	TTL   time.Duration
}

func (l *Loaded[V]) internal() internal.Loaded[V] {
	return internal.Loaded[V]{
		Value: l.Value,
		Cost:  l.Cost,
		TTL:   l.TTL,
	}
}

const (
	REMOVED = internal.REMOVED
	EVICTED = internal.EVICTED
	EXPIRED = internal.EXPIRED
)

type Builder[K comparable, V any] struct {
	cost            func(V) int64
	removalListener func(key K, value V, reason RemoveReason)
	maxsize         int64
	doorkeeper      bool
}

func NewBuilder[K comparable, V any](maxsize int64) *Builder[K, V] {
	return &Builder[K, V]{maxsize: maxsize}
}

func (b *Builder[K, V]) Cost(cost func(v V) int64) *Builder[K, V] {
	b.cost = cost
	return b
}
func (b *Builder[K, V]) Doorkeeper(enabled bool) *Builder[K, V] {
	b.doorkeeper = true
	return b
}

func (b *Builder[K, V]) RemovalListener(listener func(key K, value V, reason RemoveReason)) *Builder[K, V] {
	b.removalListener = listener
	return b
}

func (b *Builder[K, V]) Build() (*Cache[K, V], error) {
	if b.maxsize <= 0 {
		return nil, errors.New("size must be positive")
	}
	store := internal.NewStore[K, V](b.maxsize)
	if b.cost != nil {
		store.Cost(b.cost)
	}
	if b.removalListener != nil {
		store.RemovalListener(b.removalListener)
	}
	store.Doorkeeper(b.doorkeeper)
	return &Cache[K, V]{store: store}, nil
}

func (b *Builder[K, V]) BuildWithLoader(loader func(ctx context.Context, key K) (Loaded[V], error)) (*LoadingCache[K, V], error) {
	if b.maxsize <= 0 {
		return nil, errors.New("size must be positive")
	}
	if loader == nil {
		return nil, errors.New("loader function required")
	}
	store := internal.NewStore[K, V](b.maxsize)
	if b.cost != nil {
		store.Cost(b.cost)
	}
	if b.removalListener != nil {
		store.RemovalListener(b.removalListener)
	}
	store.Doorkeeper(b.doorkeeper)
	loadingStore := internal.NewLoadingStore(store)
	loadingStore.Loader(func(ctx context.Context, key K) (internal.Loaded[V], error) {
		v, err := loader(ctx, key)
		return v.internal(), err
	})
	return &LoadingCache[K, V]{store: loadingStore}, nil
}

type Cache[K comparable, V any] struct {
	store *internal.Store[K, V]
}

func (c *Cache[K, V]) Get(key K) (V, bool) {
	return c.store.Get(key)
}

func (c *Cache[K, V]) SetWithTTL(key K, value V, cost int64, ttl time.Duration) bool {
	return c.store.Set(key, value, cost, ttl)
}

func (c *Cache[K, V]) Set(key K, value V, cost int64) bool {
	return c.SetWithTTL(key, value, cost, ZERO_TTL)
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

type LoadingCache[K comparable, V any] struct {
	store *internal.LoadingStore[K, V]
}

func (c *LoadingCache[K, V]) Get(ctx context.Context, key K) (V, error) {
	return c.store.Get(ctx, key)
}

func (c *LoadingCache[K, V]) SetWithTTL(key K, value V, cost int64, ttl time.Duration) bool {
	return c.store.Set(key, value, cost, ttl)
}

func (c *LoadingCache[K, V]) Set(key K, value V, cost int64) bool {
	return c.SetWithTTL(key, value, cost, ZERO_TTL)
}

func (c *LoadingCache[K, V]) Delete(key K) {
	c.store.Delete(key)
}

func (c *LoadingCache[K, V]) Len() int {
	return c.store.Len()
}

func (c *LoadingCache[K, V]) Close() {
	c.store.Close()
}
