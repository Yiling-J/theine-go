// Package mpsc provides an efficient implementation of a multi-producer, single-consumer lock-free queue.
//
// The Push function is safe to call from multiple goroutines. The Pop and Empty APIs must only be
// called from a single, consumer goroutine.
package internal

// This implementation is based on http://www.1024cores.net/home/lock-free-algorithms/queues/non-intrusive-mpsc-node-based-queue

import (
	"sync"
	"sync/atomic"
	"unsafe"
)

type node[V any] struct {
	next *node[V]
	val  V
}

type Queue[V any] struct {
	head, tail *node[V]
	nodePool   sync.Pool
}

func NewQueue[V any]() *Queue[V] {
	q := &Queue[V]{nodePool: sync.Pool{New: func() any {
		return new(node[V])
	}}}
	stub := &node[V]{}
	q.head = stub
	q.tail = stub
	return q
}

// Push adds x to the back of the queue.
//
// Push can be safely called from multiple goroutines
func (q *Queue[V]) Push(x V) {
	n := q.nodePool.Get().(*node[V])
	n.val = x
	// current producer acquires head node
	prev := (*node[V])(atomic.SwapPointer((*unsafe.Pointer)(unsafe.Pointer(&q.head)), unsafe.Pointer(n)))

	// release node to consumer
	atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&prev.next)), unsafe.Pointer(n))
}

// Pop removes the item from the front of the queue or nil if the queue is empty
//
// Pop must be called from a single, consumer goroutine
func (q *Queue[V]) Pop() interface{} {
	tail := q.tail
	next := (*node[V])(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&tail.next)))) // acquire
	if next != nil {
		var null V
		q.tail = next
		v := next.val
		next.val = null
		tail.next = nil
		q.nodePool.Put(tail)
		return v
	}
	return nil
}

// Empty returns true if the queue is empty
//
// Empty must be called from a single, consumer goroutine
func (q *Queue[V]) Empty() bool {
	tail := q.tail
	next := (*node[V])(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&tail.next))))
	return next == nil
}
