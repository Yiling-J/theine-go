package theine

import (
	"context"
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
func (c *HybridCache[K, V]) SetWithTTL(key K, value V, cost int64, ttl time.Duration) bool {
	return c.store.Set(key, value, cost, ttl)
}

// Set inserts or updates entry in cache.
// Return false when cost > max size.
func (c *HybridCache[K, V]) Set(key K, value V, cost int64) bool {
	return c.SetWithTTL(key, value, cost, ZERO_TTL)
}

// Delete deletes key from cache.
func (c *HybridCache[K, V]) Delete(key K) error {
	return c.store.DeleteWithSecondary(key)
}

// Close closes all goroutines created by cache.
func (c *HybridCache[K, V]) Close() {
}

type HybridLoadingCache[K comparable, V any] struct {
	store *internal.LoadingStore[K, V]
}

// Get gets value by key.
func (c *HybridLoadingCache[K, V]) Get(ctx context.Context, key K) (V, error) {
	return c.store.Get(ctx, key)
}

// Set inserts or updates entry in cache with given ttl.
// Return false when cost > max size.
func (c *HybridLoadingCache[K, V]) SetWithTTL(key K, value V, cost int64, ttl time.Duration) bool {
	return c.store.Set(key, value, cost, ttl)
}

// Set inserts or updates entry in cache.
// Return false when cost > max size.
func (c *HybridLoadingCache[K, V]) Set(key K, value V, cost int64) bool {
	return c.SetWithTTL(key, value, cost, ZERO_TTL)
}

// Delete deletes key from cache.
func (c *HybridLoadingCache[K, V]) Delete(key K) error {
	return c.store.DeleteWithSecondary(key)
}

// Range calls f sequentially for each key and value present in the cache.
// If f returns false, range stops the iteration.
func (c *HybridLoadingCache[K, V]) Range(f func(key K, value V) bool) {
	c.store.Range(f)
}

// Len returns number of entries in cache.
func (c *HybridLoadingCache[K, V]) Len() int {
	return c.store.Len()
}

// SaveCache save cache data to writer.
func (c *HybridLoadingCache[K, V]) SaveCache(version uint64, writer io.Writer) error {
	return c.store.Persist(version, writer)
}

// LoadCache load cache data from reader.
func (c *HybridLoadingCache[K, V]) LoadCache(version uint64, reader io.Reader) error {
	return c.store.Recover(version, reader)
}

// Close closes all goroutines created by cache.
func (c *HybridLoadingCache[K, V]) Close() {
	c.store.Close()
}
