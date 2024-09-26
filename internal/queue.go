package internal

import (
	"sync"

	"github.com/gammazero/deque"
)

type StripedQueue[K comparable, V any] struct {
	qs            []*Queue[K, V]
	count         int
	thresholdLoad func() int32
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

func (s *StripedQueue[K, V]) Push(hash uint64, entry *Entry[K, V], cost int64, fromNVM bool) ([]QueueItem[K, V], []QueueItem[K, V]) {
	q := s.qs[hash&uint64(s.count-1)]
	return q.push(hash, entry, cost, fromNVM, s.thresholdLoad())
}

func (s *StripedQueue[K, V]) UpdateCost(hash uint64, entry *Entry[K, V], cost int64) bool {
	q := s.qs[hash&uint64(s.count-1)]
	q.mu.Lock()
	queued := entry.deque
	if entry.deque && entry.cost != cost {
		costChange := cost - entry.cost
		entry.cost = cost
		q.len += int(costChange)
	}
	q.mu.Unlock()
	return queued
}

type Queue[K comparable, V any] struct {
	deque *deque.Deque[QueueItem[K, V]]
	len   int
	size  int
	mu    sync.Mutex
}

func (q *Queue[K, V]) push(hash uint64, entry *Entry[K, V], cost int64, fromNVM bool, threshold int32) ([]QueueItem[K, V], []QueueItem[K, V]) {
	q.mu.Lock()
	entry.deque = true
	entry.cost = cost
	q.len += int(cost)
	q.deque.PushFront(QueueItem[K, V]{entry: entry, fromNVM: fromNVM})
	if q.len <= q.size {
		q.mu.Unlock()
		return nil, nil
	}

	// send to slru
	send := make([]QueueItem[K, V], 0, 2)
	// removed because frequency < slru tail frequency
	removed := make([]QueueItem[K, V], 0, 2)

	for q.len > q.size {
		evicted := q.deque.PopBack()
		evicted.entry.deque = false
		q.len -= int(evicted.entry.cost)

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
	return send, removed
}
