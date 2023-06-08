package theine_test

import (
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Yiling-J/theine-go"
	"github.com/stretchr/testify/require"
)

func TestMaxsizeZero(t *testing.T) {
	_, err := theine.NewBuilder[string, string](0).Build()
	require.NotNil(t, err)
}

func TestSet(t *testing.T) {
	client, err := theine.NewBuilder[string, string](1000).Build()
	require.Nil(t, err)
	for i := 0; i < 20000; i++ {
		key := fmt.Sprintf("key:%d", rand.Intn(100000))
		client.Set(key, key, 1)
	}
	time.Sleep(300 * time.Millisecond)
	require.True(t, client.Len() < 1200)
	client.Close()
}

func TestUpdate(t *testing.T) {
	client, err := theine.NewBuilder[string, string](1000).Build()
	require.Nil(t, err)
	key := "foo"
	for _, v := range []string{"a", "b", "c", "d", "e", "e"} {
		client.Set(key, v, 1)
		vn, ok := client.Get(key)
		require.True(t, ok)
		require.Equal(t, v, vn)
	}
}

func TestSetParallel(t *testing.T) {
	client, err := theine.NewBuilder[string, string](1000).Build()
	require.Nil(t, err)
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
	time.Sleep(300 * time.Millisecond)
	require.True(t, client.Len() < 1200)
	client.Close()
}

func TestGetSetGetDeleteGet(t *testing.T) {
	client, err := theine.NewBuilder[string, string](1000).Build()
	require.Nil(t, err)
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
	client.Close()
}

func TestDelete(t *testing.T) {
	client, err := theine.NewBuilder[string, string](100).Build()
	require.Nil(t, err)
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
	client.Close()
}

func TestGetSetParallel(t *testing.T) {
	client, err := theine.NewBuilder[string, string](1000).Build()
	require.Nil(t, err)
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
	time.Sleep(300 * time.Millisecond)
	require.True(t, client.Len() < 1200)
	client.Close()
}

func TestSetWithTTL(t *testing.T) {
	client, err := theine.NewBuilder[string, string](500).Build()
	require.Nil(t, err)
	client.SetWithTTL("foo", "foo", 1, 3600*time.Second)
	require.Equal(t, 1, client.Len())
	time.Sleep(1 * time.Second)
	client.SetWithTTL("foo", "foo", 1, 1*time.Second)
	require.Equal(t, 1, client.Len())
	time.Sleep(2 * time.Second)
	_, ok := client.Get("foo")
	require.False(t, ok)
	require.Equal(t, 0, client.Len())
	client.Close()
}

func TestSetWithTTLAutoExpire(t *testing.T) {
	client, err := theine.NewBuilder[string, string](500).Build()
	require.Nil(t, err)
	for i := 0; i < 30; i++ {
		key1 := fmt.Sprintf("key:%d", i)
		client.SetWithTTL(key1, key1, 1, time.Duration(i+1)*time.Second)
		key2 := fmt.Sprintf("key:%d:2", i)
		client.SetWithTTL(key2, key2, 1, time.Duration(i+100)*time.Second)
	}
	current := 60
	counter := 0
	for {
		time.Sleep(5 * time.Second)
		counter += 1
		require.True(t, client.Len() < current)
		current = client.Len()
		if current <= 30 {
			break
		}
	}
	require.True(t, counter < 10)
	for i := 0; i < 30; i++ {
		key := fmt.Sprintf("key:%d", i)
		_, ok := client.Get(key)
		require.False(t, ok)
	}
	client.Close()
}

func TestGetSetDeleteNoRace(t *testing.T) {
	for _, size := range []int{500, 2000, 10000, 50000} {
		builder := theine.NewBuilder[string, string](int64(size))
		builder.RemovalListener(func(key, value string, reason theine.RemoveReason) {})
		client, err := builder.Build()
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
					v, ok := client.Get(key)
					if ok && v != key {
						panic(key)
					}
					if i%3 == 0 {
						client.SetWithTTL(key, key, int64(i%10+1), time.Second*time.Duration(i%25+5))
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

func TestCost(t *testing.T) {
	client, err := theine.NewBuilder[string, string](500).Build()
	require.Nil(t, err)
	success := client.Set("z", "z", 501)
	require.False(t, success)
	for i := 0; i < 30; i++ {
		key := fmt.Sprintf("key:%d", i)
		success = client.Set(key, key, 20)
		require.True(t, success)
	}
	time.Sleep(time.Second)
	require.True(t, client.Len() == 25)

	// test cost func
	builder := theine.NewBuilder[string, string](500)
	builder.Cost(func(v string) int64 {
		return int64(len(v))
	})
	client, err = builder.Build()
	require.Nil(t, err)
	success = client.Set("z", strings.Repeat("z", 501), 0)
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

func TestCostUpdate(t *testing.T) {
	client, err := theine.NewBuilder[string, string](500).Build()
	require.Nil(t, err)
	for i := 0; i < 30; i++ {
		key := fmt.Sprintf("key:%d", i)
		success := client.Set(key, key, 20)
		require.True(t, success)
	}
	time.Sleep(time.Second)
	require.True(t, client.Len() == 25)
	// update cost
	success := client.Set("key:10", "", 200)
	require.True(t, success)
	time.Sleep(time.Second)
	// 15 * 20 + 200
	require.True(t, client.Len() == 16)
	client.Close()
}

func TestDoorkeeper(t *testing.T) {
	builder := theine.NewBuilder[string, string](500)
	builder.Doorkeeper(true)
	client, err := builder.Build()
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

func TestZeroDequeFrequency(t *testing.T) {
	client, err := theine.NewBuilder[int, int](100).Build()
	require.Nil(t, err)
	// set and access 200 entries
	for i := 0; i < 200; i++ {
		success := client.Set(i, i, 1)
		require.True(t, success)
	}
	for i := 0; i < 200; i++ {
		client.Get(i)
	}
	// set a new entry
	success := client.Set(999, 999, 1)
	require.True(t, success)
	// wait write queue process
	time.Sleep(time.Second)
	// 999 is evicted automatically, because tail entry in slru has frequency 1
	// but 999 frequency is 0
	// so increase 999 frequency
	for i := 0; i < 128; i++ {
		_, ok := client.Get(999)
		require.False(t, ok)
	}
	time.Sleep(time.Second)
	// set again, this time the tail entry will be evicted because 999 has higher frequency
	success = client.Set(999, 999, 1)
	require.True(t, success)
	// wait write queue process
	time.Sleep(time.Second)
	_, ok := client.Get(999)
	require.True(t, ok)

}

func TestRemovalListener(t *testing.T) {
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

func TestRange(t *testing.T) {
	for _, cap := range []int{100, 200000} {
		client, err := theine.NewBuilder[int, int](int64(cap)).Build()
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
