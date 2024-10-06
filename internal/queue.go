package internal

import (
	"fmt"
	"sync"

	"github.com/gammazero/deque"
)

type StripedQueue[K comparable, V any] struct {
	qs             []*Queue[K, V]
	count          int
	thresholdLoad  func() int32
	sendCallback   func(item QueueItem[K, V])
	removeCallback func(item QueueItem[K, V])
}

func NewStripedQueue[K comparable, V any](queueCount int, queueSize int, thresholdLoad func() int32) *StripedQueue[K, V] {
	sq := &StripedQueue[K, V]{
		qs:            make([]*Queue[K, V], 0),
		count:         queueCount,
		thresholdLoad: thresholdLoad,
	}
	for i := 0; i < queueCount; i++ {
		sq.qs = append(sq.qs, &Queue[K, V]{
			deque: deque.New[QueueItem[K, V]](8),
			size:  queueSize,
		})
	}
	return sq
}

func (s *StripedQueue[K, V]) Push(hash uint64, entry *Entry[K, V], cost int64, fromNVM bool) {
	q := s.qs[hash&uint64(s.count-1)]
	q.push(hash, entry, cost, fromNVM, s.thresholdLoad(), s.sendCallback, s.removeCallback)
}

func (s *StripedQueue[K, V]) UpdateCost(hash uint64, entry *Entry[K, V], costChange int64) bool {
	q := s.qs[hash&uint64(s.count-1)]
	q.mu.Lock()
	defer q.mu.Unlock()

	// The entry is in the main when this function is called, but before loading the queue index,
	// the entry is evicted from the main cache and reused for a different key. As a result, this entry
	// object now represents a different key and has already been added to another queue.
	// The queue index still exists, but we should not update it for the current queue.
	// Instead, the new queue should handle the new entry.
	index := entry.queueIndex.Load()

	switch index {
	// If the entry's queue index matches the current queue index, the entry must be in this queue.
	// Removing the entry from the queue is protected by the queue's mutex, so there is no risk of a race condition.
	case q.index:
		q.len += int(costChange)
		return true
	case -2:
		return true
	default:
		return false
	}

}

type Queue[K comparable, V any] struct {
	index int32
	deque *deque.Deque[QueueItem[K, V]]
	len   int
	size  int
	mu    sync.Mutex
}

func (q *Queue[K, V]) push(hash uint64, entry *Entry[K, V], cost int64, fromNVM bool, threshold int32, sendCallback func(item QueueItem[K, V]), removeCallback func(item QueueItem[K, V])) {
	q.mu.Lock()
	success := entry.queueIndex.CompareAndSwap(-2, q.index)
	if !success {
		panic(fmt.Sprintf("add to queue failed %d", entry.queueIndex.Load()))
	}

	q.len += int(entry.cost.Load())
	q.deque.PushFront(QueueItem[K, V]{entry: entry, fromNVM: fromNVM})
	if q.len <= q.size {
		q.mu.Unlock()
		return
	}

	// send to slru
	send := make([]QueueItem[K, V], 0, 2)
	// removed because frequency < slru tail frequency
	removed := make([]QueueItem[K, V], 0, 2)

	for q.len > q.size {
		evicted := q.deque.PopBack()
		success := evicted.entry.queueIndex.CompareAndSwap(q.index, -1)
		if !success {
			panic("evict from queue failed")
		}
		q.len -= int(evicted.entry.cost.Load())

		count := evicted.entry.frequency.Load()
		if count == -1 {
			send = append(send, evicted)
		} else {
			if int32(count) >= threshold {
				send = append(send, evicted)
			} else {
				removed = append(removed, evicted)
			}
		}
	}
	q.mu.Unlock()
	for _, item := range send {
		sendCallback(item)
	}
	for _, item := range removed {
		removeCallback(item)
	}
}
