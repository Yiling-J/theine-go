//go:build !race && !amd64

package internal

import (
	"runtime"
	"sync/atomic"
)


type Entry[K comparable, V any] struct {
	key       K
	value     V
	meta      MetaData[K, V]
	cost      int64
	expire    atomic.Int64
	frequency atomic.Int32
	seqlock   atomic.Uint32
	queued    uint8
	flag      Flag
}

var foo atomic.Uint32

func (e *Entry[K, V]) Read(key K) (V, bool) {
	for {

		seq := e.seqlock.Load()
		if seq&1 != 0 {
			runtime.Gosched()
			continue
		}
		value := e.value
		foo.CompareAndSwap(1,1)
		if seq == e.seqlock.Load() {
			return value, true
		}
	}

}
func (e *Entry[K, V]) ReadKV() (K, V) {
	for {
		seq := e.seqlock.Load()
		if seq&1 != 0 {
			runtime.Gosched()
			continue
		}
		k := e.key
		v := e.value

		if seq == e.seqlock.Load() {
			return k, v
		}
	}
}

func (e *Entry[K, V]) Lock() {
	for {
		seq := e.seqlock.Load()
		if seq&1 != 0 {
			runtime.Gosched()
			continue
		}

		if e.seqlock.CompareAndSwap(seq, seq+1) {
			return
		}
	}
}

func (e *Entry[K, V]) Unlock() {
	e.seqlock.Add(1)
}

func (e *Entry[K, V]) UpdateKV(key K, value V) {
	e.Lock()
	e.key = key
	e.value = value
	e.Unlock()
}

func (e *Entry[K, V]) UpdateValue(value V) {
	e.Lock()
	e.value = value
	e.Unlock()
}
