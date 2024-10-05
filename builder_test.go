package theine_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/Yiling-J/theine-go"
	"github.com/Yiling-J/theine-go/internal"
	"github.com/stretchr/testify/require"
)

func TestBuilder(t *testing.T) {
	// simple cache
	_, err := theine.NewBuilder[int, int](-500).Build()
	require.Error(t, err)
	builder := theine.NewBuilder[int, int](100)
	builder = builder.Cost(func(v int) int64 { return 1 })
	builder = builder.Doorkeeper(false)
	builder = builder.RemovalListener(func(key, value int, reason theine.RemoveReason) {})

	cache, err := builder.Build()
	require.Nil(t, err)
	require.Equal(t, reflect.TypeOf(&theine.Cache[int, int]{}), reflect.TypeOf(cache))

	// loading cache
	_, err = builder.Loading(nil).Build()
	require.Error(t, err)
	builderL := builder.Loading(func(ctx context.Context, key int) (theine.Loaded[int], error) {
		return theine.Loaded[int]{}, nil
	})
	cacheL, err := builderL.Build()
	require.Nil(t, err)
	require.Equal(t, reflect.TypeOf(&theine.LoadingCache[int, int]{}), reflect.TypeOf(cacheL))

	// hybrid cache
	_, err = builder.Hybrid(nil).Build()
	require.Error(t, err)
	secondary := internal.NewSimpleMapSecondary[int, int]()
	_, err = builder.Hybrid(secondary).Workers(0).Build()
	require.Error(t, err)
	builderH := builder.Hybrid(secondary).Workers(1).AdmProbability(0.8)
	cacheH, err := builderH.Build()
	require.Nil(t, err)
	require.Equal(t, reflect.TypeOf(&theine.HybridCache[int, int]{}), reflect.TypeOf(cacheH))

	// loading + hybrid
	builderLH := builderL.Hybrid(secondary)
	cacheLH, err := builderLH.Build()
	require.Nil(t, err)
	require.Equal(t, reflect.TypeOf(&theine.HybridLoadingCache[int, int]{}), reflect.TypeOf(cacheLH))

	// hybrid + loading
	builderLH = builderH.Workers(8).Loading(
		func(ctx context.Context, key int) (theine.Loaded[int], error) {
			return theine.Loaded[int]{}, nil
		})
	cacheLH, err = builderLH.Build()
	require.Nil(t, err)
	require.Equal(t, reflect.TypeOf(&theine.HybridLoadingCache[int, int]{}), reflect.TypeOf(cacheLH))
}
