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
	client, err := theine.New[string, string](1000)
	require.Nil(t, err)
	for i := 0; i < 20000; i++ {
		key := fmt.Sprintf("key:%d", rand.Intn(100000))
		client.Set(key, key)
	}
	time.Sleep(300 * time.Millisecond)
	require.True(t, client.Len() < 1200)
}

func TestSetParallel(t *testing.T) {
	client, err := theine.New[string, string](1000)
	require.Nil(t, err)
	var wg sync.WaitGroup
	for i := 1; i <= 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 10000; i++ {
				key := fmt.Sprintf("key:%d", rand.Intn(100000))
				client.Set(key, key)
			}

		}()
	}
	wg.Wait()
	time.Sleep(300 * time.Millisecond)
	require.True(t, client.Len() < 1200)
}

func TestGetSet(t *testing.T) {
	client, err := theine.New[string, string](1000)
	require.Nil(t, err)
	for i := 0; i < 20000; i++ {
		key := fmt.Sprintf("key:%d", rand.Intn(3000))
		v, ok := client.Get(key)
		if !ok {
			client.Set(key, key)
		} else {
			require.Equal(t, key, v)
		}
	}
	time.Sleep(300 * time.Millisecond)
	require.True(t, client.Len() < 1200)
}

func TestGetSetParallel(t *testing.T) {
	client, err := theine.New[string, string](1000)
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
					client.Set(key, key)
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
	client, err := theine.New[string, string](500)
	require.Nil(t, err)
	client.SetWithTTL("foo", "foo", 3600*time.Second)
	require.Equal(t, 1, client.Len())
	time.Sleep(1 * time.Second)
	client.SetWithTTL("foo", "foo", 1*time.Second)
	require.Equal(t, 1, client.Len())
	time.Sleep(2 * time.Second)
	_, ok := client.Get("foo")
	require.False(t, ok)
	require.Equal(t, 0, client.Len())
}

func TestSetWithTTLAutoExpire(t *testing.T) {
	client, err := theine.New[string, string](500)
	require.Nil(t, err)
	for i := 0; i < 30; i++ {
		key1 := fmt.Sprintf("key:%d", i)
		client.SetWithTTL(key1, key1, time.Duration(i+1)*time.Second)
		key2 := fmt.Sprintf("key:%d:2", i)
		client.SetWithTTL(key2, key2, time.Duration(i+100)*time.Second)
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
