package theine_test

import (
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/Yiling-J/theine-go"
	"github.com/stretchr/testify/require"
)

func TestSet(t *testing.T) {
	client, err := theine.New[string, string](&theine.Config[string]{MaximumSize: 1000})
	require.Nil(t, err)
	for i := 0; i < 20000; i++ {
		key := fmt.Sprintf("key:%d", rand.Intn(100000))
		client.Set(key, key, 1)
	}
	time.Sleep(300 * time.Millisecond)
	require.True(t, client.Len() < 1200)
}

func TestSetParallel(t *testing.T) {
	client, err := theine.New[string](&theine.Config[string]{MaximumSize: 1000})
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
}

func TestGetSet(t *testing.T) {
	client, err := theine.New[string](&theine.Config[string]{MaximumSize: 1000})
	require.Nil(t, err)
	for i := 0; i < 20000; i++ {
		key := fmt.Sprintf("key:%d", rand.Intn(3000))
		v, ok := client.Get(key)
		if !ok {
			client.Set(key, key, 1)
		} else {
			require.Equal(t, key, v)
		}
	}
	time.Sleep(300 * time.Millisecond)
	require.True(t, client.Len() < 1200)
}

type foo struct {
	id int
}

func TestGeneric(t *testing.T) {
	client, err := theine.New[foo](&theine.Config[int]{MaximumSize: 100})
	require.Nil(t, err)
	key := foo{id: 1}
	client.Set(key, 1, 1)
	v, ok := client.Get(key)
	require.True(t, ok)
	require.Equal(t, 1, v)

	v, ok = client.Get(foo{id: 1})
	require.True(t, ok)
	require.Equal(t, 1, v)

	_, ok = client.Get(foo{id: 2})
	require.False(t, ok)

	clientp, err := theine.New[*foo](&theine.Config[int]{MaximumSize: 100})
	require.Nil(t, err)
	key = foo{id: 1}
	clientp.Set(&key, 1, 1)
	v, ok = clientp.Get(&key)
	require.True(t, ok)
	require.Equal(t, 1, v)

	_, ok = clientp.Get(&foo{id: 1})
	require.False(t, ok)
}

func TestDelete(t *testing.T) {
	client, err := theine.New[string](&theine.Config[string]{MaximumSize: 100})
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
}

func TestGetSetParallel(t *testing.T) {
	client, err := theine.New[string](&theine.Config[string]{MaximumSize: 1000})
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
}

func TestSetWithTTL(t *testing.T) {
	client, err := theine.New[string](&theine.Config[string]{MaximumSize: 500})
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
}

func TestSetWithTTLAutoExpire(t *testing.T) {
	client, err := theine.New[string](&theine.Config[string]{MaximumSize: 500})
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
}

func TestGetSetDeleteNoRace(t *testing.T) {
	client, err := theine.New[string](&theine.Config[string]{MaximumSize: 500})
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
				client.Get(key)
				if i%3 == 0 {
					client.SetWithTTL(key, key, 1, time.Second*time.Duration(i%25+5))
				}
				if i%5 == 0 {
					client.Delete(key)
				}
			}
		}()
	}
	wg.Wait()
	time.Sleep(300 * time.Millisecond)
	require.True(t, client.Len() < 600)
}

func TestCost(t *testing.T) {
	client, err := theine.New[string](&theine.Config[string]{MaximumSize: 500})
	require.Nil(t, err)
	success := client.Set("z", "z", 501)
	require.False(t, success)
	for i := 0; i < 30; i++ {
		key := fmt.Sprintf("key:%d", i)
		success = client.Set(key, key, 20)
		require.True(t, success)
	}
	time.Sleep(time.Second)
	// lru capacity is 5, and all entries cost are 20
	// so lru can't hold any entry and the effective size is 495
	require.True(t, client.Len() == 24)

}
