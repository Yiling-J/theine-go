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

func (s *StripedQueue[K, V]) UpdateCost(hash uint64, entry *Entry[K, V], cost int64) bool {
	q := s.qs[hash&uint64(s.count-1)]
	q.mu.Lock()
	defer q.mu.Unlock()
	if entry.queued == 0 {
		entry.cost = cost
		return true
	}
	if entry.queued == 1 {
		costChange := cost - entry.cost
		entry.cost = cost
		q.len += int(costChange)
		return true

	}
	return false
}

type Queue[K comparable, V any] struct {
	deque *deque.Deque[QueueItem[K, V]]
	len   int
	size  int
	mu    sync.Mutex
}

func (q *Queue[K, V]) push(hash uint64, entry *Entry[K, V], cost int64, fromNVM bool, threshold int32, sendCallback func(item QueueItem[K, V]), removeCallback func(item QueueItem[K, V])) {
	q.mu.Lock()
	// new entry cost should be -1,
	// not -1 means already updated and cost param is stale
	if entry.cost == -1 {
		entry.cost = cost
	}
	entry.queued = 1

	q.len += int(entry.cost)
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
		evicted.entry.queued = 2
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
	for _, item := range send {
		sendCallback(item)
	}
	for _, item := range removed {
		removeCallback(item)
	}
}
