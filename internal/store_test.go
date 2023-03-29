package internal_test

import (
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type Foo struct {
	a int
	b int
	c int
}

func TestRace(t *testing.T) {
	foo := Foo{}
	go func() { foo.a = 12 }()
	go func() { foo.b = 12 }()
	go func() { foo.c = 12 }()

}

type shard struct {
	lock sync.RWMutex
	u    atomic.Uint32
}

func BenchmarkBytePool(b *testing.B) {
	var bytePool = sync.Pool{
		New: func() interface{} {
			b := make([]byte, 1024)
			return &b
		},
	}
	shards := make([]shard, 36)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			shard := &shards[rand.Intn(200000)%36]
			shard.lock.RLock()
			_ = 1
			shard.lock.RUnlock()
			obj := bytePool.Get().(*[]byte)
			_ = obj
			bytePool.Put(obj)
		}
	})
}

func BenchmarkByteAtomicInt(b *testing.B) {
	all := make([]int, 5000)
	shards := make([]shard, 36)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			shard := &shards[rand.Intn(200000)%36]
			shard.lock.RLock()
			_ = 1
			shard.lock.RUnlock()
			un := shard.u.Add(1)
			_ = all[un%5000]
		}
	})
}

var results []*time.Duration

func BenchmarkZeroDuration(b *testing.B) {
	for n := 0; n < b.N; n++ {
		zero := (0 * time.Second)
		results = append(results, &zero)
	}
}

func BenchmarkZeroDuration2(b *testing.B) {
	zero := (0 * time.Second)
	for n := 0; n < b.N; n++ {
		results = append(results, &zero)
	}
}
