package theine_test

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/Yiling-J/theine-go"
	"github.com/dgraph-io/ristretto"
)

type bar struct {
	key string
}

func BenchmarkGetTheineParallel(b *testing.B) {
	client, err := theine.New[string, foo](&theine.Config{MaximumSize: 10000})
	if err != nil {
		panic(err)
	}
	keys := []string{}
	for i := 0; i < 100000; i++ {
		keys = append(keys, fmt.Sprintf("%d", i))
	}
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		counter := 0
		for pb.Next() {
			client.Get(keys[counter%100000])
			counter++
		}
	})
}

func BenchmarkGetRistrettoParallel(b *testing.B) {
	client, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: 100000,
		MaxCost:     10000,
		BufferItems: 64,
	})
	if err != nil {
		panic(err)
	}
	keys := []string{}
	for i := 0; i < 100000; i++ {
		keys = append(keys, fmt.Sprintf("%d", i))
	}
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		counter := 0
		for pb.Next() {
			client.Get(keys[counter%100000])
			counter++
		}
	})
}

func BenchmarkSetTheineParallel(b *testing.B) {
	client, err := theine.New[string, bar](&theine.Config{MaximumSize: 10000})
	if err != nil {
		panic(err)
	}
	keys := []string{}
	for i := 0; i < 100000; i++ {
		keys = append(keys, fmt.Sprintf("%d", i))
	}
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		counter := 0
		for pb.Next() {
			client.Set(keys[counter%100000], bar{key: keys[counter%100000]})
			counter++
		}
	})
}

func BenchmarkSetRistrettoParallel(b *testing.B) {
	client, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: 100000,
		MaxCost:     10000,
		BufferItems: 64,
	})
	if err != nil {
		panic(err)
	}
	keys := []string{}
	for i := 0; i < 100000; i++ {
		keys = append(keys, fmt.Sprintf("%d", i))
	}
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		counter := 0
		for pb.Next() {
			client.Set(keys[counter%100000], bar{key: keys[counter%100000]}, 1)
			counter++
		}
	})
}

func BenchmarkZipfTheineParallel(b *testing.B) {
	client, err := theine.New[string, bar](&theine.Config{MaximumSize: 10000})
	if err != nil {
		panic(err)
	}
	z := rand.NewZipf(rand.New(rand.NewSource(time.Now().UnixNano())), 1.0001, 10, 1000000)
	keys := []string{}
	for i := 0; i < 100000; i++ {
		keys = append(keys, fmt.Sprintf("%d", z.Uint64()))
	}
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		counter := 0
		for pb.Next() {
			_, ok := client.Get(keys[counter%100000])
			if !ok {
				client.Set(keys[counter%100000], bar{key: keys[counter%100000]})
			}
			counter++
		}
	})
}

func BenchmarkZipfRistrettoParallel(b *testing.B) {
	client, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: 100000,
		MaxCost:     10000,
		BufferItems: 64,
	})
	if err != nil {
		panic(err)
	}
	z := rand.NewZipf(rand.New(rand.NewSource(time.Now().UnixNano())), 1.0001, 10, 1000000)
	keys := []string{}
	for i := 0; i < 100000; i++ {
		keys = append(keys, fmt.Sprintf("%d", z.Uint64()))
	}
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		counter := 0
		for pb.Next() {
			_, ok := client.Get(keys[counter%100000])
			if !ok {
				client.Set(keys[counter%100000], bar{key: keys[counter%100000]}, 1)
			}
			counter++
		}
	})
}
