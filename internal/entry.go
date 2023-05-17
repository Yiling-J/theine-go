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
	costChange int64
	code       int8
	rechedule  bool
}

type MetaData[K comparable, V any] struct {
	prev      *Entry[K, V]
	next      *Entry[K, V]
	wheelPrev *Entry[K, V]
	wheelNext *Entry[K, V]
	root      bool
	list      uint8 // used in slru, probation or protected
}

type Entry[K comparable, V any] struct {
	key       K
	value     V
	meta      MetaData[K, V]
	cost      atomic.Int64
	expire    atomic.Int64
	frequency atomic.Int32
	removed   bool
	deque     bool
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
	switch listType {
	case LIST_PROBATION, LIST_PROTECTED:
		if p := e.meta.next; !p.meta.root {
			return e.meta.next
		}
		return nil

	case WHEEL_LIST:
		if p := e.meta.wheelNext; !p.meta.root {
			return e.meta.wheelNext
		}
		return nil
	}
	return nil
}

func (e *Entry[K, V]) Prev(listType uint8) *Entry[K, V] {
	switch listType {
	case LIST_PROBATION, LIST_PROTECTED:
		if p := e.meta.prev; !p.meta.root {
			return e.meta.prev
		}
		return nil

	case WHEEL_LIST:
		if p := e.meta.wheelPrev; !p.meta.root {
			return e.meta.wheelPrev
		}
		return nil
	}
	return nil
}

func (e *Entry[K, V]) prev(listType uint8) *Entry[K, V] {
	switch listType {
	case LIST_PROBATION, LIST_PROTECTED:
		return e.meta.prev
	case WHEEL_LIST:
		return e.meta.wheelPrev
	}
	return nil
}

func (e *Entry[K, V]) next(listType uint8) *Entry[K, V] {
	switch listType {
	case LIST_PROBATION, LIST_PROTECTED:
		return e.meta.next
	case WHEEL_LIST:
		return e.meta.wheelNext
	}
	return nil
}

func (e *Entry[K, V]) setPrev(entry *Entry[K, V], listType uint8) {
	switch listType {
	case LIST_PROBATION, LIST_PROTECTED:
		e.meta.prev = entry
	case WHEEL_LIST:
		e.meta.wheelPrev = entry
	}
}

func (e *Entry[K, V]) setNext(entry *Entry[K, V], listType uint8) {
	switch listType {
	case LIST_PROBATION, LIST_PROTECTED:
		e.meta.next = entry
	case WHEEL_LIST:
		e.meta.wheelNext = entry
	}
}

func (e *Entry[K, V]) pentry() *Pentry[K, V] {
	return &Pentry[K, V]{
		Key:       e.key,
		Value:     e.value,
		Cost:      e.cost.Load(),
		Expire:    e.expire.Load(),
		Frequency: e.frequency.Load(),
		Removed:   e.removed,
	}
}

// entry for persistence
type Pentry[K comparable, V any] struct {
	Key       K
	Value     V
	Cost      int64
	Expire    int64
	Frequency int32
	Removed   bool
}

func (e *Pentry[K, V]) entry() *Entry[K, V] {
	en := &Entry[K, V]{
		key:     e.Key,
		value:   e.Value,
		removed: e.Removed,
	}
	en.cost.Store(e.Cost)
	en.frequency.Store(e.Frequency)
	en.expire.Store(e.Expire)
	return en
}
