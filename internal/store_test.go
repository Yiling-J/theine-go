package internal

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestStore_WindowExpire(t *testing.T) {
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
	for i := 0; i < 50; i++ {
		store.Set(i, i, 1, 200*time.Millisecond)
	}
	store.Wait()
	mu.Lock()
	require.True(t, len(expired) == 0)
	mu.Unlock()
	time.Sleep(3 * time.Second)
	mu.Lock()
	require.True(t, len(expired) > 0)
	mu.Unlock()
}

func TestStore_Window(t *testing.T) {
	store := NewStore[int, int](1000, false, true, nil, nil, nil, 0, 0, nil)
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

	for i := 0; i < 5; i++ {
		store.Set(i, i, 1, 0)
	}
	// move 0,1,2 entries to slru
	store.Set(123, 123, 8, 0)
	store.Wait()
	require.Equal(t, store.policy.window.Len(), 10)
	keys := []int{}
	for e := store.policy.window.PopTail(); e != nil; e = store.policy.window.PopTail() {
		keys = append(keys, e.key)
	}
	require.Equal(t, []int{3, 4, 123}, keys)
	require.Equal(t, 0, len(evicted))
	store.Wait()

	// test evicted callback
	// fill window with weight 2 items first
	for i := 100; i < 300; i++ {
		store.Set(i, i, 2, 0)
	}
	store.Wait()
	// mark probation full
	store.policy.slru.probation.len.Store(int64(store.policy.slru.probation.capacity))

	// add 15 weight 1 items, window currently has 5 weight2 items.
	// This will send 5 weight2 items and 5 weight1 items to probation, add probation len will increase 15
	// probation current has 3 weight 1 items: 0,1,2, they will be evicted,
	// then continue evict 6 weight2 items, which weigh sum to 12. Total 12+3 = 15
	for i := 300; i < 315; i++ {
		store.Set(i, i, 1, 0)
	}
	store.Wait()
	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, 9, len(evicted))
}

func TestStore_WindowEvict(t *testing.T) {
	store := NewStore[int, int](1000, false, true, nil, nil, nil, 0, 0, nil)
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

	for i := 0; i < 5; i++ {
		store.Set(i, i, 1, 0)
	}
	// move 0,1,2 entries to slru
	store.Set(123, 123, 8, 0)
	store.Wait()
	require.Equal(t, store.policy.window.Len(), 10)
	keys := []int{}
	for e := store.policy.window.PopTail(); e != nil; e = store.policy.window.PopTail() {
		keys = append(keys, e.key)
	}
	require.Equal(t, []int{3, 4, 123}, keys)
	require.Equal(t, 0, len(evicted))
	store.Wait()

	// test evicted callback
	// first update 3 exist item weight to 2,
	// these items are already in probation
	for i := 0; i < 3; i++ {
		store.Set(i, i, 2, 0)
	}
	store.Wait()
	// mark probation full
	store.policy.slru.probation.len.Store(int64(store.policy.slru.probation.capacity))

	// add 15 weight 1 items, window currently has no item.
	// because we update cost in probation 2, so items
	// evicted from window can't go to main, and 5 items will be evicted
	for i := 300; i < 315; i++ {
		store.Set(i, i, 1, 0)
	}
	store.Wait()
	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, 5, len(evicted))
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

	require.Equal(t, uint64(1600), store.policy.hits.Value(), int(store.policy.hits.Value()))
	require.Equal(t, uint64(1600), store.policy.misses.Value(), int(store.policy.misses.Value()))
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
