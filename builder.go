package theine

import (
	"context"
	"errors"

	"github.com/Yiling-J/theine-go/internal"
)

type baseParams[K comparable, V any] struct {
	cost            func(V) int64
	removalListener func(key K, value V, reason RemoveReason)
	maxsize         int64
	doorkeeper      bool
}

type loadingParams[K comparable, V any] struct {
	loader func(ctx context.Context, key K) (Loaded[V], error)
}

type hybridParams[K comparable, V any] struct {
	secondaryCache internal.SecondaryCache[K, V]
	workers        int
}

type Builder[K comparable, V any] struct {
	baseParams[K, V]
}

func NewBuilder[K comparable, V any](maxsize int64) *Builder[K, V] {
	b := &Builder[K, V]{}
	b.maxsize = maxsize
	return b
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
	store := internal.NewStore(b.maxsize, b.doorkeeper, b.removalListener, b.cost, nil, 0)
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
	store := internal.NewStore(b.maxsize, b.doorkeeper, b.removalListener, b.cost, nil, 0)
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
	if b.maxsize <= 0 {
		return nil, errors.New("size must be positive")
	}
	if b.loader == nil {
		return nil, errors.New("loader function required")
	}
	store := internal.NewStore(b.maxsize, b.doorkeeper, b.removalListener, b.cost, nil, 0)
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
	if b.workers <= 0 {
		return nil, errors.New("workers must be positive")
	}
	store := internal.NewStore(b.maxsize, b.doorkeeper, b.removalListener, b.cost,
		b.secondaryCache, b.workers,
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
	if b.maxsize <= 0 {
		return nil, errors.New("size must be positive")
	}
	if b.loader == nil {
		return nil, errors.New("loader function required")
	}
	if b.workers <= 0 {
		return nil, errors.New("workers must be positive")
	}
	store := internal.NewStore(
		b.maxsize, b.doorkeeper, b.removalListener, b.cost, b.secondaryCache, b.workers,
	)
	loadingStore := internal.NewLoadingStore(store)
	loadingStore.Loader(func(ctx context.Context, key K) (internal.Loaded[V], error) {
		v, err := b.loader(ctx, key)
		return v.internal(), err
	})
	return &HybridLoadingCache[K, V]{store: loadingStore}, nil

}
