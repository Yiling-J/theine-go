// Copyright (c) 2023 Alexey Mayshev. All rights reserved.
// Copyright 2009 The Go Authors. All rights reserved.
//
// Copyright notice. Initial version of the following tests was based on
// the following file from the Go Programming Language core repo:
// https://github.com/golang/go/blob/831f9376d8d730b16fb33dfd775618dffe13ce7a/src/sync/rwmutex_test.go
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
// That can be found at https://github.com/golang/go/blob/831f9376d8d730b16fb33dfd775618dffe13ce7a/LICENSE

//go:build !race

package node

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type Node[K comparable, V any] struct {
	key   K
	value V
	lock  atomic.Uint32
}

var upool = sync.Pool{
	New: func() any {
		return &atomic.Uint32{}
	},
}

func (n *Node[K, V]) Value() V {
	u := upool.Get().(*atomic.Uint32)
	for {

		seq := n.lock.Load()
		if seq&1 != 0 {
			runtime.Gosched()
			continue
		}

		u.Load()
		value := n.value

		if seq == n.lock.Load() {
			upool.Put(u)
			return value
		}
	}
}

// Lock locks the node for updates.
func (n *Node[K, V]) Lock() {
	for {
		seq := n.lock.Load()
		if seq&1 != 0 {
			runtime.Gosched()
			continue
		}

		if n.lock.CompareAndSwap(seq, seq+1) {
			return
		}
	}
}

// Unlock unlocks the node.
func (n *Node[K, V]) Unlock() {
	n.lock.Add(1)
}

// SetValue sets the value.
func (n *Node[K, V]) SetValue(value V) {
	n.value = value
}

func New[K comparable, V any](key K, value V) *Node[K, V] {
	return &Node[K, V]{
		key:   key,
		value: value,
	}
}

func reader(node *Node[int, int], num_iterations int, s *atomic.Uint32) {
	for s.Load() == 0 {
		time.Sleep(10 * time.Millisecond)
		continue
	}
	for i := 0; i < num_iterations; i++ {
		vn := node.Value()
		if vn != 0 {
			panic(fmt.Sprintf("wlock(%d)\n", vn))
		}
	}
	s.Add(1)
}

func writer(node *Node[int, int], num_iterations int, s *atomic.Uint32, total uint32) {
	writes := 0
	s.Add(1)

	for s.Load() != total {
		node.Lock()
		node.SetValue(1)
		node.SetValue(0)
		node.Unlock()
		writes++
	}
	fmt.Println("total writes: ", writes)
}

func HammerRWMutex(numReaders, num_iterations int) {
	node := &Node[int, int]{}
	c := &atomic.Uint32{}
	var i int
	for i = 0; i < numReaders; i++ {
		go reader(node, num_iterations, c)
	}
	writer(node, num_iterations, c, uint32(numReaders))
}

func TestNode_Seqlock(t *testing.T) {
	for _, p := range []int{4, 8, 16, 32, 64} {
		t.Run(fmt.Sprintf("parallel %d", p), func(t *testing.T) {
			HammerRWMutex(p, 10000000)
		})
	}
}
