package hashmap

import (
	"github.com/Yiling-J/theine-go/internal/hasher"
	"github.com/Yiling-J/theine-go/internal/utils"
	"github.com/Yiling-J/theine-go/internal/xruntime"
)

var (
	ShardCount int
)

func init() {
	parallelism := xruntime.Parallelism()
	ShardCount = 8 * int(utils.RoundUpPowerOf2(parallelism))
}

type ConcurrentHashMap[K comparable, V any] interface {
	Set(key K, value V)
	Get(key K) (V, bool)
	Delete(key K) (V, bool)
	Len() int
	Range(f func(key K, value V) bool)
	Clear()
}

type ShardMap[K comparable, V any] struct {
	hasher *hasher.Hasher[K]
	shards []*shard[K, V]
}

func (s *ShardMap[K, V]) Set(key K, value V) {
	h := s.hasher.Hash(key)
	shard := s.shards[int(h&uint64(ShardCount-1))]
	shard.set(key, value)
}

func (s *ShardMap[K, V]) Get(key K) (value V, ok bool) {
	h := s.hasher.Hash(key)
	shard := s.shards[int(h&uint64(ShardCount-1))]
	return shard.get(key)
}

func (s *ShardMap[K, V]) Delete(key K) (V, bool) {
	h := s.hasher.Hash(key)
	shard := s.shards[int(h&uint64(ShardCount-1))]
	return shard.delete(key)
}

func (s *ShardMap[K, V]) Len() int {
	var total int
	for _, s := range s.shards {
		t := s.mu.RLock()
		total += len(s.hashmap)
		s.mu.RUnlock(t)
	}
	return total
}

func (s *ShardMap[K, V]) Clear() {
	for _, s := range s.shards {
		s.mu.Lock()
		s.hashmap = map[K]V{}
		s.mu.Unlock()
	}
}

func (s *ShardMap[K, V]) Range(f func(key K, value V) bool) {
	for _, shard := range s.shards {
		tk := shard.mu.RLock()
		for k, v := range shard.hashmap {
			if !f(k, v) {
				break
			}
		}
		shard.mu.RUnlock(tk)
	}
}

func NewShardMap[K comparable, V any](hasher *hasher.Hasher[K], stringKeyFunc func(k K) string) *ShardMap[K, V] {

	s := &ShardMap[K, V]{hasher: hasher}
	s.shards = make([]*shard[K, V], 0, ShardCount)
	for i := 0; i < int(ShardCount); i++ {
		s.shards = append(s.shards, newShard[K, V]())
	}
	return s
}

type shard[K comparable, V any] struct {
	hashmap map[K]V
	mu      *RBMutex
}

func newShard[K comparable, V any]() *shard[K, V] {
	s := &shard[K, V]{
		hashmap: make(map[K]V),
		mu:      NewRBMutex(),
	}
	return s
}

func (s *shard[K, V]) set(key K, value V) {
	s.mu.Lock()
	s.hashmap[key] = value
	s.mu.Unlock()
}

func (s *shard[K, V]) get(key K) (value V, ok bool) {
	tk := s.mu.RLock()
	value, ok = s.hashmap[key]
	s.mu.RUnlock(tk)
	return
}

func (s *shard[K, V]) delete(key K) (V, bool) {
	var deleted bool
	s.mu.Lock()
	exist, ok := s.hashmap[key]
	if ok {
		delete(s.hashmap, key)
		deleted = true
	}
	s.mu.Unlock()
	return exist, deleted
}
