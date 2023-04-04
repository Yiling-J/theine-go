package internal

type TinyLfu[K comparable, V any] struct {
	size      uint
	lru       *Lru[K, V]
	slru      *Slru[K, V]
	sketch    *CountMinSketch
	lruFactor uint8
	total     uint
	hit       uint
	hr        float32
	step      int8
	hasher    *Hasher[K]
}

func NewTinyLfu[K comparable, V any](size uint, hasher *Hasher[K]) *TinyLfu[K, V] {
	lruSize := uint(float32(size) * 0.01)
	if lruSize == 0 {
		lruSize = 1
	}
	slruSize := size - lruSize
	return &TinyLfu[K, V]{
		size:   size,
		lru:    NewLru[K, V](lruSize),
		slru:   NewSlru[K, V](slruSize),
		sketch: NewCountMinSketch(size),
		step:   1,
		hasher: hasher,
	}
}

func (t *TinyLfu[K, V]) Set(entry *Entry[K, V]) *Entry[K, V] {
	// hill climbing lru factor
	if t.total >= 10*t.size && (t.total-t.hit) > t.size/2 {
		current := float32(t.hit) / float32(t.total)
		delta := current - t.hr
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
			} else if newFactor > 13 {
				newFactor = 13
			}
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
			} else if newFactor > 13 {
				newFactor = 13
			}
			t.lruFactor = uint8(newFactor)
		}
		t.hr = current
		t.hit = 0
		t.total = 0
	}

	// new entry
	if entry.list(LIST) == nil {
		if evicted := t.lru.insert(entry); evicted != nil {
			if victim := t.slru.victim(); victim != nil {
				evictedCount := t.sketch.Estimate(
					t.hasher.hash(evicted.key),
				) + uint(t.lruFactor)
				victimCount := t.sketch.Estimate(t.hasher.hash(victim.key))
				if evictedCount <= uint(victimCount) {
					return evicted
				}
			}
			return t.slru.insert(evicted)
		}
	}

	return nil
}

func (t *TinyLfu[K, V]) Access(entry interface{}) {
	t.total += 1
	switch v := entry.(type) {
	case *Entry[K, V]: // hit
		if v.list(LIST) == nil {
			return
		}
		t.sketch.Add(t.hasher.hash(v.key))
		t.hit += 1
		switch v.list(1) {
		case t.lru.list:
			t.lru.access(v)
		case t.slru.probation, t.slru.protected:
			t.slru.access(v)
		}

	case uint64: // miss
		t.sketch.Add(v)
	}
}

func (t *TinyLfu[K, V]) Remove(entry *Entry[K, V]) {
	entry.list(LIST).remove(entry)
}
