package internal

import (
	"sync"

	"github.com/gammazero/deque"
)

type StripedQueue[K comparable, V any] struct {
	qs             []*Queue[K, V]
	count          int
	thresholdLoad  func() int32
	sendCallback   func(item QueueItem[K, V])
	removeCallback func(item QueueItem[K, V])
	sketch         *CountMinSketch
}

func NewStripedQueue[K comparable, V any](queueCount int, queueSize int, sketch *CountMinSketch, thresholdLoad func() int32) *StripedQueue[K, V] {
	sq := &StripedQueue[K, V]{
		qs:            make([]*Queue[K, V], 0),
		count:         queueCount,
		thresholdLoad: thresholdLoad,
		sketch:        sketch,
	}
	for i := 0; i < queueCount; i++ {
		sq.qs = append(sq.qs, &Queue[K, V]{
			deque:  deque.New[QueueItem[K, V]](8),
			size:   queueSize,
			index:  int32(i),
			sketch: sketch,
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
	q.deque.PushFront(QueueItem[K, V]{entry: entry, fromNVM: entry.flag.IsFromNVM(), hash: hash})
}

func (s *StripedQueue[K, V]) UpdateCost(key K, hash uint64, entry *Entry[K, V], costChange int64) bool {
	q := s.qs[hash&uint64(s.count-1)]

	q.mu.Lock()
	defer q.mu.Unlock()

	index := entry.queueIndex.Load()

	switch index {

	case q.index:
		entry.policyWeight += costChange
		q.len += int(costChange)
		return true
	case -1:
		// entry is moving from queue to main
		// also send update event to main
		return false
	case -2:
		// there are two types of race conditions here:
		// - Create/update race for the same entry: Since both create and update use `+=` on the policy weight, the result is consistent.
		// - Evict/update race for different entries: When an entry is evicted and reused from the sync pool, the policy weight should not be updated in this case.
		// The race detector may flag this line, but it's safe to ignore.
		if entry.key == key {
			entry.policyWeight += costChange
		}
		return true
	case -3:
		// entry is removed from queue
		// still return true here because entry is not on main yet.
		// So don't send event to main.
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
	index  int32
	deque  *deque.Deque[QueueItem[K, V]]
	len    int
	size   int
	mu     sync.Mutex
	sketch *CountMinSketch
}

func (q *Queue[K, V]) push(hash uint64, entry *Entry[K, V], costChange int64, fromNVM bool, threshold int32, sendCallback func(item QueueItem[K, V]), removeCallback func(item QueueItem[K, V])) {
	q.mu.Lock()

	if i := entry.queueIndex.Load(); i == -3 {
		q.mu.Unlock()
		return
	}

	success := entry.queueIndex.CompareAndSwap(-2, q.index)
	if !success {
		return
	}
	// += here because of possible create/update race
	entry.policyWeight += costChange

	q.len += int(entry.policyWeight)
	q.deque.PushFront(QueueItem[K, V]{entry: entry, fromNVM: fromNVM, hash: hash})
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

		count := q.sketch.Estimate(evicted.hash)
		var index int32 = -1
		if int32(count) >= threshold {
			send = append(send, evicted)
		} else {
			index = -3
			removed = append(removed, evicted)
		}

		evicted.entry.queueIndex.CompareAndSwap(q.index, index)
	}
	q.mu.Unlock()
	for _, item := range send {
		sendCallback(item)
	}
	for _, item := range removed {
		removeCallback(item)
	}
}
