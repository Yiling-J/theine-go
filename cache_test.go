package theine_test

import (
	"context"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Yiling-J/theine-go"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
)

func TestCache_MaxsizeZero(t *testing.T) {
	_, err := theine.NewBuilder[string, string](0).Build()
	require.NotNil(t, err)
}

func TestCache_Set(t *testing.T) {
	client, err := theine.NewBuilder[string, string](1000).Build()
	require.Nil(t, err)
	defer client.Close()
	for i := 0; i < 20000; i++ {
		key := fmt.Sprintf("key:%d", rand.Intn(100000))
		client.Set(key, key, 1)
	}
	client.Wait()
	require.Equal(t, 1000, client.Len())
}

func TestCache_Update(t *testing.T) {
	client, err := theine.NewBuilder[string, string](1000).Build()
	require.Nil(t, err)
	defer client.Close()
	key := "foo"
	for _, v := range []string{"a", "b", "c", "d", "e", "e"} {
		client.Set(key, v, 1)
		vn, ok := client.Get(key)
		require.True(t, ok)
		require.Equal(t, v, vn)
	}
}

func TestCache_SetParallel(t *testing.T) {
	client, err := theine.NewBuilder[string, string](1000).Build()
	require.Nil(t, err)
	defer client.Close()
	var wg sync.WaitGroup
	for i := 1; i <= 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 10000; i++ {
				key := fmt.Sprintf("key:%d", rand.Intn(100000))
				client.Set(key, key, 1)
			}
		}()
	}
	wg.Wait()
	client.Wait()
	require.Equal(t, 1000, client.Len())
}

func TestCache_GetSetGetDeleteGet(t *testing.T) {
	for _, entryPool := range []bool{false, true} {
		t.Run(fmt.Sprintf("entrypool enable %v", entryPool), func(t *testing.T) {
			client, err := theine.NewBuilder[string, string](50000).Build()
			require.Nil(t, err)
			defer client.Close()
			for i := 0; i < 20000; i++ {
				key := fmt.Sprintf("key:%d", rand.Intn(3000))
				_, ok := client.Get(key)
				require.False(t, ok)
				client.Set(key, key, 1)
				v, ok := client.Get(key)
				require.True(t, ok)
				require.Equal(t, key, v)
				client.Delete(key)
				_, ok = client.Get(key)
				require.False(t, ok)
			}
		})
	}
}

func TestCache_Delete(t *testing.T) {
	client, err := theine.NewBuilder[string, string](100).Build()
	require.Nil(t, err)
	defer client.Close()
	client.Set("foo", "foo", 1)
	v, ok := client.Get("foo")
	require.True(t, ok)
	require.Equal(t, "foo", v)
	client.Delete("foo")
	_, ok = client.Get("foo")
	require.False(t, ok)

	client.SetWithTTL("foo", "foo", 1, 10*time.Second)
	v, ok = client.Get("foo")
	require.True(t, ok)
	require.Equal(t, "foo", v)
	client.Delete("foo")
	_, ok = client.Get("foo")
	require.False(t, ok)
}

func TestCache_GetSetParallel(t *testing.T) {
	client, err := theine.NewBuilder[string, string](1000).Build()
	require.Nil(t, err)
	defer client.Close()
	var wg sync.WaitGroup
	for i := 1; i <= 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 10000; i++ {
				key := fmt.Sprintf("key:%d", rand.Intn(3000))
				v, ok := client.Get(key)
				if !ok {
					client.Set(key, key, 1)
				} else {
					require.Equal(t, key, v)
				}
			}
		}()
	}
	wg.Wait()
	client.Wait()
	require.Equal(t, 1000, client.Len())
}

func TestCache_SetWithTTL(t *testing.T) {
	client, err := theine.NewBuilder[string, string](500).Build()
	require.Nil(t, err)
	defer client.Close()
	client.SetWithTTL("foo", "foo", 1, 3600*time.Second)
	require.Equal(t, 1, client.Len())
	time.Sleep(1 * time.Second)
	client.SetWithTTL("foo", "foo", 1, 1*time.Second)
	require.Equal(t, 1, client.Len())
	time.Sleep(3 * time.Second)
	_, ok := client.Get("foo")
	require.False(t, ok)
}

func TestCache_SetWithTTLAutoExpire(t *testing.T) {
	client, err := theine.NewBuilder[string, string](500).Build()
	require.Nil(t, err)
	defer client.Close()
	for i := 0; i < 500; i++ {
		key1 := fmt.Sprintf("key:%d", i)
		client.SetWithTTL(key1, key1, 1, time.Second)
		key2 := fmt.Sprintf("key:%d:2", i)
		client.SetWithTTL(key2, key2, 1, 100*time.Second)
	}
	time.Sleep(3 * time.Second)
	require.True(t, client.Len() < 500)
	for i := 0; i < 500; i++ {
		key := fmt.Sprintf("key:%d", i)
		_, ok := client.Get(key)
		require.False(t, ok)
	}
}

func TestCache_Cost(t *testing.T) {
	client, err := theine.NewBuilder[string, string](500).Build()
	require.Nil(t, err)
	defer client.Close()

	success := client.Set("z", "z", 501)
	require.False(t, success)
	for i := 0; i < 30; i++ {
		key := fmt.Sprintf("key:%d", i)
		success = client.Set(key, key, 20)
		require.True(t, success)
	}
	client.Wait()
	require.True(t, client.Len() <= 25 && client.Len() >= 24)
	require.True(t, client.EstimatedSize() <= 500 && client.EstimatedSize() >= 480)

	// test cost func
	builder := theine.NewBuilder[string, string](500)
	builder.Cost(func(v string) int64 {
		return int64(len(v))
	})
	client2, err := builder.Build()
	require.Nil(t, err)
	defer client2.Close()
	success = client2.Set("z", strings.Repeat("z", 501), 0)
	require.False(t, success)
	for i := 0; i < 30; i++ {
		key := fmt.Sprintf("key:%d", i)
		success = client2.Set(key, strings.Repeat("z", 20), 0)
		require.True(t, success)
	}
	client2.Wait()
	require.True(t, client2.Len() <= 25 && client2.Len() >= 24)
	require.True(t, client2.EstimatedSize() <= 500 && client2.EstimatedSize() >= 480)
}

func TestCache_CostUpdate(t *testing.T) {
	client, err := theine.NewBuilder[string, string](500).Build()
	require.Nil(t, err)
	defer client.Close()
	for i := 0; i < 30; i++ {
		key := fmt.Sprintf("key:%d", i)
		success := client.Set(key, key, 20)
		require.True(t, success)
	}
	client.Wait()
	require.True(t, client.Len() <= 25 && client.Len() >= 24)
	require.True(t, client.EstimatedSize() <= 500 && client.EstimatedSize() >= 480)

	// update cost
	success := client.Set("key:15", "", 200)
	require.True(t, success)
	client.Wait()

	require.True(
		t, client.Len() <= 16 && client.Len() >= 15,
		fmt.Sprintf("length too large %d", client.Len()),
	)
	require.True(t, client.EstimatedSize() <= 500 && client.EstimatedSize() >= 480)
}

func TestCache_EstimatedSize(t *testing.T) {
	client, err := theine.NewBuilder[int, int](500).Build()
	require.Nil(t, err)
	defer client.Close()
	ctx, cfn := context.WithCancel(context.Background())
	defer cfn()
	wg, ctx := errgroup.WithContext(ctx)
	wg.Go(func() error {
		tkr := time.NewTicker(time.Nanosecond)
		defer tkr.Stop()
		for {
			select {
			case <-tkr.C:
				client.EstimatedSize()
			case <-ctx.Done():
				return nil
			}
		}
	})
	wg.Go(func() error {
		defer cfn()
		for i := 0; i < 1200; i++ {
			if i%2 == 0 {
				client.Set(i, 1, 1)
			} else {
				client.Get(i)
			}
		}
		return nil
	})
	require.Nil(t, wg.Wait())
	client.Wait()
	require.Equal(t, 500, client.EstimatedSize())
}

func TestCache_Doorkeeper(t *testing.T) {
	builder := theine.NewBuilder[string, string](500)
	builder.Doorkeeper(true)
	client, err := builder.Build()
	require.Nil(t, err)
	defer client.Close()
	for i := 0; i < 200; i++ {
		key := fmt.Sprintf("key:%d", i)
		success := client.Set(key, key, 1)
		require.False(t, success)
	}
	require.True(t, client.Len() == 0)
	client.Wait()
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

func TestCache_RemovalListener(t *testing.T) {
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
	client, err := builder.Build()
	require.Nil(t, err)
	defer client.Close()
	for i := 0; i < 100; i++ {
		success := client.Set(i, i, 1)
		require.True(t, success)
	}
	// this will evict one entry: 0
	success := client.Set(100, 100, 1)
	require.True(t, success)
	client.Wait()
	lock.Lock()
	require.Equal(t, 1, len(evicted))
	require.True(t, evicted[0] == 0)
	lock.Unlock()
	// manually remove one
	client.Delete(5)
	client.Wait()
	lock.Lock()
	require.Equal(t, 1, len(removed))
	require.True(t, removed[5] == 5)
	lock.Unlock()
	// expire one
	for i := 0; i < 100; i++ {
		success := client.SetWithTTL(i+100, i+100, 1, 1*time.Second)
		require.True(t, success)
	}
	client.Wait()
	time.Sleep(5 * time.Second)
	lock.Lock()
	require.True(t, len(expired) > 0)
	lock.Unlock()
}

func TestCache_Range(t *testing.T) {
	for _, cap := range []int{100, 200000} {
		client, err := theine.NewBuilder[int, int](int64(cap)).Build()
		require.Nil(t, err)
		defer client.Close()
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
		// return false
		data = map[int]int{}
		client.Range(func(key, value int) bool {
			data[key] = value
			return len(data) < 20
		})
		require.Equal(t, 20, len(data))
		// expired
		client, err = theine.NewBuilder[int, int](int64(cap)).Build()
		require.Nil(t, err)
		for i := 0; i < 50; i++ {
			success := client.Set(i, i, 1)
			require.True(t, success)
		}
		for i := 50; i < 100; i++ {
			success := client.SetWithTTL(i, i, 1, time.Second)
			require.True(t, success)
		}
		client.Wait()
		time.Sleep(2 * time.Second)
		data = map[int]int{}
		client.Range(func(key, value int) bool {
			data[key] = value
			return true
		})
		require.Equal(t, 50, len(data))
		for i := 0; i < 50; i++ {
			require.Equal(t, i, data[i])
		}
	}
}

type Foo struct {
	Bar string
}

func TestCache_StringKey(t *testing.T) {
	builder := theine.NewBuilder[Foo, int](10000)
	builder.StringKey(func(k Foo) string { return k.Bar })
	client, err := builder.Build()
	require.Nil(t, err)
	defer client.Close()
	for i := 0; i < 50; i++ {
		foo := Foo{Bar: strconv.Itoa(i + 100)}
		client.Set(foo, i, 1)
		if v, ok := client.Get(Foo{Bar: strconv.Itoa(i + 100)}); !ok || v != i {
			require.FailNow(t, "")
		}
	}
}
