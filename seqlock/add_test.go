package node

import (
	"sync/atomic"
	"testing"
)

func Benchmark_AtomicAdd(b *testing.B) {
	var c atomic.Uint64
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			c.Add(1)
		}
	})
}
