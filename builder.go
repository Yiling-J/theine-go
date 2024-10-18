package theine

import (
	"context"
	"errors"

	"github.com/Yiling-J/theine-go/internal"
)

type params interface {
	validate() error
}

func validateParams(params ...params) error {
	for _, p := range params {
		if err := p.validate(); err != nil {
			return err
		}
	}
	return nil
}

type baseParams[K comparable, V any] struct {
	cost            func(V) int64
	removalListener func(key K, value V, reason RemoveReason)
	maxsize         int64
	doorkeeper      bool
	stringKeyFunc   func(key K) string
	useEntryPool    bool
}

func (p *baseParams[K, V]) validate() error {
	if p.maxsize <= 0 {
		return errors.New("size must be positive")
	}
	return nil
}

type loadingParams[K comparable, V any] struct {
	loader func(ctx context.Context, key K) (Loaded[V], error)
}

func (p *loadingParams[K, V]) validate() error {
	if p.loader == nil {
		return errors.New("loader function required")
	}
	return nil
}

type hybridParams[K comparable, V any] struct {
	secondaryCache internal.SecondaryCache[K, V]
	workers        int
	admProbability float32
}

func (p *hybridParams[K, V]) validate() error {
	if p.secondaryCache == nil {
		return errors.New("secondary cache required")
	}
	if p.workers <= 0 {
		return errors.New("workers must be positive")
	}
	return nil
}

type Builder[K comparable, V any] struct {
	baseParams[K, V]
}

func NewBuilder[K comparable, V any](maxsize int64) *Builder[K, V] {
	b := &Builder[K, V]{}
	b.maxsize = maxsize
	return b
}

// StringKey add a custom key -> string method, the string will be used in shard hashing.
func (b *Builder[K, V]) StringKey(fn func(k K) string) *Builder[K, V] {
	b.stringKeyFunc = fn
	return b
}

// Cost adds dynamic cost function to builder.
// There is a default cost function which always return 1.
func (b *Builder[K, V]) Cost(cost func(v V) int64) *Builder[K, V] {
	b.cost = cost
	return b
}

// Doorkeeper enables/disables doorkeeper.
// Doorkeeper will drop Set if they are not in bloomfilter yet.
func (b *Builder[K, V]) Doorkeeper(enabled bool) *Builder[K, V] {
	b.doorkeeper = enabled
	return b
}

// UseEntryPool enables/disables reusing evicted entries through a sync pool.
// This can significantly reduce memory allocation under heavy concurrent writes,
// but it may lead to occasional race conditions. Theine updates its policy asynchronously,
// so when an Update event is processed, the corresponding entry might have already been reused.
// Theine will compare the key again, but this does not completely eliminate the risk of a race.
func (b *Builder[K, V]) UseEntryPool(enabled bool) *Builder[K, V] {
	b.useEntryPool = enabled
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
	if err := validateParams(&b.baseParams); err != nil {
		return nil, err
	}
	store := internal.NewStore(b.maxsize, b.doorkeeper, b.useEntryPool, b.removalListener, b.cost, nil, 0, 0, b.stringKeyFunc)
	return &Cache[K, V]{store: store}, nil
}

// Add loading function and switch to LoadingBuilder.
func (b *Builder[K, V]) Loading(
	loader func(ctx context.Context, key K) (Loaded[V], error),
) *LoadingBuilder[K, V] {
	return &LoadingBuilder[K, V]{
		baseParams: b.baseParams,
		loadingParams: loadingParams[K, V]{
			loader: loader,
		},
	}
}

// Add secondary cache and switch to HybridBuilder.
func (b *Builder[K, V]) Hybrid(cache internal.SecondaryCache[K, V]) *HybridBuilder[K, V] {
	return &HybridBuilder[K, V]{
		baseParams: b.baseParams,
		hybridParams: hybridParams[K, V]{
			secondaryCache: cache,
			workers:        2,
			admProbability: 1,
		},
	}
}

// BuildWithLoader builds a loading cache client from builder with custom loader function.
func (b *Builder[K, V]) BuildWithLoader(loader func(ctx context.Context, key K) (Loaded[V], error)) (*LoadingCache[K, V], error) {
	if b.maxsize <= 0 {
		return nil, errors.New("size must be positive")
	}
	if loader == nil {
		return nil, errors.New("loader function required")
	}
	store := internal.NewStore(b.maxsize, b.doorkeeper, b.useEntryPool, b.removalListener, b.cost, nil, 0, 0, b.stringKeyFunc)
	loadingStore := internal.NewLoadingStore(store)
	loadingStore.Loader(func(ctx context.Context, key K) (internal.Loaded[V], error) {
		v, err := loader(ctx, key)
		return v.internal(), err
	})
	return &LoadingCache[K, V]{store: loadingStore}, nil
}

type LoadingBuilder[K comparable, V any] struct {
	baseParams[K, V]
	loadingParams[K, V]
}

// Add secondary cache and switch to HybridLoadingBuilder.
func (b *LoadingBuilder[K, V]) Hybrid(cache internal.SecondaryCache[K, V]) *HybridLoadingBuilder[K, V] {
	return &HybridLoadingBuilder[K, V]{
		baseParams:    b.baseParams,
		loadingParams: b.loadingParams,
		hybridParams: hybridParams[K, V]{
			secondaryCache: cache,
			workers:        2,
		},
	}
}

// Build builds a cache client from builder.
func (b *LoadingBuilder[K, V]) Build() (*LoadingCache[K, V], error) {
	if err := validateParams(&b.baseParams, &b.loadingParams); err != nil {
		return nil, err
	}
	store := internal.NewStore(b.maxsize, b.doorkeeper, b.useEntryPool, b.removalListener, b.cost, nil, 0, 0, b.stringKeyFunc)
	loadingStore := internal.NewLoadingStore(store)
	loadingStore.Loader(func(ctx context.Context, key K) (internal.Loaded[V], error) {
		v, err := b.loader(ctx, key)
		return v.internal(), err
	})
	return &LoadingCache[K, V]{store: loadingStore}, nil
}

type HybridBuilder[K comparable, V any] struct {
	baseParams[K, V]
	hybridParams[K, V]
}

// Set secondary cache workers.
// Worker will send evicted entries to secondary cache.
func (b *HybridBuilder[K, V]) Workers(w int) *HybridBuilder[K, V] {
	b.workers = w
	return b
}

// Set acceptance probability. The value has to be in the range of [0, 1].
func (b *HybridBuilder[K, V]) AdmProbability(p float32) *HybridBuilder[K, V] {
	b.admProbability = p
	return b
}

// Add loading function and switch to HybridLoadingBuilder.
func (b *HybridBuilder[K, V]) Loading(
	loader func(ctx context.Context, key K) (Loaded[V], error),
) *HybridLoadingBuilder[K, V] {
	return &HybridLoadingBuilder[K, V]{
		baseParams: b.baseParams,
		loadingParams: loadingParams[K, V]{
			loader: loader,
		},
		hybridParams: b.hybridParams,
	}
}

// Build builds a cache client from builder.
func (b *HybridBuilder[K, V]) Build() (*HybridCache[K, V], error) {
	if err := validateParams(&b.baseParams, &b.hybridParams); err != nil {
		return nil, err
	}
	store := internal.NewStore(b.maxsize, b.doorkeeper, b.useEntryPool, b.removalListener, b.cost,
		b.secondaryCache, b.workers, b.admProbability, b.stringKeyFunc,
	)
	return &HybridCache[K, V]{store: store}, nil
}

type HybridLoadingBuilder[K comparable, V any] struct {
	baseParams[K, V]
	loadingParams[K, V]
	hybridParams[K, V]
}

// Build builds a cache client from builder.
func (b *HybridLoadingBuilder[K, V]) Build() (*HybridLoadingCache[K, V], error) {
	if err := validateParams(&b.baseParams, &b.hybridParams, &b.loadingParams); err != nil {
		return nil, err
	}
	store := internal.NewStore(
		b.maxsize, b.doorkeeper, b.useEntryPool, b.removalListener, b.cost, b.secondaryCache, b.workers,
		b.admProbability, b.stringKeyFunc,
	)
	loadingStore := internal.NewLoadingStore(store)
	loadingStore.Loader(func(ctx context.Context, key K) (internal.Loaded[V], error) {
		v, err := b.loader(ctx, key)
		return v.internal(), err
	})
	return &HybridLoadingCache[K, V]{store: loadingStore}, nil

}
