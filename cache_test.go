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

// func TestSetBasic(t *testing.T) {
// 	client, err := theine.New(1000)
// 	require.Nil(t, err)
// 	for i := 0; i < 20000000; i++ {
// 		key := fmt.Sprintf("key:%d", rand.Intn(100000))
// 		client.Set(key, key)
// 	}
// }

// func TestSetBasicR(t *testing.T) {
// 	client, err := ristretto.NewCache(&ristretto.Config{
// 		NumCounters: 1000,
// 		MaxCost:     5,
// 		BufferItems: 64,
// 	})
// 	require.Nil(t, err)
// 	for i := 0; i < 20000000; i++ {
// 		key := fmt.Sprintf("key:%d", rand.Intn(100000))
// 		client.Set(key, key, 1)
// 	}
// }

// func TestSetBasicParallel(t *testing.T) {
// 	client, err := theine.New(3000)
// 	require.Nil(t, err)
// 	var wg sync.WaitGroup
// 	for i := 1; i <= 36; i++ {
// 		wg.Add(1)
// 		go func() {
// 			defer wg.Done()
// 			for i := 0; i < 100000; i++ {
// 				key := fmt.Sprintf("key:%d", rand.Intn(100000))
// 				client.Set(key, key)
// 			}

// 		}()
// 	}
// 	wg.Wait()
// 	fmt.Println("total", client.Len())
// }

// func TestSetBasicParallelR(t *testing.T) {
// 	client, err := ristretto.NewCache(&ristretto.Config{
// 		NumCounters: 30000,
// 		MaxCost:     3000,
// 		BufferItems: 64,
// 	})
// 	require.Nil(t, err)
// 	var wg sync.WaitGroup
// 	for i := 1; i <= 36; i++ {
// 		wg.Add(1)
// 		go func() {
// 			defer wg.Done()
// 			for i := 0; i < 1000000; i++ {
// 				key := fmt.Sprintf("key:%d", rand.Intn(100000))
// 				client.Set(key, key, 1)
// 			}

// 		}()
// 	}
// 	wg.Wait()
// }

func NewZipfian(s, v float64, n uint64) func() (uint64, error) {
	z := rand.NewZipf(rand.New(rand.NewSource(time.Now().UnixNano())), s, v, n)
	return func() (uint64, error) {
		return z.Uint64(), nil
	}
}

func TestSetZipfParallel(t *testing.T) {
	client, err := theine.New(3000)
	require.Nil(t, err)
	var wg sync.WaitGroup
	for i := 1; i <= 36; i++ {
		wg.Add(1)
		go func() {
			zipf := NewZipfian(1.0001, 1, 1000000)
			defer wg.Done()
			for i := 0; i < 100000; i++ {
				n, err := zipf()
				require.Nil(t, err)
				key := fmt.Sprintf("key:%d", n)
				v, ok := client.Get(key)
				if ok {
					require.Equal(t, key, v.(string))
				} else {
					client.Set(key, key)
				}
			}

		}()
	}
	wg.Wait()
	fmt.Println("total", client.Len())
}

// func TestSetZipfParallelR(t *testing.T) {
// 	client, err := ristretto.NewCache(&ristretto.Config{
// 		NumCounters: 3000,
// 		MaxCost:     5,
// 		BufferItems: 64,
// 	})
// 	require.Nil(t, err)
// 	var wg sync.WaitGroup
// 	for i := 1; i <= 36; i++ {
// 		wg.Add(1)
// 		go func() {
// 			zipf := NewZipfian(1.0001, 1, 1000000)
// 			defer wg.Done()
// 			for i := 0; i < 100000; i++ {
// 				n, err := zipf()
// 				require.Nil(t, err)
// 				key := fmt.Sprintf("key:%d", n)
// 				v, ok := client.Get(key)
// 				if ok {
// 					require.Equal(t, key, v.(string))
// 				} else {
// 					client.Set(key, key, 1)
// 				}
// 			}

// 		}()
// 	}
// 	wg.Wait()
// }

// func TestSetParallel(t *testing.T) {
// 	client, err := theine.New(100)
// 	require.Nil(t, err)
// 	var wg sync.WaitGroup
// 	for i := 1; i <= 5000; i++ {
// 		wg.Add(1)

// 		go func() {
// 			key := fmt.Sprintf("key:%d", rand.Intn(100000))
// 			defer wg.Done()
// 			client.Set(key, key)
// 		}()
// 	}
// 	wg.Wait()
// }
