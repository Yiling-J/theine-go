package theine_test

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Yiling-J/theine-go"
	"github.com/stretchr/testify/require"
)

func TestBuildError(t *testing.T) {
	_, err := theine.NewBuilder[string, string](0).BuildWithLoader(
		func(ctx context.Context, key string) (theine.Loaded[string], error) {
			return theine.Loaded[string]{Value: key}, nil
		},
	)
	require.NotNil(t, err)
	_, err = theine.NewBuilder[string, string](100).BuildWithLoader(nil)
	require.NotNil(t, err)
}

func TestLoadingCacheGetSetParallel(t *testing.T) {
	client, err := theine.NewBuilder[string, string](1000).BuildWithLoader(
		func(ctx context.Context, key string) (theine.Loaded[string], error) {
			return theine.Loaded[string]{Value: key}, nil
		},
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
		},
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
		},
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
		},
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
	counter := atomic.Uint32{}
	client, err := builder.BuildWithLoader(func(ctx context.Context, key int) (theine.Loaded[int], error) {
		time.Sleep(50 * time.Millisecond)
		counter.Add(1)
		return theine.Loaded[int]{Value: key, Cost: 1, TTL: theine.ZERO_TTL}, nil
	})
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
		}()
	}
	wg.Wait()
	require.True(t, counter.Load() < 50)

	success := client.Set(9999, 9999, 1)
	require.True(t, success)
	value, err := client.Get(context.TODO(), 9999)
	require.Nil(t, err)
	require.Equal(t, 9999, value)
	client.Delete(9999)
	require.Nil(t, err)
	value, err = client.Get(context.TODO(), 9999)
	require.Nil(t, err)
	require.Equal(t, 9999, value)
	success = client.SetWithTTL(9999, 9999, 1, 5*time.Second)
	require.True(t, success)

}

func TestLoadError(t *testing.T) {
	builder := theine.NewBuilder[int, int](100)
	client, err := builder.BuildWithLoader(func(ctx context.Context, key int) (theine.Loaded[int], error) {
		if key != 1 {
			return theine.Loaded[int]{}, errors.New("error")
		}
		return theine.Loaded[int]{Value: key, Cost: 1, TTL: theine.ZERO_TTL}, nil
	})
	require.Nil(t, err)
	_, err = client.Get(context.TODO(), 2)
	require.NotNil(t, err)
}

func TestLoadingCost(t *testing.T) {
	// test cost func
	builder := theine.NewBuilder[string, string](500)
	builder.Cost(func(v string) int64 {
		return int64(len(v))
	})
	client, err := builder.BuildWithLoader(func(ctx context.Context, key string) (theine.Loaded[string], error) {
		return theine.Loaded[string]{Value: key, Cost: 1, TTL: 0}, nil
	})
	require.Nil(t, err)
	success := client.Set("z", strings.Repeat("z", 501), 0)
	require.False(t, success)
	for i := 0; i < 30; i++ {
		key := fmt.Sprintf("key:%d", i)
		success = client.Set(key, strings.Repeat("z", 20), 0)
		require.True(t, success)
	}
	time.Sleep(time.Second)
	require.True(t, client.Len() == 25)
	client.Close()
}

func TestLoadingDoorkeeper(t *testing.T) {
	builder := theine.NewBuilder[string, string](500)
	builder.Doorkeeper(true)
	client, err := builder.BuildWithLoader(func(ctx context.Context, key string) (theine.Loaded[string], error) {
		return theine.Loaded[string]{Value: key, Cost: 1, TTL: 0}, nil
	})
	require.Nil(t, err)
	for i := 0; i < 200; i++ {
		key := fmt.Sprintf("key:%d", i)
		success := client.Set(key, key, 1)
		require.False(t, success)
	}
	require.True(t, client.Len() == 0)
	time.Sleep(time.Second)
	for i := 0; i < 200; i++ {
		key := fmt.Sprintf("key:%d", i)
		success := client.Set(key, key, 1)
		require.True(t, success)
	}
	require.True(t, client.Len() > 0)
	for i := 0; i < 500000; i++ {
		key := fmt.Sprintf("key:%d:2", i)
		client.Set(key, key, 1)
	}
}

func TestLoadingRemovalListener(t *testing.T) {
	builder := theine.NewBuilder[int, int](100)
	var lock sync.Mutex
	removed := map[int]int{}
	evicted := map[int]int{}
	expired := map[int]int{}
	builder.RemovalListener(func(key, value int, reason theine.RemoveReason) {
		lock.Lock()
		defer lock.Unlock()
		switch reason {
		case theine.REMOVED:
			removed[key] = value
		case theine.EVICTED:
			evicted[key] = value
		case theine.EXPIRED:
			expired[key] = value
		}
	})
	client, err := builder.BuildWithLoader(func(ctx context.Context, key int) (theine.Loaded[int], error) {
		return theine.Loaded[int]{Value: key, Cost: 1, TTL: 0}, nil
	})
	require.Nil(t, err)
	for i := 0; i < 100; i++ {
		success := client.Set(i, i, 1)
		require.True(t, success)
	}
	// this will evict one entry: 0
	success := client.Set(100, 100, 1)
	require.True(t, success)
	time.Sleep(100 * time.Millisecond)
	lock.Lock()
	require.Equal(t, 1, len(evicted))
	require.True(t, evicted[0] == 0)
	lock.Unlock()
	// manually remove one
	client.Delete(5)
	time.Sleep(100 * time.Millisecond)
	lock.Lock()
	require.Equal(t, 1, len(removed))
	require.True(t, removed[5] == 5)
	lock.Unlock()
	// expire one
	for i := 0; i < 100; i++ {
		success := client.SetWithTTL(i+100, i+100, 1, 1*time.Second)
		require.True(t, success)
	}
	time.Sleep(5 * time.Second)
	lock.Lock()
	require.True(t, len(expired) > 0)
	lock.Unlock()
}

func TestLoadingRange(t *testing.T) {
	for _, cap := range []int{100, 200000} {
		client, err := theine.NewBuilder[int, int](int64(cap)).BuildWithLoader(func(ctx context.Context, key int) (theine.Loaded[int], error) {
			return theine.Loaded[int]{Value: key, Cost: 1, TTL: 0}, nil
		})
		require.Nil(t, err)
		for i := 0; i < 100; i++ {
			success := client.Set(i, i, 1)
			require.True(t, success)
		}
		data := map[int]int{}
		client.Range(func(key, value int) bool {
			data[key] = value
			return true
		})
		require.Equal(t, 100, len(data))
		for i := 0; i < 100; i++ {
			require.Equal(t, i, data[i])
		}
		data = map[int]int{}
		client.Range(func(key, value int) bool {
			data[key] = value
			return len(data) < 20
		})
		require.Equal(t, 20, len(data))
	}
}

func TestLoadingGetSetDeleteNoRace(t *testing.T) {
	for _, size := range []int{500, 100000} {
		client, err := theine.NewBuilder[string, string](int64(size)).BuildWithLoader(func(ctx context.Context, key string) (theine.Loaded[string], error) {
			return theine.Loaded[string]{Value: key, Cost: 1, TTL: 0}, nil
		})
		ctx := context.TODO()
		require.Nil(t, err)
		var wg sync.WaitGroup
		keys := []string{}
		for i := 0; i < 100000; i++ {
			keys = append(keys, fmt.Sprintf("%d", rand.Intn(1000000)))
		}
		for i := 1; i <= 20; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for i := 0; i < 100000; i++ {
					key := keys[i]
					v, err := client.Get(ctx, key)
					if err != nil || v != key {
						panic(key)
					}
					if i%3 == 0 {
						client.SetWithTTL(key, key, 1, time.Second*time.Duration(i%25+5))
					}
					if i%5 == 0 {
						client.Delete(key)
					}
					if i%5000 == 0 {
						client.Range(func(key, value string) bool {
							return true
						})
					}
				}
			}()
		}
		wg.Wait()
		time.Sleep(300 * time.Millisecond)
		require.True(t, client.Len() < size+50)
		client.Close()
	}
}
