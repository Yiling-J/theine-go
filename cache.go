package theine

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/Yiling-J/theine-go/internal"
)

const (
	ZERO_TTL = 0 * time.Second
)

var VersionMismatch = internal.VersionMismatch

type RemoveReason = internal.RemoveReason
type DataBlock = internal.DataBlock[any]

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

// Cost adds dynamic cost function to builder.
// There is a default cost function which always return 1.
func (b *Builder[K, V]) Cost(cost func(v V) int64) *Builder[K, V] {
	b.cost = cost
	return b
}

// Doorkeeper enables doorkeeper.
// Doorkeeper will drop Set if they are not in bloomfilter yet.
func (b *Builder[K, V]) Doorkeeper(enabled bool) *Builder[K, V] {
	b.doorkeeper = true
	return b
}

// RemovalListener adds remove callback function to builder.
// This function is called when entry in cache is evicted/expired/deleted.
func (b *Builder[K, V]) RemovalListener(listener func(key K, value V, reason RemoveReason)) *Builder[K, V] {
	b.removalListener = listener
	return b
}

// Build builds a cache client from builder.
func (b *Builder[K, V]) Build() (*Cache[K, V], error) {
	if b.maxsize <= 0 {
		return nil, errors.New("size must be positive")
	}
	store := internal.NewStore(b.maxsize, b.doorkeeper, b.removalListener, b.cost, nil)
	return &Cache[K, V]{store: store}, nil
}

// BuildWithLoader builds a loading cache client from builder with custom loader function.
func (b *Builder[K, V]) BuildWithLoader(loader func(ctx context.Context, key K) (Loaded[V], error)) (*LoadingCache[K, V], error) {
	if b.maxsize <= 0 {
		return nil, errors.New("size must be positive")
	}
	if loader == nil {
		return nil, errors.New("loader function required")
	}
	store := internal.NewStore(b.maxsize, b.doorkeeper, b.removalListener, b.cost, nil)
	loadingStore := internal.NewLoadingStore(store)
	loadingStore.Loader(func(ctx context.Context, key K) (internal.Loaded[V], error) {
		v, err := loader(ctx, key)
		return v.internal(), err
	})
	return &LoadingCache[K, V]{store: loadingStore}, nil
}

// BuildHybrid builds a hybrid cache client from builder.
func (b *Builder[K, V]) BuildHybrid(cache internal.SecondaryCache[K, V]) (*HybridCache[K, V], error) {
	store := internal.NewStore(b.maxsize, b.doorkeeper, b.removalListener, b.cost,
		cache,
	)
	return &HybridCache[K, V]{store: store}, nil
}

type Cache[K comparable, V any] struct {
	store *internal.Store[K, V]
}

// Get gets value by key.
func (c *Cache[K, V]) Get(key K) (V, bool) {
	return c.store.Get(key)
}

// Set inserts or updates entry in cache with given ttl.
// Return false when cost > max size.
func (c *Cache[K, V]) SetWithTTL(key K, value V, cost int64, ttl time.Duration) bool {
	return c.store.Set(key, value, cost, ttl)
}

// Set inserts or updates entry in cache.
// Return false when cost > max size.
func (c *Cache[K, V]) Set(key K, value V, cost int64) bool {
	return c.SetWithTTL(key, value, cost, ZERO_TTL)
}

// Delete deletes key from cache.
func (c *Cache[K, V]) Delete(key K) {
	c.store.Delete(key)
}

// Range calls f sequentially for each key and value present in the cache.
// If f returns false, range stops the iteration.
func (c *Cache[K, V]) Range(f func(key K, value V) bool) {
	c.store.Range(f)
}

// Len returns number of entries in cache.
func (c *Cache[K, V]) Len() int {
	return c.store.Len()
}

// Close closes all goroutines created by cache.
func (c *Cache[K, V]) Close() {
	c.store.Close()
}

// SaveCache save cache data to writer.
func (c *Cache[K, V]) SaveCache(version uint64, writer io.Writer) error {
	return c.store.Persist(version, writer)
}

// LoadCache load cache data from reader.
func (c *Cache[K, V]) LoadCache(version uint64, reader io.Reader) error {
	return c.store.Recover(version, reader)
}

type LoadingCache[K comparable, V any] struct {
	store *internal.LoadingStore[K, V]
}

// Get gets value by key.
func (c *LoadingCache[K, V]) Get(ctx context.Context, key K) (V, error) {
	return c.store.Get(ctx, key)
}

// Set inserts or updates entry in cache with given ttl.
// Return false when cost > max size.
func (c *LoadingCache[K, V]) SetWithTTL(key K, value V, cost int64, ttl time.Duration) bool {
	return c.store.Set(key, value, cost, ttl)
}

// Set inserts or updates entry in cache.
// Return false when cost > max size.
func (c *LoadingCache[K, V]) Set(key K, value V, cost int64) bool {
	return c.SetWithTTL(key, value, cost, ZERO_TTL)
}

// Delete deletes key from cache.
func (c *LoadingCache[K, V]) Delete(key K) {
	c.store.Delete(key)
}

// Range calls f sequentially for each key and value present in the cache.
// If f returns false, range stops the iteration.
func (c *LoadingCache[K, V]) Range(f func(key K, value V) bool) {
	c.store.Range(f)
}

// Len returns number of entries in cache.
func (c *LoadingCache[K, V]) Len() int {
	return c.store.Len()
}

// SaveCache save cache data to writer.
func (c *LoadingCache[K, V]) SaveCache(version uint64, writer io.Writer) error {
	return c.store.Persist(version, writer)
}

// LoadCache load cache data from reader.
func (c *LoadingCache[K, V]) LoadCache(version uint64, reader io.Reader) error {
	return c.store.Recover(version, reader)
}

// Close closes all goroutines created by cache.
func (c *LoadingCache[K, V]) Close() {
	c.store.Close()
}

type Serializer[T any] interface {
	internal.Serializer[T]
}

type HybridCache[K comparable, V any] struct {
	store *internal.Store[K, V]
}

// Get gets value by key.
func (c *HybridCache[K, V]) Get(key K) (V, bool, error) {
	return c.store.GetWithSecodary(key)
}

// Set inserts or updates entry in cache with given ttl.
// Return false when cost > max size.
func (c *HybridCache[K, V]) SetWithTTL(key K, value V, cost int64, ttl time.Duration) (bool, error) {
	return c.store.SetWithSecondary(key, value, cost, ttl)
}

// Set inserts or updates entry in cache.
// Return false when cost > max size.
func (c *HybridCache[K, V]) Set(key K, value V, cost int64) (bool, error) {
	return c.SetWithTTL(key, value, cost, ZERO_TTL)
}

// Delete deletes key from cache.
func (c *HybridCache[K, V]) Delete(key K) error {
	return c.store.DeleteWithSecondary(key)
}

// Close closes all goroutines created by cache.
func (c *HybridCache[K, V]) Close() {
}
