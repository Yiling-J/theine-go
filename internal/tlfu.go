package internal

import (
	"sync/atomic"
)

type TinyLfu[K comparable, V any] struct {
	slru      *Slru[K, V]
	sketch    *CountMinSketch
	hasher    *Hasher[K]
	size      uint
	counter   uint
	total     atomic.Uint32
	hit       atomic.Uint32
	hr        float32
	threshold atomic.Int32
	lruFactor uint8
	step      int8
}

func NewTinyLfu[K comparable, V any](size uint, hasher *Hasher[K]) *TinyLfu[K, V] {
	return &TinyLfu[K, V]{
		size:   size,
		slru:   NewSlru[K, V](size),
		sketch: NewCountMinSketch(size),
		step:   1,
		hasher: hasher,
	}
}

func (t *TinyLfu[K, V]) climb() {
	total := t.total.Load()
	hit := t.hit.Load()
	current := float32(hit) / float32(total)
	delta := current - t.hr
	var diff int8
	if delta > 0.0 {
		if t.step < 0 {
			t.step -= 1
		} else {
			t.step += 1
		}
		if t.step < -13 {
			t.step = -13
		} else if t.step > 13 {
			t.step = 13
		}
		newFactor := int8(t.lruFactor) + t.step
		if newFactor < 0 {
			newFactor = 0
		} else if newFactor > 16 {
			newFactor = 16
		}
		diff = newFactor - int8(t.lruFactor)
		t.lruFactor = uint8(newFactor)
	} else if delta < 0.0 {
		// reset
		if t.step > 0 {
			t.step = -1
		} else {
			t.step = 1
		}
		newFactor := int8(t.lruFactor) + t.step
		if newFactor < 0 {
			newFactor = 0
		} else if newFactor > 16 {
			newFactor = 16
		}
		diff = newFactor - int8(t.lruFactor)
		t.lruFactor = uint8(newFactor)
	}
	t.threshold.Add(-int32(diff))
	t.hr = current
	t.hit.Store(0)
	t.total.Store(0)
}

func (t *TinyLfu[K, V]) Set(entry *Entry[K, V]) *Entry[K, V] {
	// new entry
	t.counter++
	if t.counter > 10*t.size {
		t.climb()
		t.counter = 0
	}
	if entry.meta.prev == nil {
		if victim := t.slru.victim(); victim != nil {
			freq := int(entry.frequency.Load())
			if freq == -1 {
				freq = int(t.sketch.Estimate(t.hasher.hash(entry.key)))
			}
			evictedCount := uint(freq) + uint(t.lruFactor)
			victimCount := t.sketch.Estimate(t.hasher.hash(victim.key))
			if evictedCount <= uint(victimCount) {
				t.threshold.Store(int32(victimCount) - int32(t.lruFactor))
				return entry
			} else {
				t.threshold.Store(-int32(t.lruFactor))
			}
		}
		evicted := t.slru.insert(entry)
		t.threshold.Store(0)
		return evicted
	}

	return nil
}

func (t *TinyLfu[K, V]) Access(item ReadBufItem[K, V]) {
	t.counter++
	if t.counter > 10*t.size {
		t.climb()
		t.counter = 0
	}
	if entry := item.entry; entry != nil {
		reset := t.sketch.Add(item.hash)
		if reset {
			t.threshold.Store(t.threshold.Load() / 2)
		}
		if entry.meta.prev != nil {
			var tail bool
			if entry == t.slru.victim() {
				tail = true
			}
			t.slru.access(entry)
			if tail {
				t.threshold.Store(0)
			}
		} else {
			entry.frequency.Store(int32(t.sketch.Estimate(item.hash)))
		}
	} else {
		reset := t.sketch.Add(item.hash)
		if reset {
			t.threshold.Store(t.threshold.Load() / 2)
		}
	}
}

func (t *TinyLfu[K, V]) Remove(entry *Entry[K, V]) {
	t.slru.remove(entry)
}

func (t *TinyLfu[K, V]) UpdateCost(entry *Entry[K, V], delta int64) {
	t.slru.updateCost(entry, delta)
}

func (t *TinyLfu[K, V]) EvictEntries() []*Entry[K, V] {
	removed := []*Entry[K, V]{}

	for t.slru.probation.Len()+t.slru.protected.Len() > int(t.slru.maxsize) {
		entry := t.slru.probation.PopTail()
		if entry == nil {
			break
		}
		removed = append(removed, entry)
	}
	if len(removed) > 0 {
		t.threshold.Store(0)
	}
	for t.slru.probation.Len()+t.slru.protected.Len() > int(t.slru.maxsize) {
		entry := t.slru.protected.PopTail()
		if entry == nil {
			break
		}
		removed = append(removed, entry)
	}
	return removed
}

func (t *TinyLfu[K, V]) UpdateThreshold() {
	tail := t.slru.victim()
	if tail != nil {
		t.threshold.Store(
			int32(t.sketch.Estimate(t.hasher.hash(tail.key))),
		)
	} else {
		t.threshold.Store(0)
	}
}
