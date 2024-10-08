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
			index: int32(i),
		})
	}
	return sq
}

func (s *StripedQueue[K, V]) Push(hash uint64, entry *Entry[K, V], cost int64, fromNVM bool) {
	q := s.qs[hash&uint64(s.count-1)]
	q.push(hash, entry, cost, fromNVM, s.thresholdLoad(), s.sendCallback, s.removeCallback)
}

func (s *StripedQueue[K, V]) PushSimple(hash uint64, entry *Entry[K, V]) {
	q := s.qs[hash&uint64(s.count-1)]
	q.len += int(entry.policyWeight)
	q.deque.PushFront(QueueItem[K, V]{entry: entry, fromNVM: entry.flag.IsFromNVM()})
}

func (s *StripedQueue[K, V]) UpdateCost(key K, hash uint64, entry *Entry[K, V], costChange int64) bool {
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
		entry.policyWeight += costChange
		q.len += int(costChange)
		return true
	case -1:
		// entry is moving from queue to main
		// also send update event to main
		return false
	case -2:
		// there are 2 kinds of race here:
		// - create/update race for same entry, because create also
		// use += on policy weight, the result is consistent.
		// - evict/update race for differnet entry, because evicted
		// entry will be resued from sync pool, in this case policy weight should not update.
		if entry.key == key {
			entry.policyWeight += costChange
		}
		return true
	case -3:
		// entry is removed from queue
		// still return true here because entry is not on main yet.
		return true
	default:
		return false
	}

}

func (s *StripedQueue[K, V]) Delete(hash uint64, entry *Entry[K, V]) bool {
	q := s.qs[hash&uint64(s.count-1)]
	q.mu.Lock()
	defer q.mu.Unlock()

	index := entry.queueIndex.Load()

	switch index {
	case q.index:
		entry.queueIndex.Store(-3)
		return true
	case -2:
		entry.queueIndex.Store(-3)
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

func (q *Queue[K, V]) push(hash uint64, entry *Entry[K, V], costChange int64, fromNVM bool, threshold int32, sendCallback func(item QueueItem[K, V]), removeCallback func(item QueueItem[K, V])) {
	q.mu.Lock()

	if i := entry.queueIndex.Load(); i == -3 {
		q.mu.Unlock()
		return
	}

	success := entry.queueIndex.CompareAndSwap(-2, q.index)
	if !success {
		panic(fmt.Sprintf("add to queue failed %d", entry.queueIndex.Load()))
	}
	// += here because of possible create/update race
	entry.policyWeight += costChange

	q.len += int(entry.policyWeight)
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
		q.len -= int(evicted.entry.policyWeight)

		if evicted.entry.queueIndex.Load() == -3 {
			continue
		}

		count := evicted.entry.frequency.Load()
		var index int32 = -1
		if count == -1 {
			send = append(send, evicted)
		} else {
			if int32(count) >= threshold {
				send = append(send, evicted)
			} else {
				index = -3
				removed = append(removed, evicted)
			}
		}

		success := evicted.entry.queueIndex.CompareAndSwap(q.index, index)
		if !success {
			panic(fmt.Sprintf("evict queue failed %d %d",
				evicted.entry.queueIndex.Load(), q.index))
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
