package internal

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"runtime"
	"sync"
	"time"

	"github.com/zeebo/xxh3"

	"github.com/Yiling-J/theine-go/internal/bf"
	"github.com/Yiling-J/theine-go/internal/xruntime"
)

type RemoveReason uint8

const (
	REMOVED RemoveReason = iota
	EVICTED
	EXPIRED
)

var (
	VersionMismatch    = errors.New("version mismatch")
	RoundedParallelism int
	ShardCount         int
	StripedBufferSize  int
	WriteChanSize      int
	WriteBufferSize    int
)

func init() {
	parallelism := xruntime.Parallelism()
	RoundedParallelism = int(RoundUpPowerOf2(parallelism))
	ShardCount = 4 * RoundedParallelism
	StripedBufferSize = 4 * RoundedParallelism
	WriteChanSize = 64 * RoundedParallelism
	WriteBufferSize = 128
}

type Shard[K comparable, V any] struct {
	hashmap   map[K]*Entry[K, V]
	dookeeper *bf.Bloomfilter
	group     *Group[K, Loaded[V]]
	vgroup    *Group[K, V] // used in secondary cache
	counter   uint
	mu        *RBMutex
}

func NewShard[K comparable, V any](doorkeeper bool) *Shard[K, V] {
	s := &Shard[K, V]{
		hashmap: make(map[K]*Entry[K, V]),
		group:   NewGroup[K, Loaded[V]](),
		vgroup:  NewGroup[K, V](),
		mu:      NewRBMutex(),
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

type Store[K comparable, V any] struct {
	entryPool         *sync.Pool
	writeChan         chan WriteBufItem[K, V]
	writeBuffer       []WriteBufItem[K, V]
	hasher            *Hasher[K]
	removalListener   func(key K, value V, reason RemoveReason)
	removalCallback   func(kv dequeKV[K, V], reason RemoveReason) error
	kvBuilder         func(entry *Entry[K, V]) dequeKV[K, V]
	policy            *TinyLfu[K, V]
	timerwheel        *TimerWheel[K, V]
	stripedBuffer     []*Buffer[K, V]
	mask              uint32
	cost              func(V) int64
	shards            []*Shard[K, V]
	cap               uint
	shardCount        uint
	policyMu          sync.Mutex
	doorkeeper        bool
	closed            bool
	secondaryCache    SecondaryCache[K, V]
	secondaryCacheBuf chan SecondaryCacheItem[K, V]
	probability       float32
	rg                *rand.Rand
	ctx               context.Context
	cancel            context.CancelFunc
	maintenanceTicker *time.Ticker
	waitChan          chan bool
}

// New returns a new data struct with the specified capacity
func NewStore[K comparable, V any](
	maxsize int64, doorkeeper bool, entryPool bool, listener func(key K, value V, reason RemoveReason),
	cost func(v V) int64, secondaryCache SecondaryCache[K, V], workers int, probability float32, stringKeyFunc func(k K) string,
) *Store[K, V] {
	hasher := NewHasher(stringKeyFunc)
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

	costfn := func(v V) int64 { return 1 }
	if cost != nil {
		costfn = cost
	}

	stripedBuffer := make([]*Buffer[K, V], 0, StripedBufferSize)
	for i := 0; i < StripedBufferSize; i++ {
		stripedBuffer = append(stripedBuffer, NewBuffer[K, V]())
	}

	s := &Store[K, V]{
		cap:           uint(maxsize),
		hasher:        hasher,
		policy:        NewTinyLfu[K, V](uint(maxsize), hasher),
		stripedBuffer: stripedBuffer,
		mask:          uint32(StripedBufferSize - 1),
		writeChan:     make(chan WriteBufItem[K, V], WriteChanSize),
		writeBuffer:   make([]WriteBufItem[K, V], 0, WriteBufferSize),
		shardCount:    uint(shardCount),
		doorkeeper:    doorkeeper,
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
		waitChan:        make(chan bool),
	}
	if entryPool {
		s.entryPool = &sync.Pool{New: func() any { return &Entry[K, V]{} }}
	}

	s.removalCallback = func(kv dequeKV[K, V], reason RemoveReason) error {
		if s.removalListener != nil {
			s.removalListener(kv.k, kv.v, reason)
		}
		return nil
	}

	s.shards = make([]*Shard[K, V], 0, s.shardCount)
	for i := 0; i < int(s.shardCount); i++ {
		s.shards = append(s.shards, NewShard[K, V](doorkeeper))
	}
	s.policy.removeCallback = func(entry *Entry[K, V]) {
		s.removeEntry(entry, EVICTED)
	}

	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.timerwheel = NewTimerWheel[K, V](uint(maxsize))
	s.timerwheel.clock.RefreshNowCache()

	go s.maintenance()
	if s.secondaryCache != nil {
		s.secondaryCacheBuf = make(chan SecondaryCacheItem[K, V], 256)
		for i := 0; i < workers; i++ {
			go s.processSecondary()
		}
		s.rg = rand.New(rand.New(rand.NewSource(0)))
	}
	return s
}

func (s *Store[K, V]) getFromShard(key K, hash uint64, shard *Shard[K, V]) (V, bool) {
	tk := shard.mu.RLock()
	entry, ok := shard.get(key)
	var value V
	if ok {
		expire := entry.expire.Load()
		var expired bool
		if expire != 0 {
			// Cached now is refreshed every second by ticker.
			// However, since tickers aren't guaranteed to be precise
			// https://github.com/golang/go/issues/45632
			// relax this to 30 seconds. If the entry's expiration time is
			// less than 30 seconds compared to the cached now,
			// refetch the accurate now and compare again.
			nowCached := s.timerwheel.clock.NowNanoCached()
			if expire-nowCached <= 0 {
				expired = true
			} else if expire-nowCached < 30*1e9 {
				now := s.timerwheel.clock.NowNano()
				expired = (expire-now <= 0)
			} else {
				expired = false
			}
		}

		if expired {
			ok = false
		} else {
			value = entry.value
		}
	}
	shard.mu.RUnlock(tk)

	if ok {
		s.policy.hits.Add(1)
	} else {
		s.policy.misses.Add(1)
		return value, false
	}

	idx := s.getReadBufferIdx()
	var send ReadBufItem[K, V]
	send.hash = hash
	if ok {
		send.entry = entry
	}

	pb := s.stripedBuffer[idx].Add(send)
	if pb != nil {
		s.drainRead(pb.Returned)
		s.stripedBuffer[idx].Free()
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
		// load and store should be atomic
		shard.mu.Lock()
		v, cost, expire, ok, err := s.secondaryCache.Get(key)
		if err != nil {
			shard.mu.Unlock()
			return v, err
		}
		if !ok {
			shard.mu.Unlock()
			return v, &NotFound{}
		}
		if expire <= s.timerwheel.clock.NowNano() {
			err = s.secondaryCache.Delete(key)
			if err == nil {
				err = &NotFound{}
			}
			shard.mu.Unlock()
			return v, err
		}

		// insert to cache
		_, _, _ = s.setShard(shard, h, key, v, cost, expire, true)
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

func (s *Store[K, V]) setEntry(hash uint64, shard *Shard[K, V], cost int64, entry *Entry[K, V], fromNVM bool) {
	shard.set(entry.key, entry)
	shard.mu.Unlock()
	s.writeChan <- WriteBufItem[K, V]{
		code: NEW, entry: entry, hash: hash, fromNVM: fromNVM, costChange: cost,
	}

}

func (s *Store[K, V]) setShard(shard *Shard[K, V], hash uint64, key K, value V, cost int64, expire int64, nvmClean bool) (*Shard[K, V], *Entry[K, V], bool) {
	exist, ok := shard.get(key)

	if ok {
		exist.value = value
		old := exist.weight.Swap(cost)

		// create/update events order might change due to race,
		// send cost change in event and apply them to entry policy weight
		// so different order still works.
		costChange := cost - old
		shard.mu.Unlock()

		var reschedule bool
		if expire > 0 {
			old := exist.expire.Swap(expire)
			if old != expire {
				reschedule = true
			}
		}

		s.writeChan <- WriteBufItem[K, V]{
			entry: exist, code: UPDATE, costChange: costChange, rechedule: reschedule,
			hash: hash,
		}
		return shard, exist, true
	}

	if s.doorkeeper {
		if shard.counter > uint(shard.dookeeper.Capacity) {
			shard.dookeeper.Reset()
			shard.counter = 0
		}
		hit := shard.dookeeper.Insert(hash)
		if !hit {
			shard.counter += 1
			shard.mu.Unlock()
			return shard, nil, false
		}
	}
	var entry *Entry[K, V]

	if s.entryPool != nil {
		entry = s.entryPool.Get().(*Entry[K, V])
		if entry.key == key {
			// put back and create an entry manually
			// because same key reuse might cause race condition
			s.entryPool.Put(entry)
			entry = &Entry[K, V]{}
		}
	} else {
		entry = &Entry[K, V]{}
	}

	entry.key = key
	entry.value = value
	entry.expire.Store(expire)
	entry.weight.Store(cost)
	entry.policyWeight = 0
	entry.flag = Flag{}
	s.setEntry(hash, shard, cost, entry, nvmClean)
	return shard, entry, true

}

func (s *Store[K, V]) setInternal(key K, value V, cost int64, expire int64, nvmClean bool) (*Shard[K, V], *Entry[K, V], bool) {
	h, index := s.index(key)
	shard := s.shards[index]
	shard.mu.Lock()

	return s.setShard(shard, h, key, value, cost, expire, nvmClean)

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

func (s *Store[K, V]) Delete(key K) {
	h, index := s.index(key)
	shard := s.shards[index]
	shard.mu.Lock()
	entry, ok := shard.get(key)
	if ok {
		shard.delete(entry)
	}
	shard.mu.Unlock()
	if ok {
		s.writeChan <- WriteBufItem[K, V]{entry: entry, code: REMOVE, hash: h}
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
		s.writeChan <- WriteBufItem[K, V]{entry: entry, code: REMOVE}
	}
	return nil
}

func (s *Store[K, V]) Len() int {
	total := 0
	for _, s := range s.shards {
		tk := s.mu.RLock()
		total += s.len()
		s.mu.RUnlock(tk)
	}
	return total
}

func (s *Store[K, V]) EstimatedSize() int {
	s.policyMu.Lock()
	total := s.policy.slru.protected.Len() + s.policy.slru.probation.Len()
	s.policyMu.Unlock()
	return total
}

func (s *Store[K, V]) index(key K) (uint64, int) {
	base := s.hasher.hash(key)
	return base, int(base & uint64(s.shardCount-1))
}

func (s *Store[K, V]) postDelete(entry *Entry[K, V]) {
	var zero V
	entry.value = zero
	if s.entryPool != nil {
		s.entryPool.Put(entry)
	}
}

// remove entry from cache/policy/timingwheel and add back to pool
// this method must be used with policy mutex together
func (s *Store[K, V]) removeEntry(entry *Entry[K, V], reason RemoveReason) {
	entry.flag.SetRemoved(true)
	_, index := s.index(entry.key)
	shard := s.shards[index]

	if reason == EXPIRED {
		// entry might updated already
		// update expire filed are protected by shard mutex
		if entry.expire.Load() > s.timerwheel.clock.NowNano() {
			return
		}
	}

	if prev := entry.meta.prev; prev != nil {
		s.policy.Remove(entry, false)
	}
	if entry.meta.wheelPrev != nil {
		s.timerwheel.deschedule(entry)
	}

	switch reason {
	case EVICTED, EXPIRED:
		if reason == EVICTED && !entry.flag.IsFromNVM() && s.secondaryCache != nil {
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
		entry.flag.SetDeleted(true)
		kv := s.kvBuilder(entry)
		_ = s.removalCallback(kv, reason)
	}
}

func (s *Store[K, V]) drainRead(buffer []ReadBufItem[K, V]) {
	s.policyMu.Lock()
	for _, e := range buffer {
		s.policy.Access(e)
	}
	s.policyMu.Unlock()
}

func (s *Store[K, V]) sinkWrite(item WriteBufItem[K, V]) {
	entry := item.entry
	if entry == nil {
		return
	}

	// entry removed by API explicitly will not resue by sync pool,
	// so all events can be ignored except the REMOVE one.
	if entry.flag.IsDeleted() {
		return
	}
	if item.code == REMOVE {
		entry.flag.SetDeleted(true)
	}

	if item.fromNVM {
		entry.flag.SetFromNVM(item.fromNVM)
	}

	// ignore removed entries, except code NEW
	// which will reset removed flag
	if entry.flag.IsRemoved() && item.code != NEW {
		return
	}

	// lock free because store API never read/modify entry metadata
	switch item.code {
	case NEW:
		entry.flag.SetRemoved(false)
		if expire := entry.expire.Load(); expire != 0 {

			if expire <= s.timerwheel.clock.NowNano() {
				s.removeEntry(entry, EXPIRED)
				return
			} else {
				s.timerwheel.schedule(entry)
			}
		}

		s.policy.sketch.Add(item.hash)
		entry.policyWeight = item.costChange
		s.policy.Set(entry)

	case REMOVE:
		s.removeEntry(entry, REMOVED)
	case EVICTE:
		s.removeEntry(entry, EVICTED)
	case UPDATE:
		// update entry policy weight
		entry.policyWeight += item.costChange

		if item.rechedule {
			s.timerwheel.schedule(entry)
		}

		// create/update race
		if entry.meta.prev == nil {
			return
		}

		if item.costChange != 0 {
			// recheck hash if entry pool enabled to avoid race
			if s.entryPool != nil {
				hh := s.hasher.hash(entry.key)
				if hh != item.hash {
					return
				}
			}
			// update policy weight
			s.policy.UpdateCost(entry, item.costChange)
		}
	}
	item.entry = nil
}

func (s *Store[K, V]) drainWrite() {

	var wait bool
	for _, item := range s.writeBuffer {
		if item.code == WAIT {
			wait = true
			continue
		}
		s.sinkWrite(item)
	}

	s.writeBuffer = s.writeBuffer[:0]
	if wait {
		s.waitChan <- true
	}

}

func (s *Store[K, V]) maintenance() {

	go func() {
		s.policyMu.Lock()
		s.maintenanceTicker = time.NewTicker(time.Second)
		s.policyMu.Unlock()

		for {
			select {
			case <-s.ctx.Done():
				s.maintenanceTicker.Stop()
				return
			case <-s.maintenanceTicker.C:
				s.policyMu.Lock()
				s.timerwheel.clock.RefreshNowCache()
				if s.closed {
					s.policyMu.Unlock()
					return
				}
				s.timerwheel.advance(0, s.removeEntry)
				s.maintenanceTicker.Reset(time.Second)
				s.policyMu.Unlock()
			}
		}
	}()

	// continuously receive the first item from the buffered channel.
	// avoid a busy loop while still processing data in batches.
	for first := range s.writeChan {
		s.writeBuffer = append(s.writeBuffer, first)
	loop:
		for i := 0; i < WriteBufferSize-1; i++ {
			select {
			case item, ok := <-s.writeChan:
				if !ok {
					return
				}
				s.writeBuffer = append(s.writeBuffer, item)
			default:
				break loop
			}
		}

		s.policyMu.Lock()
		s.drainWrite()
		s.policyMu.Unlock()

	}
}

func (s *Store[K, V]) Range(f func(key K, value V) bool) {
	now := s.timerwheel.clock.NowNano()
	for _, shard := range s.shards {
		tk := shard.mu.RLock()
		for _, entry := range shard.hashmap {
			expire := entry.expire.Load()
			if expire != 0 && expire <= now {
				continue
			}
			if !f(entry.key, entry.value) {
				shard.mu.RUnlock(tk)
				return
			}
		}
		shard.mu.RUnlock(tk)
	}
}

// used in test
func (s *Store[K, V]) RangeEntry(f func(entry *Entry[K, V])) {
	for _, shard := range s.shards {
		tk := shard.mu.RLock()
		for _, entry := range shard.hashmap {
			f(entry)
		}
		shard.mu.RUnlock(tk)
	}
}

func (s *Store[K, V]) Stats() Stats {
	return newStats(s.policy.hits.Value(), s.policy.misses.Value())
}

func (s *Store[K, V]) Close() {

	for _, s := range s.shards {
		tk := s.mu.RLock()
		s.hashmap = nil
		s.mu.RUnlock(tk)
	}
	s.policyMu.Lock()
	s.closed = true
	s.cancel()
	s.policyMu.Unlock()
	close(s.writeChan)
}

func (s *Store[K, V]) getReadBufferIdx() int {
	return int(xruntime.Fastrand() & s.mask)
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

func (s *Store[K, V]) Persist(version uint64, writer io.Writer) error {
	blockEncoder := gob.NewEncoder(writer)
	s.policyMu.Lock()
	meta := &StoreMeta{
		Version:   version,
		StartNano: s.timerwheel.clock.Start.UnixNano(),
		Sketch:    s.policy.sketch,
	}
	err := meta.Persist(writer, blockEncoder)
	if err != nil {
		return err
	}
	err = s.policy.window.Persist(writer, blockEncoder, 2)
	if err != nil {
		return err
	}
	// write protected first, so if cache size changed
	// when restore, protected entries write to new cache first
	err = s.policy.slru.protected.Persist(writer, blockEncoder, 4)
	if err != nil {
		return err
	}
	err = s.policy.slru.probation.Persist(writer, blockEncoder, 3)
	if err != nil {
		return err
	}
	s.policyMu.Unlock()

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
		tk := item.shard.mu.RLock()
		// first double check key still exists in map,
		// not exist means key already deleted by Delete API
		_, exist := item.shard.get(item.entry.key)
		if exist {
			err := s.secondaryCache.Set(
				item.entry.key, item.entry.value,
				item.entry.weight.Load(), item.entry.expire.Load(),
			)
			item.shard.mu.RUnlock(tk)
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
			item.shard.mu.RUnlock(tk)
		}
	}
}

// wait write chan, used in test
func (s *Store[K, V]) Wait() {
	s.writeChan <- WriteBufItem[K, V]{code: WAIT}
	<-s.waitChan
}

func (s *Store[K, V]) Recover(version uint64, reader io.Reader) error {
	blockDecoder := gob.NewDecoder(reader)
	block := &DataBlock[any]{}
	s.policyMu.Lock()
	defer s.policyMu.Unlock()
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
		if block.Type == 255 {
			break
		}
		switch block.Type {
		case 1: // metadata
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
		case 2: // windlw lru
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
				if s.policy.window.Len() < int(s.policy.window.capacity) {
					entry := pentry.entry()
					s.policy.window.PushBack(entry)
					s.insertSimple(entry)
				}
			}
		case 3: // main-probation
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
				if l1.len+l2.len < int64(s.policy.slru.maxsize) {
					entry := pentry.entry()
					l2.PushBack(entry)
					s.insertSimple(entry)
				}
			}
		case 4: // main protected
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
				if l.len < int64(l.capacity) {
					entry := pentry.entry()
					l.PushBack(entry)
					s.insertSimple(entry)
				}
			}

		}
	}
	return nil
}

type debugInfo struct {
	WindowWeight         int64
	WindowWeightField    int64
	WindowCount          int64
	ProbationWeight      int64
	ProbationWeightField int64
	ProbationCount       int64
	ProtectedWeight      int64
	ProtectedWeightField int64
	ProtectedCount       int64
}

func (i debugInfo) String() string {
	final := ""
	final += fmt.Sprintf("total items in window list %d\n", i.WindowCount)
	final += fmt.Sprintf("sum of weight of window list %v\n", i.WindowWeight)
	final += fmt.Sprintf("total items in probation list %d\n", i.ProbationCount)
	final += fmt.Sprintf("sum of wieght of probation list %d\n", i.ProbationWeight)
	final += fmt.Sprintf("total items in protected list %d\n", i.ProtectedCount)
	final += fmt.Sprintf("sum of wieght of protected list %d\n", i.ProtectedWeight)
	final += fmt.Sprintf("total items %d\n", i.WindowCount+i.ProbationCount+i.ProtectedCount)
	return final
}

func (i debugInfo) TotalCount() int64 {
	return i.WindowCount + i.ProbationCount + i.ProtectedCount
}

func (i debugInfo) TotalWeight() int64 {
	tw := i.WindowWeight
	tw += i.ProbationWeight
	tw += i.ProtectedWeight
	return tw
}

// used for test, only
func (s *Store[K, V]) DebugInfo() debugInfo {

	var windowSum int64
	var windowCount int64
	s.policy.window.rangef(func(e *Entry[K, V]) {
		windowSum += e.policyWeight
		windowCount += 1
	})

	var probationSum int64
	var probationCount int64
	s.policy.slru.probation.rangef(func(e *Entry[K, V]) {
		probationSum += e.policyWeight
		probationCount += 1
	})

	var protectedSum int64
	var protectedCount int64
	s.policy.slru.protected.rangef(func(e *Entry[K, V]) {
		protectedCount += 1
		protectedSum += e.policyWeight
	})

	return debugInfo{
		WindowWeight:         windowSum,
		WindowWeightField:    int64(s.policy.window.Len()),
		WindowCount:          windowCount,
		ProbationWeight:      probationSum,
		ProbationWeightField: int64(s.policy.slru.probation.Len()),
		ProbationCount:       probationCount,
		ProtectedWeight:      protectedSum,
		ProtectedWeightField: int64(s.policy.slru.protected.Len()),
		ProtectedCount:       protectedCount,
	}

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
			// load and store should be atomic
			shard.mu.Lock()

			// first try get from secondary cache
			if s.secondaryCache != nil {
				vs, cost, expire, ok, err := s.secondaryCache.Get(key)
				var notFound *NotFound
				if err != nil && !errors.As(err, &notFound) {
					shard.mu.Unlock()
					return Loaded[V]{}, err
				}
				if ok {
					_, _, _ = s.setShard(shard, h, key, vs, cost, expire, true)
					return Loaded[V]{Value: vs}, nil
				}
			}

			loaded, err := s.loader(ctx, key)
			var expire int64
			if loaded.TTL != 0 {
				expire = s.timerwheel.clock.ExpireNano(loaded.TTL)
			}
			if loaded.Cost == 0 {
				loaded.Cost = s.cost(loaded.Value)
			}

			if err == nil {
				s.setShard(shard, h, key, loaded.Value, loaded.Cost, expire, false)
			} else {
				shard.mu.Unlock()
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
