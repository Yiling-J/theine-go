package internal

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gammazero/deque"
)

const (
	MAX_READ_BUFF_SIZE  = 64
	MIN_WRITE_BUFF_SIZE = 4
	MAX_WRITE_BUFF_SIZE = 1024
)

type RemoveReason uint8

const (
	REMOVED RemoveReason = iota
	EVICTED
	EXPIRED
)

type Shard[K comparable, V any] struct {
	hashmap   map[K]*Entry[K, V]
	mu        sync.RWMutex
	dookeeper *doorkeeper
	size      uint
	qsize     uint
	counter   uint
	deque     *deque.Deque[*Entry[K, V]]
	group     Group[K, Loaded[V]]
}

func NewShard[K comparable, V any](size uint, qsize uint) *Shard[K, V] {
	return &Shard[K, V]{
		hashmap:   make(map[K]*Entry[K, V], size),
		dookeeper: newDoorkeeper(int(20*size), 0.01),
		size:      size,
		qsize:     qsize,
		deque:     deque.New[*Entry[K, V]](int(qsize)),
	}
}

func (s *Shard[K, V]) set(key K, entry *Entry[K, V]) {
	s.hashmap[key] = entry
}

func (s *Shard[K, V]) get(key K) (entry *Entry[K, V], ok bool) {
	entry, ok = s.hashmap[key]
	return
}

func (s *Shard[K, V]) delete(entry *Entry[K, V]) bool {
	var deleted bool
	exist, ok := s.hashmap[entry.key]
	if ok && exist == entry {
		delete(s.hashmap, exist.key)
		deleted = true
	}
	return deleted
}

func (s *Shard[K, V]) len() int {
	return len(s.hashmap)
}

type Metrics struct {
}

type Store[K comparable, V any] struct {
	tailUpdate      bool
	cap             uint
	shards          []*Shard[K, V]
	shardCount      uint
	policy          *TinyLfu[K, V]
	timerwheel      *TimerWheel[K, V]
	readbuf         *Queue[ReadBufItem[K, V]]
	readCounter     *atomic.Uint32
	writebuf        chan WriteBufItem[K, V]
	hasher          *Hasher[K]
	entryPool       sync.Pool
	cost            func(V) int64
	mlock           sync.Mutex
	doorkeeper      bool
	closed          bool
	removalListener func(key K, value V, reason RemoveReason)
}

// New returns a new data struct with the specified capacity
func NewStore[K comparable, V any](maxsize int64) *Store[K, V] {
	hasher := NewHasher[K]()
	writeBufSize := maxsize / 100
	if writeBufSize < MIN_WRITE_BUFF_SIZE {
		writeBufSize = MIN_WRITE_BUFF_SIZE
	}
	if writeBufSize > MAX_WRITE_BUFF_SIZE {
		writeBufSize = MAX_WRITE_BUFF_SIZE
	}
	shardCount := 1
	for int(shardCount) < runtime.NumCPU()*8 {
		shardCount *= 2
	}
	dequeSize := int(maxsize) / 100 / shardCount
	shardSize := int(maxsize) / shardCount
	if shardSize < 50 {
		shardSize = 50
	}
	policySize := int(maxsize) - (dequeSize * shardCount)
	s := &Store[K, V]{
		cap:         uint(maxsize),
		hasher:      hasher,
		policy:      NewTinyLfu[K, V](uint(policySize), hasher),
		readCounter: &atomic.Uint32{},
		readbuf:     NewQueue[ReadBufItem[K, V]](),
		writebuf:    make(chan WriteBufItem[K, V], writeBufSize),
		entryPool:   sync.Pool{New: func() any { return &Entry[K, V]{} }},
		cost:        func(v V) int64 { return 1 },
		shardCount:  uint(shardCount),
		doorkeeper:  false,
	}
	s.shards = make([]*Shard[K, V], 0, s.shardCount)
	for i := 0; i < int(s.shardCount); i++ {
		s.shards = append(s.shards, NewShard[K, V](uint(shardSize), uint(dequeSize)))
	}

	s.timerwheel = NewTimerWheel[K, V](uint(maxsize))
	go s.maintance()
	return s
}

func (s *Store[K, V]) Cost(cost func(v V) int64) {
	s.cost = cost
}
func (s *Store[K, V]) Doorkeeper(enabled bool) {
	s.doorkeeper = enabled
}

func (s *Store[K, V]) RemovalListener(listener func(key K, value V, reason RemoveReason)) {
	s.removalListener = listener
}

func (s *Store[K, V]) getFromShard(key K, hash uint64, shard *Shard[K, V]) (V, bool) {
	new := s.readCounter.Add(1)
	shard.mu.RLock()
	entry, ok := shard.get(key)
	var value V
	if ok {
		expire := entry.expire.Load()
		if expire != 0 && expire <= s.timerwheel.clock.nowNano() {
			ok = false
		} else {
			s.policy.hit.Add(1)
			value = entry.value
		}
	}
	switch {
	case new < MAX_READ_BUFF_SIZE:
		var send ReadBufItem[K, V]
		send.hash = hash
		if ok {
			send.entry = entry
		}
		shard.mu.RUnlock()
		s.readbuf.Push(send)
	case new == MAX_READ_BUFF_SIZE:
		shard.mu.RUnlock()
		s.drainRead()
	default:
		shard.mu.RUnlock()
	}
	return value, ok
}

func (s *Store[K, V]) Get(key K) (V, bool) {
	s.policy.total.Add(1)
	h, index := s.index(key)
	shard := s.shards[index]
	return s.getFromShard(key, h, shard)
}

func (s *Store[K, V]) Set(key K, value V, cost int64, ttl time.Duration) bool {
	if cost == 0 {
		cost = s.cost(value)
	}
	if cost > int64(s.cap) {
		return false
	}
	h, index := s.index(key)
	shard := s.shards[index]
	var expire int64
	if ttl != 0 {
		expire = s.timerwheel.clock.expireNano(ttl)
	}
	shard.mu.Lock()
	exist, ok := shard.get(key)
	if ok {
		var reschedule bool
		var costChange int64
		exist.value = value
		shard.mu.Unlock()
		if expire > 0 {
			old := exist.expire.Swap(expire)
			if old != expire {
				reschedule = true
			}
		}
		oldCost := exist.cost.Swap(cost)
		if oldCost != cost {
			costChange = cost - oldCost
		}
		if reschedule || costChange != 0 {
			s.writebuf <- WriteBufItem[K, V]{
				entry: exist, code: UPDATE, costChange: costChange, rechedule: reschedule,
			}
		}
		return true
	}
	if s.doorkeeper {
		if shard.counter > 20*shard.size {
			shard.dookeeper.reset()
			shard.counter = 0
		}
		hit := shard.dookeeper.insert(h)
		if !hit {
			shard.counter += 1
			shard.mu.Unlock()
			return false
		}
	}
	entry := s.entryPool.Get().(*Entry[K, V])
	entry.frequency.Store(-1)
	entry.shard = uint16(index)
	entry.key = key
	entry.value = value
	entry.expire.Store(expire)
	entry.cost.Store(cost)
	shard.set(key, entry)
	// cost larger than deque size, send to policy directly
	if cost > int64(shard.qsize) {
		shard.mu.Unlock()
		s.writebuf <- WriteBufItem[K, V]{entry: entry, code: NEW}
		return true
	}
	shard.deque.PushFront(entry)
	var k K
	var v V
	if shard.deque.Len() > int(shard.qsize) {
		evicted := shard.deque.PopBack()
		expire := evicted.expire.Load()
		if expire != 0 && expire <= s.timerwheel.clock.nowNano() {
			deleted := shard.delete(evicted)
			if deleted {
				k, v = evicted.key, evicted.value
				s.postDelete(evicted, EXPIRED)
			}
			shard.mu.Unlock()
			if deleted {
				if s.removalListener != nil {
					s.removalListener(k, v, EXPIRED)
				}
			}
		} else {
			count := evicted.frequency.Load()
			if count == -1 {
				count = 0
			}
			if int32(count) >= s.policy.threshold.Load() {
				shard.mu.Unlock()
				s.writebuf <- WriteBufItem[K, V]{entry: evicted, code: NEW}
			} else {
				deleted := shard.delete(evicted)
				if deleted {
					k, v = evicted.key, evicted.value
					s.postDelete(evicted, EXPIRED)
				}
				shard.mu.Unlock()
				if deleted {
					if s.removalListener != nil {
						s.removalListener(k, v, EVICTED)
					}
				}
			}
		}
	} else {
		shard.mu.Unlock()
	}
	return true
}

func (s *Store[K, V]) Delete(key K) {
	_, index := s.index(key)
	shard := s.shards[index]
	shard.mu.Lock()
	entry, ok := shard.get(key)
	if ok {
		shard.delete(entry)
	}
	shard.mu.Unlock()
	if ok {
		s.writebuf <- WriteBufItem[K, V]{entry: entry, code: REMOVE}
	}
}

func (s *Store[K, V]) Len() int {
	total := 0
	for _, s := range s.shards {
		s.mu.RLock()
		total += s.len()
		s.mu.RUnlock()
	}
	return total
}

// spread hash before get index
func (s *Store[K, V]) index(key K) (uint64, int) {
	base := s.hasher.hash(key)
	h := ((base >> 16) ^ base) * 0x45d9f3b
	h = ((h >> 16) ^ h) * 0x45d9f3b
	h = (h >> 16) ^ h
	return base, int(h & uint64(s.shardCount-1))
}

func (s *Store[K, V]) postDelete(entry *Entry[K, V], reason RemoveReason) {
	var zero V
	entry.value = zero
	s.entryPool.Put(entry)
}

// remove entry from cache/policy/timingwheel and add back to pool
func (s *Store[K, V]) removeEntry(entry *Entry[K, V], reason RemoveReason) {
	if prev := entry.meta.prev; prev != nil {
		s.policy.Remove(entry)
		if prev.meta.root {
			s.tailUpdate = true
		}
	}
	if entry.meta.wheelPrev != nil {
		s.timerwheel.deschedule(entry)
	}
	var k K
	var v V
	switch reason {
	case EVICTED, EXPIRED:
		shard := s.shards[entry.shard]
		shard.mu.Lock()
		deleted := shard.delete(entry)
		shard.mu.Unlock()
		if deleted {
			k, v = entry.key, entry.value
			if s.removalListener != nil {
				s.removalListener(k, v, reason)
			}
			s.postDelete(entry, reason)
		}
	// already removed from shard map
	case REMOVED:
		shard := s.shards[entry.shard]
		shard.mu.RLock()
		k, v = entry.key, entry.value
		shard.mu.RUnlock()
		if s.removalListener != nil {
			s.removalListener(k, v, reason)
		}
	}
}

func (s *Store[K, V]) drainRead() {
	s.mlock.Lock()
	for {
		v, ok := s.readbuf.Pop()
		if !ok {
			break
		}
		s.policy.Access(v)
	}
	s.mlock.Unlock()
	s.readCounter.Store(0)
}

func (s *Store[K, V]) maintance() {
	go func() {
		for {
			time.Sleep(500 * time.Millisecond)
			s.mlock.Lock()
			if s.closed {
				s.mlock.Unlock()
				return
			}
			s.timerwheel.advance(0, s.removeEntry)
			if s.tailUpdate {
				s.policy.UpdateThreshold()
				s.tailUpdate = false
			}
			s.mlock.Unlock()
		}
	}()

	for item := range s.writebuf {
		s.mlock.Lock()
		entry := item.entry
		if entry == nil {
			s.mlock.Unlock()
			continue
		}

		// lock free because store API never read/modify entry metadata
		switch item.code {
		case NEW:
			if entry.expire.Load() != 0 {
				s.timerwheel.schedule(entry)
			}
			evicted := s.policy.Set(entry)
			if evicted != nil {
				s.removeEntry(evicted, EVICTED)
			}
			removed := s.policy.EvictEntries()
			for _, e := range removed {
				s.removeEntry(e, EVICTED)
			}
		case REMOVE:
			s.removeEntry(entry, REMOVED)
		case UPDATE:
			if item.rechedule {
				s.timerwheel.schedule(entry)
			}
			if item.costChange != 0 {
				s.policy.UpdateCost(entry, item.costChange)
				removed := s.policy.EvictEntries()
				for _, e := range removed {
					s.removeEntry(e, EVICTED)
				}
			}
		}
		item.entry = nil
		if s.tailUpdate {
			s.policy.UpdateThreshold()
			s.tailUpdate = false
		}
		s.mlock.Unlock()
	}
}

func (s *Store[K, V]) Close() {
	for _, s := range s.shards {
		s.mu.RLock()
		s.hashmap = nil
		s.mu.RUnlock()
	}
	s.mlock.Lock()
	s.closed = true
	s.mlock.Unlock()
	close(s.writebuf)
}

type Loaded[V any] struct {
	Value V
	Cost  int64
	TTL   time.Duration
}

type LoadingStore[K comparable, V any] struct {
	loader func(ctx context.Context, key K) (Loaded[V], error)
	*Store[K, V]
}

func NewLoadingStore[K comparable, V any](store *Store[K, V]) *LoadingStore[K, V] {
	return &LoadingStore[K, V]{
		Store: store,
	}
}

func (s *LoadingStore[K, V]) Loader(loader func(ctx context.Context, key K) (Loaded[V], error)) {
	s.loader = loader
}

func (s *LoadingStore[K, V]) Get(ctx context.Context, key K) (V, error) {
	s.policy.total.Add(1)
	h, index := s.index(key)
	shard := s.shards[index]
	v, ok := s.getFromShard(key, h, shard)
	if !ok {
		loaded, err, _ := shard.group.Do(key, func() (Loaded[V], error) {
			loaded, err := s.loader(ctx, key)
			if err == nil {
				s.Set(key, loaded.Value, loaded.Cost, loaded.TTL)
			}
			return loaded, err
		})
		return loaded.Value, err
	}
	return v, nil
}
