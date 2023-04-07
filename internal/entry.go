package internal

import "sync/atomic"

const (
	NEW int8 = iota
	REMOVE
	UPDATE
)

type ReadBufItem[K comparable, V any] struct {
	entry *Entry[K, V]
	hash  uint64
}
type WriteBufItem[K comparable, V any] struct {
	entry      *Entry[K, V]
	code       int8
	costChange int64
	rechedule  bool
}

type MetaData[K comparable, V any] struct {
	prev       *Entry[K, V]
	next       *Entry[K, V]
	wheelPrev  *Entry[K, V]
	wheelNext  *Entry[K, V]
	_list      *List[K, V]
	_wheelList *List[K, V]
}

type Entry[K comparable, V any] struct {
	removed bool
	shard   uint16
	cost    atomic.Int64
	key     K
	value   V
	expire  atomic.Int64
	meta    MetaData[K, V]
}

func NewEntry[K comparable, V any](key K, value V, cost int64, expire int64) *Entry[K, V] {
	entry := &Entry[K, V]{
		key:   key,
		value: value,
	}
	entry.cost.Store(cost)
	if expire > 0 {
		entry.expire.Store(expire)
	}
	return entry
}

func (e *Entry[K, V]) Next(listType uint8) *Entry[K, V] {
	if listType == WHEEL_LIST {
		if p := e.meta.wheelNext; e.meta._wheelList != nil && p != &e.meta._wheelList.root {
			return p
		}
		return nil
	}
	if p := e.meta.next; e.meta._list != nil && p != &e.meta._list.root {
		return p
	}
	return nil
}

func (e *Entry[K, V]) Prev(listType uint8) *Entry[K, V] {
	if listType == WHEEL_LIST {
		if p := e.meta.wheelPrev; e.meta._wheelList != nil && p != &e.meta._wheelList.root {
			return p
		}
		return nil
	}
	if p := e.meta.prev; e.meta._list != nil && p != &e.meta._list.root {
		return p
	}
	return nil
}

func (e *Entry[K, V]) list(listType uint8) *List[K, V] {
	if listType == WHEEL_LIST {
		return e.meta._wheelList
	}
	return e.meta._list
}

func (e *Entry[K, V]) prev(listType uint8) *Entry[K, V] {
	if listType == WHEEL_LIST {
		return e.meta.wheelPrev
	}
	return e.meta.prev
}

func (e *Entry[K, V]) next(listType uint8) *Entry[K, V] {
	if listType == WHEEL_LIST {
		return e.meta.wheelNext
	}
	return e.meta.next
}

func (e *Entry[K, V]) setPrev(entry *Entry[K, V], listType uint8) {
	if listType == WHEEL_LIST {
		e.meta.wheelPrev = entry
	} else {
		e.meta.prev = entry
	}
}

func (e *Entry[K, V]) setNext(entry *Entry[K, V], listType uint8) {
	if listType == WHEEL_LIST {
		e.meta.wheelNext = entry
	} else {
		e.meta.next = entry
	}
}
