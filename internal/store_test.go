package internal

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestStore_QueueExpire(t *testing.T) {
	store := NewStore[int, int](5000, false, true, nil, nil, nil, 0, 0, nil)
	defer store.Close()

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
		entry.weight.Store(1)
		entry.queueIndex.Store(-2)
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

func TestStore_ProcessQueue(t *testing.T) {
	store := NewStore[int, int](20000, false, true, nil, nil, nil, 0, 0, nil)
	defer store.Close()

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
		entry.weight.Store(1)
		entry.queueIndex.Store(-2)
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
	time.Sleep(1 * time.Second)
	store.Wait()

	// test evicted callback, cost less than threshold will be evicted immediately
	store.policy.threshold.Store(100)
	for i := 10; i < 15; i++ {
		entry := &Entry[int, int]{key: i}
		entry.weight.Store(1)
		entry.queueIndex.Store(-2)
		_, index := store.index(i)
		store.shards[index].mu.Lock()
		store.shards[index].hashmap[i] = entry
		store.shards[index].mu.Unlock()

		store.shards[0].mu.Lock()
		store.setEntry(h, store.shards[0], 1, entry, false)
	}

	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, 5, len(evicted))
}

func TestStore_RemoveQueue(t *testing.T) {
	store := NewStore[int, int](20000, false, true, nil, nil, nil, 0, 0, nil)
	defer store.Close()
	h, index := store.index(123)
	qindex := h & uint64(RoundedParallelism-1)
	q := store.queue.qs[qindex]

	shard := store.shards[index]
	store.Set(123, 123, 8, 0)
	entry := shard.hashmap[123]
	// this will send key 123 to policy because deque is full
	q.size = 10
	q.len = 10
	entryNew := &Entry[int, int]{key: 1}
	entryNew.weight.Store(1)
	entryNew.queueIndex.Store(-2)
	store.queue.Push(h, entryNew, 1, false)
	shard.hashmap[1] = entryNew
	// delete key
	store.Delete(123)

	time.Sleep(1 * time.Second)
	store.writeChan <- WriteBufItem[int, int]{}
	time.Sleep(1 * time.Second)

	// because 123 is removed already, it should not be on any LRU list
	store.mlock.Lock()
	require.True(t, entry.flag.IsRemoved())
	require.Nil(t, entry.meta.prev)
	require.Nil(t, entry.meta.next)
	store.mlock.Unlock()
	_, ok := store.Get(123)
	require.False(t, ok)
}

func TestStore_DoorKeeperDynamicSize(t *testing.T) {
	store := NewStore[int, int](200000, true, true, nil, nil, nil, 0, 0, nil)
	defer store.Close()
	shard := store.shards[0]
	require.True(t, shard.dookeeper.Capacity == 512)
	for i := 0; i < 5000; i++ {
		shard.set(i, &Entry[int, int]{})
	}
	require.True(t, shard.dookeeper.Capacity > 100000)
}

func TestStore_PolicyCounter(t *testing.T) {
	store := NewStore[int, int](1000, false, true, nil, nil, nil, 0, 0, nil)
	defer store.Close()
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

func TestStore_GetExpire(t *testing.T) {
	store := NewStore[int, int](1000, false, true, nil, nil, nil, 0, 0, nil)
	defer store.Close()

	_, i := store.index(123)
	fakeNow := store.timerwheel.clock.NowNano() - 100*10e9
	testNow := store.timerwheel.clock.NowNano()
	entry := &Entry[int, int]{
		key:   123,
		value: 123,
	}
	entry.queueIndex.Store(-2)
	entry.expire.Store(fakeNow)

	store.shards[i].hashmap[123] = entry
	store.mlock.Lock()

	// already exprired
	store.timerwheel.clock.SetNowCache(fakeNow + 1)
	_, ok := store.Get(123)
	require.False(t, ok)

	// use cached now, not expire
	store.timerwheel.clock.SetNowCache(fakeNow - 31*10e9)
	v, ok := store.Get(123)
	require.True(t, ok)
	require.Equal(t, 123, v)

	// less than 30 seconds and not expired, use real now
	store.timerwheel.clock.SetNowCache(fakeNow - 1)
	_, ok = store.Get(123)
	require.False(t, ok)
	store.mlock.Unlock()

	// ticker refresh cached now
	time.Sleep(1200 * time.Millisecond)
	cachedNow := store.timerwheel.clock.NowNanoCached()
	require.True(t, cachedNow > testNow)
}
