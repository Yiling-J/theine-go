package theine_test

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Yiling-J/theine-go"
	"github.com/stretchr/testify/require"
)

func TestLoadingCacheGetSetParallel(t *testing.T) {
	client, err := theine.NewBuilder[string, string](1000).BuildWithLoader(
		func(ctx context.Context, key string) (theine.Loaded[string], error) {
			return theine.Loaded[string]{Value: key}, nil
		}, true,
	)
	require.Nil(t, err)
	var wg sync.WaitGroup
	for i := 1; i <= 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx := context.TODO()
			for i := 0; i < 10000; i++ {
				key := fmt.Sprintf("key:%d", rand.Intn(3000))
				v, err := client.Get(ctx, key)
				require.Nil(t, err)
				require.Equal(t, key, v)
			}
		}()
	}
	wg.Wait()
	time.Sleep(300 * time.Millisecond)
	require.True(t, client.Len() < 1200)
	client.Close()
}

func TestLoadingCacheSetWithTTL(t *testing.T) {
	counter := 0
	client, err := theine.NewBuilder[string, string](500).BuildWithLoader(
		func(ctx context.Context, key string) (theine.Loaded[string], error) {
			counter++
			return theine.Loaded[string]{Value: key, TTL: 1 * time.Second}, nil
		}, true,
	)
	require.Nil(t, err)
	v, err := client.Get(context.TODO(), "foo")
	require.Nil(t, err)
	require.Equal(t, "foo", v)
	require.Equal(t, 1, client.Len())
	require.Equal(t, 1, counter)

	time.Sleep(2 * time.Second)

	v, err = client.Get(context.TODO(), "foo")
	require.Nil(t, err)
	require.Equal(t, "foo", v)
	require.Equal(t, 1, client.Len())
	require.Equal(t, 2, counter)
	client.Close()

	counter = 0
	client, err = theine.NewBuilder[string, string](500).BuildWithLoader(
		func(ctx context.Context, key string) (theine.Loaded[string], error) {
			counter++
			return theine.Loaded[string]{Value: key, TTL: 10 * time.Second}, nil
		}, true,
	)
	require.Nil(t, err)
	v, err = client.Get(context.TODO(), "foo")
	require.Nil(t, err)
	require.Equal(t, "foo", v)
	require.Equal(t, 1, client.Len())
	require.Equal(t, 1, counter)

	time.Sleep(2 * time.Second)

	v, err = client.Get(context.TODO(), "foo")
	require.Nil(t, err)
	require.Equal(t, "foo", v)
	require.Equal(t, 1, client.Len())
	require.Equal(t, 1, counter)
	client.Close()
}

func TestLoadingCacheSetWithTTLAutoExpire(t *testing.T) {
	client, err := theine.NewBuilder[string, string](500).BuildWithLoader(
		func(ctx context.Context, key string) (theine.Loaded[string], error) {
			return theine.Loaded[string]{Value: key, TTL: 5 * time.Second}, nil
		}, true,
	)
	require.Nil(t, err)
	for i := 0; i < 30; i++ {
		key := fmt.Sprintf("key:%d", i)
		v, err := client.Get(context.TODO(), key)
		require.Nil(t, err)
		require.Equal(t, key, v)
	}
	for {
		time.Sleep(5 * time.Second)
		if client.Len() == 0 {
			break
		}
	}
	client.Close()
}

func TestLoadingCache(t *testing.T) {
	builder := theine.NewBuilder[int, int](100)
	counter := map[int]*atomic.Uint32{1: {}, 2: {}, 3: {}}
	client, err := builder.BuildWithLoader(func(ctx context.Context, key int) (theine.Loaded[int], error) {
		time.Sleep(40 * time.Millisecond)
		counter[key].Add(1)
		return theine.Loaded[int]{Value: key, Cost: 1, TTL: theine.ZERO_TTL}, nil
	}, false)
	require.Nil(t, err)
	var wg sync.WaitGroup
	for i := 1; i <= 2000; i++ {
		wg.Add(1)
		go func() {
			ctx := context.TODO()
			defer wg.Done()
			v, err := client.Get(ctx, 1)
			if err != nil || v != 1 {
				panic("")
			}
			v, err = client.Get(ctx, 2)
			if err != nil || v != 2 {
				panic("")
			}
			v, err = client.Get(ctx, 3)
			if err != nil || v != 3 {
				panic("")
			}
		}()
	}
	wg.Wait()
	c1 := counter[1]
	c2 := counter[2]
	c3 := counter[3]
	require.True(t, c1.Load() == 2000)
	require.True(t, c2.Load() == 2000)
	require.True(t, c3.Load() == 2000)

}

func TestLoadingCacheSingleFlight(t *testing.T) {
	builder := theine.NewBuilder[int, int](100)
	counter := map[int]*atomic.Uint32{1: {}, 2: {}, 3: {}}
	client, err := builder.BuildWithLoader(func(ctx context.Context, key int) (theine.Loaded[int], error) {
		time.Sleep(40 * time.Millisecond)
		counter[key].Add(1)
		return theine.Loaded[int]{Value: key, Cost: 1, TTL: theine.ZERO_TTL}, nil
	}, true)
	require.Nil(t, err)
	var wg sync.WaitGroup
	for i := 1; i <= 2000; i++ {
		wg.Add(1)
		go func() {
			ctx := context.TODO()
			defer wg.Done()
			v, err := client.Get(ctx, 1)
			if err != nil || v != 1 {
				panic("")
			}
			v, err = client.Get(ctx, 2)
			if err != nil || v != 2 {
				panic("")
			}
			v, err = client.Get(ctx, 3)
			if err != nil || v != 3 {
				panic("")
			}
		}()
	}
	wg.Wait()
	c1 := counter[1]
	c2 := counter[2]
	c3 := counter[3]
	require.True(t, c1.Load() == 1)
	require.True(t, c2.Load() == 1)
	require.True(t, c3.Load() == 1)

}
