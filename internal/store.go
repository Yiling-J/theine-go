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

func (s *Shard[K, V]) delete(key K) {
	s.hashmap.Delete(key)
}

func (s *Shard[K, V]) len() int {
	return s.hashmap.Len()
}

type Store[K comparable, V any] struct {
	cap          uint
	shards       []*Shard[K, V]
	shardCount   uint
	readChan     chan int
	writeChan    chan int
	policy       *TinyLfu[K, V]
	timerwheel   *TimerWheel[K, V]
	writeCounter *atomic.Uint32
	readbuf      *Queue
	readCounter  *atomic.Uint32
	writebuf     *LockedBuf[K, V]
	closeChan    chan int
	hasher       *Hasher[K]
}

// New returns a new data struct with the specified capacity
func NewStore[K comparable, V any](cap uint) *Store[K, V] {
	hasher := NewHasher[K]()
	s := &Store[K, V]{
		cap: cap, hasher: hasher, policy: NewTinyLfu[K, V](cap, hasher),
		readChan: make(chan int), writeChan: make(chan int),
		readCounter: &atomic.Uint32{}, writeCounter: &atomic.Uint32{},
		readbuf: NewQueue(), writebuf: NewLockedBuf[K, V](MAX_WRITE_BUFF_SIZE),
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
		if entry.expire != 0 && entry.expire <= s.timerwheel.clock.nowNano() {
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
		s.readChan <- MAINTANCE
	default:
	}
	return value, ok
}

func (s *Store[K, V]) Set(key K, value V, ttl time.Duration) {
	index := s.index(key)
	shard := s.shards[index]
	shard.mu.Lock()
	exist, ok := shard.get(key)
	entry := &Entry[K, V]{shard: uint16(index), key: key, value: value}
	if ttl != 0 {
		entry.expire = s.timerwheel.clock.expireNano(ttl)
	}
	shard.set(key, entry)
	shard.mu.Unlock()

	if ok {
		full := s.writebuf.Push(BufItem[K, V]{entry: exist, code: RETIRED})
		if full {
			s.writeChan <- MAINTANCE
		}
	}
	full := s.writebuf.Push(BufItem[K, V]{entry: entry, code: ALIVE})
	if full {
		s.writeChan <- MAINTANCE
	}
}

func (s *Store[K, V]) Delete(key K) {
	index := s.index(key)
	shard := s.shards[index]
	shard.mu.Lock()
	entry, ok := shard.get(key)
	if ok {
		s.writebuf.Push(BufItem[K, V]{entry: entry, code: RETIRED})
		shard.delete(key)
	}
	var maintance bool
	new := s.writeCounter.Add(1)
	if new > MAX_WRITE_BUFF_SIZE {
		maintance = true
	}
	shard.mu.Unlock()
	if maintance {
		s.writeChan <- MAINTANCE
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
	for {
		v := s.readbuf.Pop()
		if v == nil {
			break
		}
		s.policy.Access(v)
	}
	s.readCounter.Store(0)
}

// remove entry from cache/policy/timingwheel
func (s *Store[K, V]) removeEntry(entry *Entry[K, V]) {
	shard := s.shards[entry.shard]
	shard.mu.Lock()
	shard.delete(entry.key)
	shard.mu.Unlock()
	s.timerwheel.deschedule(entry)
}

func (s *Store[K, V]) processWriteBuf(buf []BufItem[K, V]) {
	for _, item := range buf {
		entry := item.entry
		if entry == nil {
			continue
		}

		// no lock when updating policy,
		// because store API never read/modify entry metadata
		// all metadata related operations are done in maintance period
		switch item.code {
		case ALIVE:
			if entry.removed {
				continue
			}
			if entry.expire != 0 {
				s.timerwheel.schedule(entry)
			}
			evicted := s.policy.Set(entry)
			if evicted == nil {
				continue
			}
			s.removeEntry(evicted)
			evicted.removed = true
		case RETIRED: // remove from policy and wheel
			if entry.removed || entry.list(LIST) == nil {
				continue
			}
			s.policy.Remove(entry)
			s.timerwheel.deschedule(entry)
			entry.removed = true
		case EXPIRED: // remove from cache and policy
			if entry.removed {
				continue
			}
			s.policy.Remove(entry)
			shard := s.shards[entry.shard]
			shard.mu.Lock()
			shard.delete(entry.key)
			shard.mu.Unlock()
			entry.removed = true
		}
		item.entry = nil
	}
}

func (s *Store[K, V]) drainWrite() {
	s.processWriteBuf(s.writebuf.buf)
	s.processWriteBuf(s.writebuf.extra)
	// reset counter and buf slice and extra
	s.writebuf.counter.Store(0)
	s.writebuf.extra = s.writebuf.extra[:0]
	s.writebuf.lock.Unlock()
}

func (s *Store[K, V]) maintance() {
	ticker := time.NewTicker(500 * time.Millisecond)
	for {
		select {
		case <-s.readChan:
			s.drainRead()
		case <-s.writeChan:
			s.drainWrite()
		case <-ticker.C:
			locked := s.writebuf.lock.TryLock()
			if locked {
				s.drainRead()
				s.drainWrite()
			}
			s.timerwheel.advance(0, s.drainWrite)
		case <-s.closeChan:
			return
		}
	}

}

func (s *Store[K, V]) Close() {
	close(s.closeChan)
}
