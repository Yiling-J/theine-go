package internal

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"io"
	"math/rand"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gammazero/deque"
	"github.com/zeebo/xxh3"

	"github.com/Yiling-J/theine-go/internal/bf"
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

var (
	VersionMismatch = errors.New("version mismatch")
)

type Shard[K comparable, V any] struct {
	hashmap   map[K]*Entry[K, V]
	dookeeper *bf.Bloomfilter
	deque     *deque.Deque[*Entry[K, V]]
	group     *Group[K, Loaded[V]]
	vgroup    *Group[K, V] // used in secondary cache
	size      uint
	qsize     uint
	qlen      int
	counter   uint
	mu        sync.RWMutex
}

func NewShard[K comparable, V any](size uint, qsize uint, doorkeeper bool) *Shard[K, V] {
	s := &Shard[K, V]{
		hashmap: make(map[K]*Entry[K, V]),
		size:    size,
		qsize:   qsize,
		deque:   deque.New[*Entry[K, V]](),
		group:   NewGroup[K, Loaded[V]](),
		vgroup:  NewGroup[K, V](),
	}
	if doorkeeper {
		s.dookeeper = bf.New(0.01)
	}
	return s
}

func (s *Shard[K, V]) set(key K, entry *Entry[K, V]) {
	s.hashmap[key] = entry
	if s.dookeeper != nil {
		ds := 20 * len(s.hashmap)
		if ds > s.dookeeper.Capacity {
			s.dookeeper.EnsureCapacity(ds)
		}
	}
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
	entryPool         sync.Pool
	writebuf          chan WriteBufItem[K, V]
	hasher            *Hasher[K]
	removalListener   func(key K, value V, reason RemoveReason)
	removalCallback   func(kv dequeKV[K, V], reason RemoveReason) error
	kvBuilder         func(entry *Entry[K, V]) dequeKV[K, V]
	policy            *TinyLfu[K, V]
	timerwheel        *TimerWheel[K, V]
	readbuf           *Queue[ReadBufItem[K, V]]
	cost              func(V) int64
	readCounter       *atomic.Uint32
	shards            []*Shard[K, V]
	cap               uint
	shardCount        uint
	mlock             sync.Mutex
	tailUpdate        bool
	doorkeeper        bool
	closed            bool
	secondaryCache    SecondaryCache[K, V]
	secondaryCacheBuf chan SecondaryCacheItem[K, V]
	probability       float32
	rg                *rand.Rand
}

// New returns a new data struct with the specified capacity
func NewStore[K comparable, V any](
	maxsize int64, doorkeeper bool, listener func(key K, value V, reason RemoveReason),
	cost func(v V) int64, secondaryCache SecondaryCache[K, V], workers int, probability float32, stringKeyFunc func(k K) string,
) *Store[K, V] {
	hasher := NewHasher(stringKeyFunc)
	writeBufSize := maxsize / 100
	if writeBufSize < MIN_WRITE_BUFF_SIZE {
		writeBufSize = MIN_WRITE_BUFF_SIZE
	}
	if writeBufSize > MAX_WRITE_BUFF_SIZE {
		writeBufSize = MAX_WRITE_BUFF_SIZE
	}
	shardCount := 1
	for shardCount < runtime.GOMAXPROCS(0)*2 {
		shardCount *= 2
	}
	if shardCount < 16 {
		shardCount = 16
	}
	if shardCount > 128 {
		shardCount = 128
	}
	dequeSize := int(maxsize) / 100 / shardCount
	shardSize := int(maxsize) / shardCount
	if shardSize < 50 {
		shardSize = 50
	}
	policySize := int(maxsize) - (dequeSize * shardCount)
	costfn := func(v V) int64 { return 1 }
	if cost != nil {
		costfn = cost
	}

	s := &Store[K, V]{
		cap:         uint(maxsize),
		hasher:      hasher,
		policy:      NewTinyLfu[K, V](uint(policySize), hasher),
		readCounter: &atomic.Uint32{},
		readbuf:     NewQueue[ReadBufItem[K, V]](),
		writebuf:    make(chan WriteBufItem[K, V], writeBufSize),
		entryPool:   sync.Pool{New: func() any { return &Entry[K, V]{} }},
		shardCount:  uint(shardCount),
		doorkeeper:  doorkeeper,
		kvBuilder: func(entry *Entry[K, V]) dequeKV[K, V] {
			return dequeKV[K, V]{
				k: entry.key,
				v: entry.value,
			}
		},
		removalListener: listener,
		cost:            costfn,
		secondaryCache:  secondaryCache,
		probability:     probability,
	}
	s.removalCallback = func(kv dequeKV[K, V], reason RemoveReason) error {
		if s.removalListener != nil {
			s.removalListener(kv.k, kv.v, reason)
		}
		return nil
	}
	s.shards = make([]*Shard[K, V], 0, s.shardCount)
	for i := 0; i < int(s.shardCount); i++ {
		s.shards = append(s.shards, NewShard[K, V](uint(shardSize), uint(dequeSize), doorkeeper))
	}

	s.timerwheel = NewTimerWheel[K, V](uint(maxsize))
	go s.maintance()
	if s.secondaryCache != nil {
		s.secondaryCacheBuf = make(chan SecondaryCacheItem[K, V], 256)
		s.secondaryCache.SetClock(s.timerwheel.clock)
		for i := 0; i < workers; i++ {
			go s.processSecondary()
		}
		s.rg = rand.New(rand.New(rand.NewSource(0)))
	}
	return s
}

func (s *Store[K, V]) getFromShard(key K, hash uint64, shard *Shard[K, V]) (V, bool) {
	new := s.readCounter.Add(1)
	shard.mu.RLock()
	entry, ok := shard.get(key)
	var value V
	if ok {
		expire := entry.expire.Load()
		if expire != 0 && expire <= s.timerwheel.clock.NowNano() {
			ok = false
		} else {
			s.policy.hit.Add(1)
			value = entry.value
		}
	}
	shard.mu.RUnlock()
	switch {
	case new < MAX_READ_BUFF_SIZE:
		var send ReadBufItem[K, V]
		send.hash = hash
		if ok {
			send.entry = entry
		}
		s.readbuf.Push(send)
	case new == MAX_READ_BUFF_SIZE:
		var send ReadBufItem[K, V]
		send.hash = hash
		if ok {
			send.entry = entry
		}
		s.readbuf.Push(send)
		s.drainRead()
	}
	return value, ok
}

func (s *Store[K, V]) Get(key K) (V, bool) {
	h, index := s.index(key)
	shard := s.shards[index]
	return s.getFromShard(key, h, shard)
}

func (s *Store[K, V]) GetWithSecodary(key K) (value V, ok bool, err error) {
	h, index := s.index(key)
	shard := s.shards[index]
	value, ok = s.getFromShard(key, h, shard)
	if ok {
		return value, true, nil
	}

	value, err, _ = shard.vgroup.Do(key, func() (v V, err error) {
		v, cost, expire, ok, err := s.secondaryCache.Get(key)
		if err != nil {
			return v, err
		}
		if !ok {
			return v, &NotFound{}
		}
		// insert to cache
		_, _, _ = s.setInternal(key, v, cost, expire, true)
		return v, err
	})
	var notFound *NotFound
	if errors.As(err, &notFound) {
		return value, false, nil
	}
	if err != nil {
		return value, false, err
	}
	return value, true, nil
}

func (s *Store[K, V]) setEntry(shard *Shard[K, V], cost int64, entry *Entry[K, V]) {
	shard.set(entry.key, entry)
	// cost larger than deque size, send to policy directly
	if cost > int64(shard.qsize) {
		shard.mu.Unlock()
		s.writebuf <- WriteBufItem[K, V]{entry: entry, code: NEW}
		return
	}
	entry.deque = true
	shard.deque.PushFront(entry)
	shard.qlen += int(cost)
	s.processDeque(shard)
}

func (s *Store[K, V]) setInternal(key K, value V, cost int64, expire int64, nvmClean bool) (*Shard[K, V], *Entry[K, V], bool) {
	h, index := s.index(key)
	shard := s.shards[index]
	shard.mu.Lock()
	exist, ok := shard.get(key)
	if ok {
		var reschedule bool
		var costChange int64
		exist.value = value
		oldCost := exist.cost.Swap(cost)
		if oldCost != cost {
			costChange = cost - oldCost
			if exist.deque {
				shard.qlen += int(costChange)
			}
		}
		shard.mu.Unlock()
		if expire > 0 {
			old := exist.expire.Swap(expire)
			if old != expire {
				reschedule = true
			}
		}
		if reschedule || costChange != 0 {
			s.writebuf <- WriteBufItem[K, V]{
				entry: exist, code: UPDATE, costChange: costChange, rechedule: reschedule,
			}
		}
		return shard, exist, true
	}
	if s.doorkeeper {
		if shard.counter > uint(shard.dookeeper.Capacity) {
			shard.dookeeper.Reset()
			shard.counter = 0
		}
		hit := shard.dookeeper.Insert(h)
		if !hit {
			shard.counter += 1
			shard.mu.Unlock()
			return shard, nil, false
		}
	}
	entry := s.entryPool.Get().(*Entry[K, V])
	entry.frequency.Store(-1)
	entry.key = key
	entry.value = value
	entry.expire.Store(expire)
	entry.cost.Store(cost)
	entry.nvmClean = nvmClean
	s.setEntry(shard, cost, entry)
	return shard, entry, true

}

func (s *Store[K, V]) Set(key K, value V, cost int64, ttl time.Duration) bool {
	if cost == 0 {
		cost = s.cost(value)
	}
	if cost > int64(s.cap) {
		return false
	}
	var expire int64
	if ttl != 0 {
		expire = s.timerwheel.clock.ExpireNano(ttl)
	}
	_, _, ok := s.setInternal(key, value, cost, expire, false)
	return ok
}

type dequeKV[K comparable, V any] struct {
	k K
	v V
}

func (s *Store[K, V]) processDeque(shard *Shard[K, V]) {
	if shard.qlen <= int(shard.qsize) {
		shard.mu.Unlock()
		return
	}
	// send to slru
	send := make([]*Entry[K, V], 0, 2)
	// removed because frequency < slru tail frequency
	removedkv := make([]dequeKV[K, V], 0, 2)
	// expired
	expiredkv := make([]dequeKV[K, V], 0, 2)
	for shard.qlen > int(shard.qsize) {
		evicted := shard.deque.PopBack()
		evicted.deque = false
		expire := evicted.expire.Load()
		shard.qlen -= int(evicted.cost.Load())
		if expire != 0 && expire <= s.timerwheel.clock.NowNano() {
			deleted := shard.delete(evicted)
			// double check because entry maybe removed already by Delete API
			if deleted {
				expiredkv = append(expiredkv, s.kvBuilder(evicted))
				s.postDelete(evicted)
			}
		} else {
			count := evicted.frequency.Load()
			threshold := s.policy.threshold.Load()
			if count == -1 {
				send = append(send, evicted)
			} else {
				if int32(count) >= threshold {
					send = append(send, evicted)
				} else {
					deleted := shard.delete(evicted)
					// double check because entry maybe removed already by Delete API
					if deleted {
						removedkv = append(removedkv, s.kvBuilder(evicted))
						s.postDelete(evicted)
					}
				}
			}
		}
	}
	shard.mu.Unlock()
	for _, entry := range send {
		s.writebuf <- WriteBufItem[K, V]{entry: entry, code: NEW}
	}
	if s.removalListener != nil {
		for _, e := range removedkv {
			_ = s.removalCallback(e, EVICTED)
		}
		for _, e := range expiredkv {
			_ = s.removalCallback(e, EXPIRED)
		}
	}
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

func (s *Store[K, V]) DeleteWithSecondary(key K) error {
	_, index := s.index(key)
	shard := s.shards[index]
	shard.mu.Lock()
	entry, ok := shard.get(key)
	if ok {
		shard.delete(entry)
		if s.secondaryCache != nil {
			err := s.secondaryCache.Delete(key)
			if err != nil {
				shard.mu.Unlock()
				return err
			}
		}
	}
	shard.mu.Unlock()
	if ok {
		s.writebuf <- WriteBufItem[K, V]{entry: entry, code: REMOVE}
	}
	return nil
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

func (s *Store[K, V]) postDelete(entry *Entry[K, V]) {
	var zero V
	entry.value = zero
	s.entryPool.Put(entry)
}

// remove entry from cache/policy/timingwheel and add back to pool
func (s *Store[K, V]) removeEntry(entry *Entry[K, V], reason RemoveReason) {
	if prev := entry.meta.prev; prev != nil {
		s.policy.Remove(entry)
	}
	if entry.meta.wheelPrev != nil {
		s.timerwheel.deschedule(entry)
	}
	switch reason {
	case EVICTED, EXPIRED:
		_, index := s.index(entry.key)
		shard := s.shards[index]
		if reason == EVICTED && !entry.nvmClean && s.secondaryCache != nil {
			var rn float32 = 1
			if s.probability < 1 {
				rn = s.rg.Float32()
			}

			if rn <= s.probability {
				s.secondaryCacheBuf <- SecondaryCacheItem[K, V]{
					entry:  entry,
					reason: reason,
					shard:  shard,
				}
				return
			}
		}
		shard.mu.Lock()
		deleted := shard.delete(entry)
		shard.mu.Unlock()
		if deleted {
			k, v := entry.key, entry.value
			if s.removalListener != nil {
				s.removalListener(k, v, reason)
			}
			s.postDelete(entry)
		}
	// already removed from shard map
	case REMOVED:
		_, index := s.index(entry.key)
		shard := s.shards[index]
		shard.mu.RLock()
		kv := s.kvBuilder(entry)
		shard.mu.RUnlock()
		_ = s.removalCallback(kv, reason)
	}
}

func (s *Store[K, V]) drainRead() {
	s.policy.total.Add(MAX_READ_BUFF_SIZE)
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
			s.policy.UpdateThreshold()
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
			if entry.removed {
				s.mlock.Unlock()
				continue
			}
			if entry.expire.Load() != 0 {
				s.timerwheel.schedule(entry)
			}
			evicted := s.policy.Set(entry)
			if evicted != nil {
				s.removeEntry(evicted, EVICTED)
				s.tailUpdate = true
			}
			removed := s.policy.EvictEntries()
			for _, e := range removed {
				s.tailUpdate = true
				s.removeEntry(e, EVICTED)
			}
		case REMOVE:
			entry.removed = true
			s.removeEntry(entry, REMOVED)
			s.policy.threshold.Store(-1)
		case UPDATE:
			if item.rechedule {
				s.timerwheel.schedule(entry)
			}
			if item.costChange != 0 {
				s.policy.UpdateCost(entry, item.costChange)
				removed := s.policy.EvictEntries()
				for _, e := range removed {
					s.tailUpdate = true
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

func (s *Store[K, V]) Range(f func(key K, value V) bool) {
	now := s.timerwheel.clock.NowNano()
	for _, shard := range s.shards {
		shard.mu.RLock()
		for _, entry := range shard.hashmap {
			expire := entry.expire.Load()
			if expire != 0 && expire <= now {
				continue
			}
			if !f(entry.key, entry.value) {
				shard.mu.RUnlock()
				return
			}
		}
		shard.mu.RUnlock()
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

type StoreMeta struct {
	Version   uint64
	StartNano int64
	Sketch    *CountMinSketch
}

func (m *StoreMeta) Persist(writer io.Writer, blockEncoder *gob.Encoder) error {
	buffer := bytes.NewBuffer(make([]byte, 0, BlockBufferSize))
	block := NewBlock[*StoreMeta](1, buffer, blockEncoder)
	_, err := block.Write(m)
	if err != nil {
		return err
	}
	err = block.Save()
	if err != nil {
		return err
	}
	return nil
}

func persistDeque[K comparable, V any](dq *deque.Deque[*Entry[K, V]], writer io.Writer, blockEncoder *gob.Encoder) error {
	buffer := bytes.NewBuffer(make([]byte, 0, BlockBufferSize))
	block := NewBlock[*Pentry[K, V]](4, buffer, blockEncoder)
	for dq.Len() > 0 {
		e := dq.PopBack().pentry()
		full, err := block.Write(e)
		if err != nil {
			return err
		}
		if full {
			buffer.Reset()
			block = NewBlock[*Pentry[K, V]](4, buffer, blockEncoder)
		}
	}
	err := block.Save()
	if err != nil {
		return err
	}
	buffer.Reset()
	return nil
}

func (s *Store[K, V]) Persist(version uint64, writer io.Writer) error {
	blockEncoder := gob.NewEncoder(writer)
	s.mlock.Lock()
	meta := &StoreMeta{
		Version:   version,
		StartNano: s.timerwheel.clock.Start.UnixNano(),
		Sketch:    s.policy.sketch,
	}
	err := meta.Persist(writer, blockEncoder)
	if err != nil {
		return err
	}
	err = s.policy.slru.protected.Persist(writer, blockEncoder, 2)
	if err != nil {
		return err
	}
	err = s.policy.slru.probation.Persist(writer, blockEncoder, 3)
	if err != nil {
		return err
	}
	s.mlock.Unlock()

	for _, sd := range s.shards {
		sd.mu.RLock()
		err = persistDeque(sd.deque, writer, blockEncoder)
		if err != nil {
			return err
		}
		sd.mu.RUnlock()
	}

	// write end block
	block := NewBlock[int](255, bytes.NewBuffer(make([]byte, 0)), blockEncoder)
	_, err = block.Write(1)
	if err != nil {
		return err
	}
	return block.Save()
}

func (s *Store[K, V]) insertSimple(entry *Entry[K, V]) {
	_, index := s.index(entry.key)
	s.shards[index].set(entry.key, entry)
	if entry.expire.Load() != 0 {
		s.timerwheel.schedule(entry)
	}
}

func (s *Store[K, V]) processSecondary() {
	for item := range s.secondaryCacheBuf {
		item.shard.mu.RLock()
		// first double check key still exists in map,
		// not exist means key already deleted by Delete API
		_, exist := item.shard.get(item.entry.key)
		if exist {
			err := s.secondaryCache.Set(
				item.entry.key, item.entry.value,
				item.entry.cost.Load(), item.entry.expire.Load(),
			)
			item.shard.mu.RUnlock()
			if err != nil {
				s.secondaryCache.HandleAsyncError(err)
				continue
			}
			if item.reason == EVICTED {
				item.shard.mu.Lock()
				deleted := item.shard.delete(item.entry)
				if deleted {
					s.postDelete(item.entry)
				}
				item.shard.mu.Unlock()
			}
		} else {
			item.shard.mu.RUnlock()
		}
	}
}

func (s *Store[K, V]) Recover(version uint64, reader io.Reader) error {
	blockDecoder := gob.NewDecoder(reader)
	block := &DataBlock[any]{}
	s.mlock.Lock()
	defer s.mlock.Unlock()
	for {
		// reset block first
		block.Data = nil
		block.Type = 0
		block.CheckSum = 0

		err := blockDecoder.Decode(block)
		if err != nil {
			return err
		}
		if block.CheckSum != xxh3.Hash(block.Data) {
			return errors.New("checksum mismatch")
		}

		reader := bytes.NewReader(block.Data)
		if err != nil {
			return err
		}
		if block.Type == 255 {
			break
		}
		switch block.Type {
		case 1:
			metaDecoder := gob.NewDecoder(reader)
			m := &StoreMeta{}
			err = metaDecoder.Decode(m)
			if err != nil {
				return err
			}
			if m.Version != version {
				return VersionMismatch
			}
			s.policy.sketch = m.Sketch
			s.timerwheel.clock.SetStart(m.StartNano)
		case 2:
			entryDecoder := gob.NewDecoder(reader)
			for {
				pentry := &Pentry[K, V]{}
				err := entryDecoder.Decode(pentry)
				if err == io.EOF {
					break
				}
				if err != nil {
					return err
				}
				expire := pentry.Expire
				if expire != 0 && expire < s.timerwheel.clock.NowNano() {
					continue
				}
				l := s.policy.slru.protected
				if l.len < int(l.capacity) {
					entry := pentry.entry()
					l.PushBack(entry)
					s.insertSimple(entry)
				}
			}
		case 3:
			entryDecoder := gob.NewDecoder(reader)
			for {
				pentry := &Pentry[K, V]{}
				err := entryDecoder.Decode(pentry)
				if err == io.EOF {
					break
				}
				if err != nil {
					return err
				}
				expire := pentry.Expire
				if expire != 0 && expire < s.timerwheel.clock.NowNano() {
					continue
				}
				l1 := s.policy.slru.protected
				l2 := s.policy.slru.probation
				if l1.len+l2.len < int(s.policy.slru.maxsize) {
					entry := pentry.entry()
					l2.PushBack(entry)
					s.insertSimple(entry)
				}
			}
		case 4:
			entryDecoder := gob.NewDecoder(reader)
			for {
				pentry := &Pentry[K, V]{}
				err := entryDecoder.Decode(pentry)
				if err == io.EOF {
					break
				}
				if err != nil {
					return err
				}
				expire := pentry.Expire
				if expire != 0 && expire < s.timerwheel.clock.NowNano() {
					continue
				}
				entry := pentry.entry()
				_, index := s.index(entry.key)
				shard := s.shards[index]
				shard.mu.Lock()
				s.setEntry(shard, pentry.Cost, entry)
			}
		}
	}
	return nil
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
	h, index := s.index(key)
	shard := s.shards[index]
	v, ok := s.getFromShard(key, h, shard)
	if !ok {
		loaded, err, _ := shard.group.Do(key, func() (Loaded[V], error) {
			// first try get from secondary cache
			if s.secondaryCache != nil {
				vs, cost, expire, ok, err := s.secondaryCache.Get(key)
				var notFound *NotFound
				if err != nil && !errors.As(err, &notFound) {
					return Loaded[V]{}, err
				}
				if ok {
					_, _, _ = s.setInternal(key, vs, cost, expire, true)
					return Loaded[V]{Value: vs}, nil
				}
			}
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

type NotFound struct{}

func (e *NotFound) Error() string {
	return "not found"
}
