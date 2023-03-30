package internal

import (
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tidwall/hashmap"
	"github.com/zeebo/xxh3"
)

type OPCODE int8

const (
	SET                 OPCODE = 1
	DELETE              OPCODE = 2
	MAX_READ_BUFF_SIZE         = 128
	MAX_WRITE_BUFF_SIZE        = 64
	MAINTANCE                  = 1
)

type Writer struct {
	key   string
	code  OPCODE
	entry *Entry
}

type Shard struct {
	hashmap   *hashmap.Map[string, *Entry]
	mu        sync.RWMutex
	writeMu   sync.Mutex
	maintance bool
}

func NewShard(size uint) *Shard {
	return &Shard{
		hashmap: hashmap.New[string, *Entry](int(size)),
	}
}

func (s *Shard) set(key string, entry *Entry) {
	s.hashmap.Set(key, entry)
}

func (s *Shard) get(key string) (entry *Entry, ok bool) {
	entry, ok = s.hashmap.Get(key)
	return
}

func (s *Shard) delete(key string) {
	s.hashmap.Delete(key)
}

func (s *Shard) len() int {
	return s.hashmap.Len()
}

type Store struct {
	cap         uint
	shards      []*Shard
	shardCount  uint
	readChan    chan int
	writeChan   chan int
	policy      *TinyLfu
	timerwheel  *TimerWheel
	setCounter  *atomic.Int64
	readbuf     *Queue
	readCounter *atomic.Uint32
	writebuf    *Queue
}

// New returns a new data struct with the specified capacity
func NewStore(cap uint) *Store {
	s := &Store{cap: cap}
	s.shardCount = 1
	for int(s.shardCount) < runtime.NumCPU()*8 {
		s.shardCount *= 2
	}
	shardSize := cap / s.shardCount
	if shardSize < 50 {
		shardSize = 50
	}
	s.shards = make([]*Shard, 0, s.shardCount)
	for i := 0; i < int(s.shardCount); i++ {
		s.shards = append(s.shards, NewShard(shardSize))
	}

	s.policy = NewTinyLfu(cap)
	s.readChan = make(chan int)
	s.writeChan = make(chan int)
	s.setCounter = &atomic.Int64{}
	s.readCounter = &atomic.Uint32{}
	s.readbuf = NewQueue()
	s.writebuf = NewQueue()
	go s.maintance()
	return s
}

func NewStoreSync(cap uint) *Store {
	s := &Store{cap: cap}
	s.shardCount = 1
	for int(s.shardCount) < runtime.NumCPU()*8 {
		s.shardCount *= 2
	}
	shardSize := cap / s.shardCount
	if shardSize < 50 {
		shardSize = 50
	}
	s.shards = make([]*Shard, 0, s.shardCount)
	for i := 0; i < int(s.shardCount); i++ {
		s.shards = append(s.shards, NewShard(shardSize))
	}
	s.policy = NewTinyLfu(cap)
	s.setCounter = &atomic.Int64{}
	return s
}

func (s *Store) Get(key string) (interface{}, bool) {
	index := s.index(key)
	shard := s.shards[index]
	new := s.readCounter.Add(1)
	shard.mu.RLock()
	entry, ok := shard.get(key)
	shard.mu.RUnlock()
	if ok && entry.expire != 0 && entry.expire <= s.timerwheel.clock.nowNano() {
		ok = false
	}
	switch {
	case new < MAX_READ_BUFF_SIZE:
		if ok {
			s.readbuf.Push(entry)
		} else {
			s.readbuf.Push(key)
		}
	case new == MAX_READ_BUFF_SIZE:
		s.readChan <- MAINTANCE
	default:
	}
	if ok {
		return entry.value, ok
	}
	return nil, ok
}

func (s *Store) Set(key string, value interface{}, ttl time.Duration) {
	index := s.index(key)
	shard := s.shards[index]
	shard.mu.Lock()
	entry, ok := shard.get(key)
	if ok {
		entry.value = value
		if ttl != 0 {
			entry.expire = s.timerwheel.clock.expireNano(ttl)
		}
		shard.mu.Unlock()
	} else {
		entry = &Entry{status: ALIVE}
		entry.key = key
		entry.value = value
		if ttl != 0 {
			entry.expire = s.timerwheel.clock.expireNano(ttl)
		}
		shard.set(key, entry)
		shard.mu.Unlock()

	}

	// update set counter
	new := s.setCounter.Add(1)
	s.writebuf.Push(entry)
	if new%MAX_WRITE_BUFF_SIZE == 0 {
		s.writeChan <- MAINTANCE
	}
}

func (s *Store) SetSync(key string, value interface{}, ttl time.Duration) {
	index := s.index(key)
	shard := s.shards[index]
	exist, ok := shard.get(key)
	if ok {
		exist.value = value
		if ttl != 0 {
			exist.expire = s.timerwheel.clock.expireNano(ttl)
		}
		s.policy.Set(exist)
	} else {
		entry := &Entry{}
		entry.key = key
		entry.value = value
		if ttl != 0 {
			entry.expire = s.timerwheel.clock.expireNano(ttl)
		}
		shard.set(key, entry)
		evicted := s.policy.Set(entry)
		if evicted != nil {
			s.shards[s.index(evicted.key)].delete(evicted.key)
		}
	}
}

func (s *Store) Delete(key string) {
	index := s.index(key)
	shard := s.shards[index]
	shard.mu.Lock()
	entry, ok := shard.get(key)
	if ok {
		s.writebuf.Push(Writer{key: key, code: DELETE, entry: entry})
		shard.delete(key)
	}
	var maintance bool
	new := s.setCounter.Add(1)
	if new > MAX_WRITE_BUFF_SIZE {
		maintance = true
	}
	shard.mu.Unlock()
	if maintance {
		s.writeChan <- MAINTANCE
	}
}

func (s *Store) Len() int {
	total := 0
	for _, s := range s.shards {
		s.mu.RLock()
		total += s.len()
		s.mu.RUnlock()
	}
	return total
}

func (s *Store) Data() string {
	all := []string{}
	for _, s := range s.shards {
		s.mu.RLock()
		all = append(all, s.hashmap.Keys()...)
		s.mu.RUnlock()
	}
	return strings.Join(all, "/")
}

func (s *Store) WriteBufLen() int {
	total := 0
	for _, s := range s.shards {
		s.mu.RLock()
		total += 1
		s.mu.RUnlock()
	}
	return total
}

func (s *Store) index(key string) int {
	return int(xxh3.HashString(key) & uint64(s.shardCount-1))
}

func (s *Store) Maintance() {
}

func (s *Store) drainRead() {
	for {
		v := s.readbuf.Pop()
		if v == nil {
			break
		}
		s.policy.Access(v)
	}
	s.readCounter.Store(0)
}

func (s *Store) drainWrite() {
	for {
		v := s.writebuf.Pop()
		if v == nil {
			break
		}
		entry := v.(*Entry)

		// no lock when updating policy,
		// because store API never read/modify entry metadata
		// all metadata related operations are done in maintance period
		switch entry.status {
		case ALIVE:
			evicted := s.policy.Set(entry)
			if evicted == nil {
				continue
			}
			evicted.status = RETIRED
			index := s.index(evicted.key)
			shard := s.shards[index]
			shard.mu.Lock()
			shard.delete(evicted.key)
			shard.mu.Unlock()
		case RETIRED:
			// s.policy.Remove(entry)
		}
	}

}

func (s *Store) maintance() {
	ticker := time.NewTicker(5000 * time.Millisecond)
	for {
		select {
		case <-s.readChan:
			s.drainRead()
		case <-s.writeChan:
			s.drainWrite()
		case <-ticker.C:
			s.drainRead()
			s.drainWrite()
		}
	}

}
