package internal

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestStore_DequeExpire(t *testing.T) {
	store := NewStore[int, int](5000, false, nil, nil, nil, 0, 0, nil)

	expired := map[int]int{}
	var mu sync.Mutex
	store.removalListener = func(key, value int, reason RemoveReason) {
		if reason == EXPIRED {
			mu.Lock()
			expired[key] = value
			mu.Unlock()
		}
	}
	expire := store.timerwheel.clock.ExpireNano(200 * time.Millisecond)
	for i := 0; i < 50; i++ {
		entry := &Entry[int, int]{key: i}
		entry.expire.Store(expire)
		entry.cost = 1
		store.shards[0].mu.Lock()
		store.setEntry(123, store.shards[0], 1, entry, false)
		_, index := store.index(i)
		store.shards[index].hashmap[i] = entry
	}
	mu.Lock()
	require.True(t, len(expired) == 0)
	mu.Unlock()
	time.Sleep(3 * time.Second)
	mu.Lock()
	require.True(t, len(expired) > 0)
	mu.Unlock()
}

func TestStore_ProcessDeque(t *testing.T) {
	store := NewStore[int, int](20000, false, nil, nil, nil, 0, 0, nil)

	evicted := map[int]int{}
	var mu sync.Mutex
	store.removalListener = func(key, value int, reason RemoveReason) {
		if reason == EVICTED {
			mu.Lock()
			evicted[key] = value
			mu.Unlock()
		}
	}
	h, _ := store.index(123)
	qindex := h & uint64(RoundedParallelism-1)
	for _, q := range store.queue.qs {
		q.size = 10
	}

	for i := 0; i < 5; i++ {
		entry := &Entry[int, int]{key: i}
		entry.cost = 1
		store.shards[0].mu.Lock()
		store.setEntry(h, store.shards[0], 1, entry, false)
		_, index := store.index(i)
		store.shards[index].hashmap[i] = entry
	}

	// move 0,1,2 entries to slru
	store.Set(123, 123, 8, 0)
	require.Equal(t, store.queue.qs[qindex].deque.Len(), 3)
	keys := []int{}
	for store.queue.qs[qindex].deque.Len() != 0 {
		e := store.queue.qs[qindex].deque.PopBack()
		keys = append(keys, e.entry.key)
	}
	require.Equal(t, []int{3, 4, 123}, keys)
	require.Equal(t, 0, len(evicted))

	// test evicted callback, cost less than threshold will be evicted immediately
	store.policy.threshold.Store(100)
	for i := 10; i < 15; i++ {
		entry := &Entry[int, int]{key: i}
		entry.cost = 1

		store.shards[0].mu.Lock()
		store.setEntry(h, store.shards[0], 1, entry, false)
		_, index := store.index(i)
		store.shards[index].mu.Lock()
		store.shards[index].hashmap[i] = entry
		store.shards[index].mu.Unlock()
	}
	time.Sleep(1 * time.Second)
	mu.Lock()
	require.Equal(t, 5, len(evicted))
	mu.Unlock()
}

func TestStore_RemoveDeque(t *testing.T) {
	store := NewStore[int, int](20000, false, nil, nil, nil, 0, 0, nil)
	h, index := store.index(123)
	qindex := h & uint64(RoundedParallelism-1)
	q := store.queue.qs[qindex]

	shard := store.shards[index]
	store.Set(123, 123, 8, 0)
	entry := shard.hashmap[123]
	store.Delete(123)
	// this will send key 123 to policy because deque is full
	q.size = 10
	q.len = 10
	entryNew := &Entry[int, int]{key: 1}
	entryNew.cost = 1
	store.queue.Push(h, entryNew, 1, false)
	shard.hashmap[1] = entryNew

	time.Sleep(1 * time.Second)
	store.writeChan <- WriteBufItem[int, int]{}
	time.Sleep(1 * time.Second)

	// because 123 is removed already, it should not be on any LRU list
	shard.mu.Lock()
	require.True(t, entry.flag.IsRemoved())
	require.Nil(t, entry.meta.prev)
	require.Nil(t, entry.meta.next)
	shard.mu.Unlock()
	_, ok := store.Get(123)
	require.False(t, ok)
}

func TestStore_DoorKeeperDynamicSize(t *testing.T) {
	store := NewStore[int, int](200000, true, nil, nil, nil, 0, 0, nil)
	shard := store.shards[0]
	require.True(t, shard.dookeeper.Capacity == 512)
	for i := 0; i < 5000; i++ {
		shard.set(i, &Entry[int, int]{})
	}
	require.True(t, shard.dookeeper.Capacity > 100000)
}

func TestStore_PolicyCounter(t *testing.T) {
	store := NewStore[int, int](1000, false, nil, nil, nil, 0, 0, nil)
	for i := 0; i < 1000; i++ {
		store.Set(i, i, 1, 0)
	}
	// hit
	for i := 0; i < 1600; i++ {
		store.Get(100)
	}
	// miss
	for i := 0; i < 1600; i++ {
		store.Get(10000)
	}

	require.Equal(t, uint64(1600), store.policy.hits.Value())
	require.Equal(t, uint64(1600), store.policy.misses.Value())
}
