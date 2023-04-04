package internal

import (
	"sync"
	"sync/atomic"
)

type LockedBuf[K comparable, V any] struct {
	cap     int
	buf     []BufItem[K, V]
	extra   []BufItem[K, V]
	lock    sync.RWMutex
	counter atomic.Uint32
}

func NewLockedBuf[K comparable, V any](cap int) *LockedBuf[K, V] {
	return &LockedBuf[K, V]{
		cap:   cap + 1,
		buf:   make([]BufItem[K, V], cap*2),
		extra: make([]BufItem[K, V], 0, cap*2),
	}
}

// Push add v to buf, return true if buf is full.
// Caller should handle the buf, reset the count and finally unlock.
func (b *LockedBuf[K, V]) Push(v BufItem[K, V]) bool {
	b.lock.RLock()
	new := b.counter.Add(1)
	switch {
	case new < uint32(b.cap):
		b.buf[new] = v
		b.lock.RUnlock()
	case new == uint32(b.cap):
		b.buf[new] = v
		b.lock.RUnlock()
		b.lock.Lock()
		return true
	case new > uint32(b.cap):
		b.lock.RUnlock()
		b.lock.Lock()
		b.extra = append(b.extra, v)
		b.lock.Unlock()
	}
	return false
}
