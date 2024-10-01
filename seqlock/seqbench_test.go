package node_test

import (
	"runtime"
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

var p uint32

func o2(n *t) any {
	for {
		seq := n.lock.Load()
		if seq&1 != 0 {
			runtime.Gosched()
			continue
		}
		value := n.value
		atomic.LoadUint32(&p)

		if n.lock.Load() == seq {
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

func BenchmarkNode_O2(b *testing.B) {
	t := &t{
		value: 666,
	}
	for n := 0; n < b.N; n++ {
		o2(t)
	}
}
