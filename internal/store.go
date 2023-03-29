package internal

import (
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cespare/xxhash/v2"
)

type OPCODE int8

const (
	SET                 OPCODE = 1
	DELETE              OPCODE = 2
	MAX_READ_BUFF_SIZE         = 64
	MAX_WRITE_BUFF_SIZE        = 32
	MAINTANCE                  = 1
)

type Writer struct {
	key   string
	code  OPCODE
	entry *Entry
}

type Shard struct {
	hashmap     map[string]*Entry
	readbuf     []string
	readCounter *atomic.Uint32
	writebuf    []Writer
	mu          sync.RWMutex
	writeMu     sync.Mutex
	maintance   bool
}

func NewShard(size uint) *Shard {
	return &Shard{
		hashmap:     make(map[string]*Entry, size),
		readbuf:     make([]string, MAX_READ_BUFF_SIZE),
		writebuf:    make([]Writer, 0, MAX_WRITE_BUFF_SIZE),
		readCounter: &atomic.Uint32{},
	}
}

func (s *Shard) set(key string, entry *Entry) {
	s.hashmap[key] = entry
}

func (s *Shard) get(key string) (entry *Entry, ok bool) {
	entry, ok = s.hashmap[key]
	return
}

func (s *Shard) delete(key string) {
	delete(s.hashmap, key)
}

func (s *Shard) len() int {
	return len(s.hashmap)
}

type Store struct {
	cap           uint
	shards        []*Shard
	shardCount    uint
	maintanceChan chan int
	policy        *TinyLfu
	timerwheel    *TimerWheel
	entryPool     *sync.Pool
	setCounter    *atomic.Int64
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
	s.entryPool = &sync.Pool{New: func() any {
		return &Entry{}
	}}
	s.policy = NewTinyLfu(cap)
	s.maintanceChan = make(chan int)
	s.setCounter = &atomic.Int64{}
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
	s.entryPool = &sync.Pool{New: func() any {
		return &Entry{status: NEW}
	}}
	s.policy = NewTinyLfu(cap)
	s.setCounter = &atomic.Int64{}
	return s
}

func (s *Store) Get(key string) (interface{}, bool) {
	index := s.index(key)
	shard := s.shards[index]
	shard.mu.RLock()
	var maintance bool
	defer func() {
		shard.mu.RUnlock()
		if maintance {
			// skip if already in maintance
			select {
			case s.maintanceChan <- MAINTANCE:
			default:
			}
		}
	}()
	entry, ok := shard.get(key)
	if ok && entry.expire != 0 && entry.expire <= s.timerwheel.clock.nowNano() {
		ok = false
	}
	new := shard.readCounter.Add(1)
	if new < MAX_READ_BUFF_SIZE {
		shard.readbuf[new] = key
	} else {
		maintance = true
	}
	if ok {
		return entry.value, ok
	}
	return nil, ok
}

func (s *Store) Set(key string, value interface{}, ttl time.Duration) {
	index := s.index(key)
	shard := s.shards[index]
	maintance := false
	shard.mu.RLock()
	exist, ok := shard.hashmap[key]
	shard.mu.RUnlock()
	if ok {
		shard.mu.Lock()
		exist.value = value
		if ttl != 0 {
			exist.expire = s.timerwheel.clock.expireNano(ttl)
		}
		shard.mu.Unlock()
	} else {
		for {
			entry := s.entryPool.Get().(*Entry)
			if entry.status == ALIVE || entry.status == RETIRED {
				continue
			}
			entry.key = key
			entry.value = value
			if ttl != 0 {
				entry.expire = s.timerwheel.clock.expireNano(ttl)
			}
			shard.mu.Lock()
			shard.hashmap[key] = entry
			shard.mu.Unlock()
			break
		}

	}

	// update set counter
	new := s.setCounter.Add(1)
	if new > MAX_WRITE_BUFF_SIZE {
		maintance = true
	}

	// update writebuf
	shard.writeMu.Lock()
	shard.writebuf = append(shard.writebuf, Writer{key: key, code: SET})
	shard.writeMu.Unlock()

	if maintance {
		s.maintanceChan <- MAINTANCE
	}
}

func (s *Store) SetSync(key string, value interface{}, ttl time.Duration) {
	index := s.index(key)
	shard := s.shards[index]
	exist, ok := shard.hashmap[key]
	if ok {
		exist.value = value
		if ttl != 0 {
			exist.expire = s.timerwheel.clock.expireNano(ttl)
		}
		s.policy.Set(exist)
	} else {
		entry := s.entryPool.Get().(*Entry)
		entry.key = key
		entry.value = value
		if ttl != 0 {
			entry.expire = s.timerwheel.clock.expireNano(ttl)
		}
		shard.hashmap[key] = entry
		evicted := s.policy.Set(entry)
		if evicted != nil {
			delete(s.shards[s.index(evicted.key)].hashmap, evicted.key)
		}
	}
}

func (s *Store) Delete(key string) {
	index := s.index(key)
	shard := s.shards[index]
	shard.mu.Lock()
	entry, ok := shard.hashmap[key]
	if ok {
		shard.writebuf = append(shard.writebuf, Writer{key: key, code: DELETE, entry: entry})
		delete(shard.hashmap, key)
	}
	var maintance bool
	new := s.setCounter.Add(1)
	if new > MAX_WRITE_BUFF_SIZE {
		maintance = true
	}
	shard.mu.Unlock()
	if maintance {
		s.maintanceChan <- MAINTANCE
	}
}

func (s *Store) Len() int {
	total := 0
	for _, s := range s.shards {
		s.mu.RLock()
		total += len(s.hashmap)
		s.mu.RUnlock()
	}
	return total
}

func (s *Store) Data() string {
	all := []string{}
	for _, s := range s.shards {
		s.mu.RLock()
		for k := range s.hashmap {
			all = append(all, k)
		}
		s.mu.RUnlock()
	}
	return strings.Join(all, "/")
}

func (s *Store) WriteBufLen() int {
	total := 0
	for _, s := range s.shards {
		s.mu.RLock()
		total += len(s.writebuf)
		s.mu.RUnlock()
	}
	return total
}

func (s *Store) index(key string) int {
	return int(xxhash.Sum64String(key) & uint64(s.shardCount-1))
}

func (s *Store) Maintance() {
	s.setCounter.Store(44)
	s.maintanceChan <- MAINTANCE
}

func (s *Store) drainRead() {
	for _, shard := range s.shards {
		if shard.readCounter.Load() < MAX_READ_BUFF_SIZE {
			continue
		}

		shard.mu.Lock()
		readbuf := shard.readbuf
		shard.readbuf = make([]string, MAX_READ_BUFF_SIZE)
		shard.mu.Unlock()

		new := []interface{}{}
		shard.mu.RLock()
		for _, key := range readbuf {
			if key == "" {
				continue
			}
			entry, ok := shard.hashmap[key]
			if ok {
				new = append(new, entry)
			} else {
				new = append(new, key)
			}
		}
		shard.mu.RUnlock()
		shard.readCounter.Store(0)
		for _, v := range new {
			s.policy.Access(v)
		}
	}
}

func (s *Store) drainWrite() map[int][]*Entry {
	if s.setCounter.Load() <= MAX_WRITE_BUFF_SIZE {
		return nil
	}
	defer s.setCounter.Add(-MAX_WRITE_BUFF_SIZE)
	removedMap := map[int][]*Entry{}

	for _, shard := range s.shards {
		shard.writeMu.Lock()
		writebuf := shard.writebuf
		shard.writebuf = []Writer{}
		shard.writeMu.Unlock()
		// get from hashmap again to avoid race if op is set
		shard.mu.RLock()
		for _, w := range writebuf {
			if w.code == DELETE {
				continue
			}
			entry, ok := shard.hashmap[w.key]
			if !ok {
				continue
			}
			w.entry = entry
			writebuf = append(writebuf, w)
		}
		shard.mu.RUnlock()

		// no lock when updating policy,
		// because store API never read/modify entry metadata
		// all metadata related operations are done in maintance period
		for _, v := range writebuf {
			if v.entry == nil {
				continue
			}
			switch v.code {
			case SET:
				// a remove->set will make removed one alive again
				v.entry.status = ALIVE
				evicted := s.policy.Set(v.entry)
				if evicted == nil {
					continue
				}
				evicted.status = RETIRED
				index := s.index(evicted.key)
				shard, ok := removedMap[index]
				if !ok {
					removedMap[index] = []*Entry{evicted}
				} else {
					removedMap[index] = append(shard, evicted)
				}
			case DELETE:
				v.entry.status = RETIRED
				s.policy.Remove(v.entry)
				index := s.index(v.entry.key)
				shard, ok := removedMap[index]
				if !ok {
					removedMap[index] = []*Entry{v.entry}
				} else {
					removedMap[index] = append(shard, v.entry)
				}
			}
		}
	}
	return removedMap
}

// process read/write buf
func (s *Store) drainBuf() {
	s.drainRead()
	removed := s.drainWrite()

	// obtain lock from shard before remove from map
	for index, entries := range removed {
		if len(entries) == 0 {
			continue
		}
		shard := s.shards[index]
		shard.mu.Lock()
		for _, entry := range entries {
			// another set make this entry alive again
			if entry.status == ALIVE {
				continue
			}
			// already removed
			if entry.status == REMOVED {
				continue
			}
			_, ok := shard.hashmap[entry.key]
			if ok {
				delete(shard.hashmap, entry.key)
				entry.status = REMOVED
				s.entryPool.Put(entry)
			}
		}
		shard.mu.Unlock()
	}
}

func (s *Store) maintance() {
	ticker := time.NewTicker(5000 * time.Millisecond)
	for {
		select {
		case <-s.maintanceChan:
			s.drainBuf()
		case <-ticker.C:
			// expire entries
		}
	}

}
