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

type foo struct{}

func BenchmarkGetTheineParallel(b *testing.B) {
	client, err := theine.NewBuilder[string, foo](100000).Build()
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
	client.Close()

}

func BenchmarkGetRistrettoParallel(b *testing.B) {
	client, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: 1000000,
		MaxCost:     100000,
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
	client.Close()

}

func BenchmarkSetTheineParallel(b *testing.B) {
	client, err := theine.NewBuilder[string, bar](100000).Build()
	if err != nil {
		panic(err)
	}
	keys := []string{}
	for i := 0; i < 1000000; i++ {
		keys = append(keys, fmt.Sprintf("%d", i))
	}
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		counter := 0
		for pb.Next() {
			client.Set(keys[counter%1000000], bar{key: keys[counter%1000000]}, 1)
			counter++
		}
	})
	client.Close()

}

func BenchmarkSetRistrettoParallel(b *testing.B) {
	client, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: 1000000,
		MaxCost:     100000,
		BufferItems: 64,
	})
	if err != nil {
		panic(err)
	}
	keys := []string{}
	for i := 0; i < 1000000; i++ {
		keys = append(keys, fmt.Sprintf("%d", i))
	}
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		counter := 0
		for pb.Next() {
			client.Set(keys[counter%1000000], bar{key: keys[counter%1000000]}, 1)
			counter++
		}
	})
	client.Close()

}

func BenchmarkZipfTheineParallel(b *testing.B) {
	client, err := theine.NewBuilder[string, bar](100000).Build()
	if err != nil {
		panic(err)
	}
	z := rand.NewZipf(rand.New(rand.NewSource(time.Now().UnixNano())), 1.0001, 10, 1000000)
	keys := []string{}
	for i := 0; i < 1000000; i++ {
		keys = append(keys, fmt.Sprintf("%d", z.Uint64()))
	}
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		counter := 0
		for pb.Next() {
			_, ok := client.Get(keys[counter%1000000])
			if !ok {
				client.Set(keys[counter%1000000], bar{key: keys[counter%1000000]}, 1)
			}
			counter++
		}
	})
	client.Close()

}

func BenchmarkZipfRistrettoParallel(b *testing.B) {
	client, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: 1000000,
		MaxCost:     100000,
		BufferItems: 64,
	})
	if err != nil {
		panic(err)
	}
	z := rand.NewZipf(rand.New(rand.NewSource(time.Now().UnixNano())), 1.0001, 10, 1000000)
	keys := []string{}
	for i := 0; i < 1000000; i++ {
		keys = append(keys, fmt.Sprintf("%d", z.Uint64()))
	}
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		counter := 0
		for pb.Next() {
			_, ok := client.Get(keys[counter%1000000])
			if !ok {
				client.Set(keys[counter%1000000], bar{key: keys[counter%1000000]}, 1)
			}
			counter++
		}
	})
	client.Close()

}
