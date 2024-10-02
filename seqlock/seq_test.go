package node

import (
	"fmt"
	"runtime"
	"sync/atomic"
	"testing"
	"time"
)

type Value[V any] struct {
	v V
}

type Node[K comparable, V any] struct {
	key   K
	value atomic.Pointer[Value[V]]
	lock  atomic.Uint32
}

func (n *Node[K, V]) Value() V {
	for {

		seq := n.lock.Load()
		if seq&1 != 0 {
			runtime.Gosched()
			continue
		}

		value := n.value.Load()

		if seq == n.lock.Load() {
			return value.v
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
	n.value.Store(&Value[V]{value})
}

func New[K comparable, V any](key K, value V) *Node[K, V] {
	n := &Node[K, V]{
		key: key,
	}
	n.value.Store(&Value[V]{value})
	return n
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
			HammerRWMutex(p, 5000000)
		})
	}
}
