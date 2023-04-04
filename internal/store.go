package internal

import (
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tidwall/hashmap"
)

type OPCODE int8

const (
	SET                 OPCODE = 1
	DELETE              OPCODE = 2
	MAX_READ_BUFF_SIZE         = 128
	MAX_WRITE_BUFF_SIZE        = 64
	MAINTANCE                  = 1
)

type Shard[K comparable, V any] struct {
	hashmap *hashmap.Map[K, *Entry[K, V]]
	mu      sync.RWMutex
}

func NewShard[K comparable, V any](size uint) *Shard[K, V] {
	return &Shard[K, V]{
		hashmap: hashmap.New[K, *Entry[K, V]](int(size)),
	}
}

func (s *Shard[K, V]) set(key K, entry *Entry[K, V]) {
	s.hashmap.Set(key, entry)
}

func (s *Shard[K, V]) get(key K) (entry *Entry[K, V], ok bool) {
	entry, ok = s.hashmap.Get(key)
	return
}

func (s *Shard[K, V]) delete(entry *Entry[K, V]) bool {
	var deleted bool
	exist, ok := s.hashmap.Get(entry.key)
	if ok && exist == entry {
		_, deleted = s.hashmap.Delete(entry.key)
	}
	return deleted
}

func (s *Shard[K, V]) len() int {
	return s.hashmap.Len()
}

type Store[K comparable, V any] struct {
	cap         uint
	shards      []*Shard[K, V]
	shardCount  uint
	policy      *TinyLfu[K, V]
	timerwheel  *TimerWheel[K, V]
	readbuf     *Queue
	readCounter *atomic.Uint32
	writebuf    *LockedBuf[K, V]
	closeChan   chan int
	hasher      *Hasher[K]
	entryPool   sync.Pool
	removeBuf   []*Entry[K, V]
	drainLock   sync.Mutex
}

// New returns a new data struct with the specified capacity
func NewStore[K comparable, V any](cap uint) *Store[K, V] {
	hasher := NewHasher[K]()
	s := &Store[K, V]{
		cap: cap, hasher: hasher, policy: NewTinyLfu[K, V](cap, hasher),
		readCounter: &atomic.Uint32{},
		readbuf:     NewQueue(), writebuf: NewLockedBuf[K, V](MAX_WRITE_BUFF_SIZE),
		entryPool: sync.Pool{New: func() any { return &Entry[K, V]{} }},
		removeBuf: make([]*Entry[K, V], 0, MAX_WRITE_BUFF_SIZE*2),
	}
	s.shardCount = 1
	for int(s.shardCount) < runtime.NumCPU()*8 {
		s.shardCount *= 2
	}
	shardSize := cap / s.shardCount
	if shardSize < 50 {
		shardSize = 50
	}
	s.shards = make([]*Shard[K, V], 0, s.shardCount)
	for i := 0; i < int(s.shardCount); i++ {
		s.shards = append(s.shards, NewShard[K, V](shardSize))
	}

	s.closeChan = make(chan int)
	s.timerwheel = NewTimerWheel(cap, s.writebuf)
	go s.maintance()
	return s
}

func (s *Store[K, V]) Get(key K) (V, bool) {
	index := s.index(key)
	shard := s.shards[index]
	new := s.readCounter.Add(1)
	shard.mu.RLock()
	entry, ok := shard.get(key)
	var value V
	if ok {
		expire := entry.expire.Load()
		if expire != 0 && expire <= s.timerwheel.clock.nowNano() {
			ok = false
		} else {
			value = entry.value
		}
	}
	shard.mu.RUnlock()
	switch {
	case new < MAX_READ_BUFF_SIZE:
		if ok {
			s.readbuf.Push(entry)
		} else {
			s.readbuf.Push(s.hasher.hash(key))
		}
	case new == MAX_READ_BUFF_SIZE:
		s.drainRead()
	default:
	}
	return value, ok
}

func (s *Store[K, V]) Set(key K, value V, ttl time.Duration) {
	index := s.index(key)
	shard := s.shards[index]
	var expire int64
	if ttl != 0 {
		expire = s.timerwheel.clock.expireNano(ttl)
	}
	shard.mu.Lock()
	exist, ok := shard.get(key)
	if ok {
		exist.value = value
		if expire > 0 {
			exist.expire.Store(expire)
		}
		shard.mu.Unlock()
		return
	}
	entry := s.entryPool.Get().(*Entry[K, V])
	entry.shard = uint16(index)
	entry.key = key
	entry.value = value
	entry.expire.Store(expire)
	shard.set(key, entry)
	shard.mu.Unlock()
	full := s.writebuf.Push(BufItem[K, V]{entry: entry, code: ALIVE})
	if full {
		s.drainWrite()
	}
}

func (s *Store[K, V]) Delete(key K) {
	index := s.index(key)
	shard := s.shards[index]
	shard.mu.Lock()
	entry, ok := shard.get(key)
	if ok {
		shard.delete(entry)
	}
	shard.mu.Unlock()
	full := s.writebuf.Push(BufItem[K, V]{entry: entry, code: RETIRED})
	if full {
		s.drainWrite()
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

func (s *Store[K, V]) WriteBufLen() int {
	total := 0
	for _, s := range s.shards {
		s.mu.RLock()
		total += 1
		s.mu.RUnlock()
	}
	return total
}

// spread hash before get index
func (s *Store[K, V]) index(key K) int {
	h := s.hasher.hash(key)
	h = ((h >> 16) ^ h) * 0x45d9f3b
	h = ((h >> 16) ^ h) * 0x45d9f3b
	h = (h >> 16) ^ h
	return int(h & uint64(s.shardCount-1))
}

func (s *Store[K, V]) drainRead() {
	s.drainLock.Lock()
	for {
		v := s.readbuf.Pop()
		if v == nil {
			break
		}
		s.policy.Access(v)
	}
	s.readCounter.Store(0)
	s.drainLock.Unlock()
}

// remove entry from cache/policy/timingwheel and add back to pool
func (s *Store[K, V]) removeEntry(entry *Entry[K, V]) {
	if entry.list(LIST) != nil {
		s.policy.Remove(entry)
	}
	if entry.list(WHEEL_LIST) != nil {
		s.timerwheel.deschedule(entry)
	}
	shard := s.shards[entry.shard]
	shard.mu.Lock()
	deleted := shard.delete(entry)
	shard.mu.Unlock()
	if deleted {
		entry.removed = false
		s.entryPool.Put(entry)
	}
}

func (s *Store[K, V]) processWriteBuf(buf []BufItem[K, V]) {
	for _, item := range buf {
		entry := item.entry
		if entry == nil || entry.removed {
			continue
		}

		// no lock when updating policy,
		// because store API never read/modify entry metadata
		// all metadata related operations are done in maintance period
		switch item.code {
		case ALIVE:
			if entry.expire.Load() != 0 {
				s.timerwheel.schedule(entry)
			}
			evicted := s.policy.Set(entry)
			if evicted == nil {
				continue
			}
			evicted.removed = true
			s.removeBuf = append(s.removeBuf, evicted)
		case EXPIRED: // remove from cache and policy
			s.removeBuf = append(s.removeBuf, entry)
		}
		item.entry = nil
	}

	for i, entry := range s.removeBuf {
		s.removeEntry(entry)
		s.removeBuf[i] = nil
	}
	s.removeBuf = s.removeBuf[:0]
}

func (s *Store[K, V]) drainWrite() {
	s.drainLock.Lock()
	s.processWriteBuf(s.writebuf.buf)
	s.processWriteBuf(s.writebuf.extra)
	// reset counter and buf slice and extra
	s.writebuf.counter.Store(0)
	s.writebuf.extra = s.writebuf.extra[:0]
	s.writebuf.lock.Unlock()
	s.drainLock.Unlock()
}

func (s *Store[K, V]) maintance() {
	ticker := time.NewTicker(500 * time.Millisecond)
	for {
		select {
		case <-ticker.C:
			if s.writebuf.lock.TryLock() {
				s.drainRead()
				s.drainWrite()
			}
			s.drainLock.Lock()
			s.timerwheel.advance(0, s.drainWrite)
			s.drainLock.Unlock()
		case <-s.closeChan:
			return
		}
	}

}

func (s *Store[K, V]) Close() {
	close(s.closeChan)
}
