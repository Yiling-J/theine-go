package internal

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDequeExpire(t *testing.T) {
	store := NewStore[int, int](20000, false, nil, nil, nil, 0, 0, nil)

	expired := map[int]int{}
	store.removalListener = func(key, value int, reason RemoveReason) {
		if reason == EXPIRED {
			expired[key] = value
		}
	}
	_, index := store.index(123)
	expire := store.timerwheel.clock.ExpireNano(200 * time.Millisecond)
	for i := 0; i < 50; i++ {
		entry := &Entry[int, int]{key: i}
		entry.expire.Store(expire)
		entry.cost.Store(1)
		store.shards[index].deque.PushFront(entry)
		store.shards[index].qlen += 1
		store.shards[index].hashmap[i] = entry
	}
	require.True(t, len(expired) == 0)
	time.Sleep(1 * time.Second)
	store.Set(123, 123, 1, 1*time.Second)
	require.True(t, len(expired) > 0)
}

func TestProcessDeque(t *testing.T) {
	store := NewStore[int, int](20000, false, nil, nil, nil, 0, 0, nil)

	evicted := map[int]int{}
	store.removalListener = func(key, value int, reason RemoveReason) {
		if reason == EVICTED {
			evicted[key] = value
		}
	}
	_, index := store.index(123)
	shard := store.shards[index]
	shard.qsize = 10

	for i := 0; i < 5; i++ {
		entry := &Entry[int, int]{key: i}
		entry.cost.Store(1)
		store.shards[index].deque.PushFront(entry)
		store.shards[index].qlen += 1
		store.shards[index].hashmap[i] = entry
	}

	// move 0,1,2 entries to slru
	store.Set(123, 123, 8, 0)
	require.Equal(t, store.shards[index].deque.Len(), 3)
	keys := []int{}
	for store.shards[index].deque.Len() != 0 {
		e := store.shards[index].deque.PopBack()
		keys = append(keys, e.key)
	}
	require.Equal(t, []int{3, 4, 123}, keys)
	require.Equal(t, 0, len(evicted))

	// test evicted callback, cost less than threshold will be evicted immediately
	store.policy.threshold.Store(100)
	for i := 10; i < 15; i++ {
		entry := &Entry[int, int]{key: i}
		entry.cost.Store(1)
		store.shards[index].deque.PushFront(entry)
		store.shards[index].qlen += 1
		store.shards[index].hashmap[i] = entry
	}
	store.shards[index].mu.Lock()
	store.processDeque(store.shards[index])
	require.Equal(t, 5, len(evicted))
}

func TestRemoveDeque(t *testing.T) {
	store := NewStore[int, int](20000, false, nil, nil, nil, 0, 0, nil)
	_, index := store.index(123)
	shard := store.shards[index]
	store.Set(123, 123, 8, 0)
	entry := shard.hashmap[123]
	store.Delete(123)
	// this will send key 123 to policy because deque is full
	shard.qsize = 10
	shard.qlen = 10
	entryNew := &Entry[int, int]{key: 1}
	entryNew.cost.Store(1)
	shard.deque.PushFront(entryNew)
	shard.qlen += 1
	shard.hashmap[1] = entryNew
	// wait policy process
	time.Sleep(time.Second)
	// because 123 is removed already, it should not be on any LRU list
	shard.mu.Lock()
	require.True(t, entry.removed)
	require.Nil(t, entry.meta.prev)
	require.Nil(t, entry.meta.next)
	shard.mu.Unlock()
	_, ok := store.Get(123)
	require.False(t, ok)
}

func TestDoorKeeperDynamicSize(t *testing.T) {
	store := NewStore[int, int](200000, true, nil, nil, nil, 0, 0, nil)
	shard := store.shards[0]
	require.True(t, shard.dookeeper.Capacity == 512)
	for i := 0; i < 5000; i++ {
		shard.set(i, &Entry[int, int]{})
	}
	require.True(t, shard.dookeeper.Capacity > 100000)
}
