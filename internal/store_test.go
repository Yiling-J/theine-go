package internal

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestStore_WindowExpire(t *testing.T) {
	store := NewStore[int, int](&StoreOptions[int, int]{MaxSize: 5000})
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
	store := NewStore[int, int](&StoreOptions[int, int]{MaxSize: 1000})
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
}
func TestStore_WindowEvict(t *testing.T) {
	store := NewStore[int, int](&StoreOptions[int, int]{MaxSize: 1000})
	store.policy.sketch.EnsureCapacity(1000)
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
	require.Equal(t, int(store.policy.window.capacity), 10)

	// test evicted callback
	// fill window with weight 2 items first
	for i := 0; i < 500; i++ {
		store.Set(i, i, 2, 0)
	}
	store.Wait()
	require.Equal(t, 0, len(evicted))

	// add 15 weight 1 items, window currently has 5 weight2 items.
	// This will send 5 weight2 items and 5 weight1 items to probation,
	// all items has freq 1 in cache, which means these 15 entries don't
	// have enough freq to be admitted.
	for i := 700; i < 715; i++ {
		store.Set(i, i, 1, 0)
	}
	store.Wait()
	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, 10, len(evicted))
}

func TestStore_DoorKeeperDynamicSize(t *testing.T) {
	store := NewStore[int, int](&StoreOptions[int, int]{MaxSize: 200000, Doorkeeper: true})
	defer store.Close()
	shard := store.shards[0]
	require.True(t, shard.dookeeper.Capacity == 512)
	for i := 0; i < 5000; i++ {
		shard.set(i, &Entry[int, int]{})
	}
	require.True(t, shard.dookeeper.Capacity > 100000)
}

func TestStore_PolicyCounter(t *testing.T) {
	store := NewStore[int, int](&StoreOptions[int, int]{MaxSize: 1000})
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
	store := NewStore[int, int](&StoreOptions[int, int]{MaxSize: 1000})
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
	store.policyMu.Lock()

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
	store.policyMu.Unlock()

	// ticker refresh cached now
	time.Sleep(1200 * time.Millisecond)
	cachedNow := store.timerwheel.clock.NowNanoCached()
	require.True(t, cachedNow > testNow)
}

func TestStore_SinkWritePolicyWeight(t *testing.T) {
	store := NewStore[int, int](&StoreOptions[int, int]{MaxSize: 10000})
	defer store.Close()

	entry := &Entry[int, int]{key: 1, value: 1}
	h := store.hasher.Hash(1)

	// wright change 5 -> 1 -> 8
	store.sinkWrite(WriteBufItem[int, int]{
		entry:      entry,
		costChange: -4,
		code:       UPDATE,
		hash:       h,
	})

	store.sinkWrite(WriteBufItem[int, int]{
		entry:      entry,
		costChange: 5,
		code:       NEW,
		hash:       h,
	})

	store.sinkWrite(WriteBufItem[int, int]{
		entry:      entry,
		costChange: 7,
		code:       UPDATE,
		hash:       h,
	})

	require.Equal(t, 8, int(store.policy.weightedSize))
}

func TestStore_CloseRace(t *testing.T) {
	store := NewStore[int, int](&StoreOptions[int, int]{MaxSize: 1000})

	var wg sync.WaitGroup
	var closed atomic.Bool
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			counter := i * 5
			countdown := -1
			defer wg.Done()
			for {
				// continue get/set 20 times after cache closed
				if countdown == 0 {
					return
				}
				if closed.Load() && countdown == -1 {
					countdown = 20
				}
				store.Get(counter)
				store.Set(100, 100, 1, 0)
				counter += i
				if countdown > 0 {
					countdown -= 1
				}
			}
		}(i)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		store.Close()
		closed.Store(true)
	}()
	wg.Wait()

	_ = store.Set(100, 100, 1, 0)
	v, ok := store.Get(100)
	require.False(t, ok)
	require.Equal(t, 0, v)
	require.NotNil(t, store.ctx.Err())
}

func TestStore_CloseRaceLoadingCache(t *testing.T) {
	store := NewStore[int, int](&StoreOptions[int, int]{MaxSize: 1000})
	loadingStore := NewLoadingStore(store)
	loadingStore.loader = func(ctx context.Context, key int) (Loaded[int], error) {
		return Loaded[int]{Value: 100, Cost: 1}, nil
	}
	ctx := context.TODO()

	var wg sync.WaitGroup
	var closed atomic.Bool
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			counter := i * 5
			countdown := -1
			defer wg.Done()
			for {
				// continue get/set 20 times after cache closed
				if countdown == 0 {
					return
				}
				if closed.Load() && countdown == -1 {
					countdown = 20
				}
				_, err := loadingStore.Get(ctx, counter)
				if countdown > 0 {
					require.Equal(t, ErrCacheClosed, err)
				}
				counter += i
				if countdown > 0 {
					countdown -= 1
				}
			}
		}(i)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		loadingStore.Close()
		closed.Store(true)
	}()
	wg.Wait()

	_, err := loadingStore.Get(ctx, 100)
	require.Equal(t, ErrCacheClosed, err)
	require.NotNil(t, store.ctx.Err())
}
