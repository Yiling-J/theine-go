//go:build race

package internal

import (
	"sync"
	"sync/atomic"
)

type Entry[K comparable, V any] struct {
	key       K
	value     V
	meta      MetaData[K, V]
	cost      int64
	expire    atomic.Int64
	frequency atomic.Int32
	mu        sync.Mutex
	queued    uint8
	flag      Flag
}

func (e *Entry[K, V]) Read(key K) (V, bool) {
	e.mu.Lock()
	v := e.value
	e.mu.Unlock()
	return v, true

}
func (e *Entry[K, V]) ReadKV() (K, V) {
	e.mu.Lock()
	k := e.key
	v := e.value
	e.mu.Unlock()
	return k, v
}

func (e *Entry[K, V]) UpdateKV(key K, value V) {
	e.mu.Lock()
	e.key = key
	e.value = value
	e.mu.Unlock()
}

func (e *Entry[K, V]) UpdateValue(value V) {
	e.mu.Lock()
	e.value = value
	e.mu.Unlock()
}

func (e *Entry[K,V]) Lock() {
	e.mu.Lock()
}

func (e *Entry[K,V]) Unlock() {
	e.mu.Unlock()
}
