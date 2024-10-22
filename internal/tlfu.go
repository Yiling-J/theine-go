package internal

import (
	"math"
)

type TinyLfu[K comparable, V any] struct {
	window         *List[K, V]
	windowSize     uint
	slru           *Slru[K, V]
	sketch         *CountMinSketch
	hasher         *Hasher[K]
	size           uint
	counter        uint
	misses         *UnsignedCounter
	hits           *UnsignedCounter
	hitsPrev       uint64
	missesPrev     uint64
	hr             float32
	step           float32
	removeCallback func(entry *Entry[K, V])
}

func NewTinyLfu[K comparable, V any](size uint, hasher *Hasher[K]) *TinyLfu[K, V] {
	windowSize := uint(float32(size) * 0.01)
	if windowSize < 1 {
		windowSize = 1
	}
	mainSize := size - windowSize
	tlfu := &TinyLfu[K, V]{
		size:       size,
		slru:       NewSlru[K, V](mainSize),
		sketch:     NewCountMinSketch(),
		step:       -float32(size) * 0.0625,
		hasher:     hasher,
		misses:     NewUnsignedCounter(),
		hits:       NewUnsignedCounter(),
		windowSize: windowSize,
		window:     NewList[K, V](windowSize, LIST_WINDOW),
	}

	return tlfu
}

func (t *TinyLfu[K, V]) resizeWindow() {
	currentSize := t.window.capacity

	amount := int(t.windowSize - currentSize)

	if v := int(t.slru.protected.capacity); v >= amount {
		t.slru.protected.capacity = uint(v - amount)
	} else {
		amount = int(t.slru.protected.capacity)
		t.slru.protected.capacity = 0
	}

	t.windowSize = t.window.capacity + uint(amount)
	t.window.capacity = t.windowSize

	t.slru.maxsize = t.size - t.windowSize
	t.slru.probation.capacity = t.slru.maxsize

	// resize policy
	for t.window.Len() > int(t.window.capacity) {
		e := t.window.PopTail()
		if e == nil {
			break
		}
		ev := t.slru.probation.PushFront(e)
		if ev != nil {
			panic("ttt0")
		}
	}

	for t.slru.protected.Len() > int(t.slru.protected.capacity) {
		entry := t.slru.protected.PopTail()
		if entry == nil {
			break
		}
		evicted := t.slru.probation.PushFront(entry)
		if evicted != nil {
			ev := t.window.PushFront(evicted)
			if ev != nil {
				panic("ttt1")
			}
		}
	}

	for t.slru.probation.Len()+t.slru.protected.Len() > int(t.slru.maxsize) {
		evicted := t.slru.probation.PopTail()
		if evicted == nil {
			break
		}
		ev := t.window.PushFront(evicted)
		if ev != nil {
			panic("ttt2")
		}
	}
}

func (t *TinyLfu[K, V]) climb() {
	hits := t.hits.Value()
	misses := t.misses.Value()

	hitsInc := hits - t.hitsPrev
	missesInc := misses - t.missesPrev

	t.hitsPrev = hits
	t.missesPrev = misses

	current := float32(hitsInc) / float32(hitsInc+missesInc)
	delta := current - t.hr

	var amount float32
	if delta >= 0 {
		amount = t.step
	} else {
		amount = -t.step
	}

	nextStepSize := amount * 0.98
	if math.Abs(float64(delta)) >= 0.05 {
		nextStepSizeAbs := float32(t.size) * 0.0625
		if amount >= 0 {
			nextStepSize = nextStepSizeAbs
		} else {
			nextStepSize = -nextStepSizeAbs
		}
	}

	t.step = nextStepSize

	new := float32(t.windowSize) + amount
	if new > 0 {
		t.windowSize = uint(math.Min(float64(new), float64(t.size)*0.8))
	} else {
		t.windowSize = 1
	}
	t.hr = current
}

func (t *TinyLfu[K, V]) Set(entry *Entry[K, V]) *Entry[K, V] {

	if entry.meta.prev == nil {
		if victim := t.window.PushFront(entry); victim != nil {
			if tail := t.slru.victim(); tail != nil {
				victimFreq := t.sketch.Estimate(t.hasher.hash(victim.key))
				tailFreq := t.sketch.Estimate(t.hasher.hash(tail.key))

				if victimFreq <= tailFreq {
					return victim
				} else {
					return t.slru.insert(victim)
				}
			} else {
				count := t.slru.probation.count + t.slru.protected.count
				t.sketch.EnsureCapacity(uint(count))
				return t.slru.insert(victim)
			}
		}

	}
	return nil
}

func (t *TinyLfu[K, V]) Access(item ReadBufItem[K, V]) {
	t.counter++
	if t.counter > 10*t.size {
		t.climb()
		t.resizeWindow()
		t.counter = 0
	}

	if entry := item.entry; entry != nil {
		t.sketch.Add(item.hash)
		if entry.meta.prev != nil {
			if entry.flag.IsWindow() {
				t.window.MoveToFront(entry)
			} else {
				t.slru.access(entry)
			}
		}
	}
}

func (t *TinyLfu[K, V]) Remove(entry *Entry[K, V]) {
	if entry.flag.IsWindow() {
		t.window.Remove(entry)
	} else {
		t.slru.remove(entry)
	}
}

func (t *TinyLfu[K, V]) UpdateCost(entry *Entry[K, V], weightChange int64) {
	if entry.flag.IsWindow() {
		t.window.len.Add(weightChange)
	} else {
		t.slru.updateCost(entry, weightChange)
	}
}

func (t *TinyLfu[K, V]) EvictEntries() {

	for t.window.Len() > int(t.window.capacity) {
		e := t.window.PopTail()
		if e == nil {
			break
		}
		t.slru.probation.PushFront(e)
	}

	for t.slru.probation.Len()+t.slru.protected.Len() > int(t.slru.maxsize) {
		entry := t.slru.probation.PopTail()
		if entry == nil {
			break
		}
		t.removeCallback(entry)
	}
	for t.slru.probation.Len()+t.slru.protected.Len() > int(t.slru.maxsize) {
		entry := t.slru.protected.PopTail()
		if entry == nil {
			break
		}
		t.removeCallback(entry)
	}
}
