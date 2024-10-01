package node_test

import (
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
)

type t struct {
	lock  atomic.Uint32
	value any
}

func o1(n *t) any {
	for {
		seq := n.lock.Load()
		if seq&1 != 0 {
			runtime.Gosched()
			continue
		}

		value := n.value

		newSeq := n.lock.Load()
		if seq == newSeq {
			return value
		}
	}
}

var upool = sync.Pool{
	New: func() any {
		return &atomic.Uint32{}
	},
}

func o2(n *t) any {
	u := upool.Get().(*atomic.Uint32)
	for {
		seq := n.lock.Load()
		if seq&1 != 0 {
			runtime.Gosched()
			continue
		}
		value := n.value
		u.CompareAndSwap(0, 0)

		if n.lock.Load() == seq {
			upool.Put(u)
			return value
		}
	}
}

func BenchmarkNode_O1(b *testing.B) {
	t := &t{
		value: 666,
	}
	for n := 0; n < b.N; n++ {
		o1(t)
	}
}

func BenchmarkNode_O1P(b *testing.B) {
	t := &t{
		value: 666,
	}
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			o1(t)
		}
	})
}

func BenchmarkNode_O2(b *testing.B) {
	t := &t{
		value: 666,
	}
	for n := 0; n < b.N; n++ {
		o2(t)
	}
}

func BenchmarkNode_O2P(b *testing.B) {
	t := &t{
		value: 666,
	}
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			o2(t)
		}
	})
}
