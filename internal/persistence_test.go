package internal

import (
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestStorePersistence(t *testing.T) {
	store := NewStore[int, int](1000, false, nil, nil, nil, 0, 0, nil)
	for i := 0; i < 20; i++ {
		_ = store.Set(i, i, 1, 0)
	}
	time.Sleep(200 * time.Millisecond)
	for i := 0; i < 10; i++ {
		_, _ = store.Get(i)
	}

	for _, buf := range store.stripedBuffer {
		store.drainRead(buf.items())
	}
	// now 0-9 in protected and 10-19 in probation
	require.Equal(t, 10, store.policy.slru.protected.Len())
	require.Equal(t, 10, store.policy.slru.probation.Len())
	require.ElementsMatch(t,
		strings.Split("9/8/7/6/5/4/3/2/1/0", "/"),
		strings.Split(store.policy.slru.protected.display(), "/"),
	)
	require.ElementsMatch(t,
		strings.Split("19/18/17/16/15/14/13/12/11/10", "/"),
		strings.Split(store.policy.slru.probation.display(), "/"),
	)
	// add 5 entries to shard deque
	for i := 20; i < 25; i++ {
		entry := &Entry[int, int]{
			key:   i,
			value: i,
		}
		entry.cost.Store(int64(1))
		entry.frequency.Store(int32(i))
		store.shards[0].deque.PushFront(entry)
	}
	// update sketch
	for i := 0; i < 10; i++ {
		_, _ = store.Get(5)
	}
	for _, buf := range store.stripedBuffer {
		store.drainRead(buf.items())
	}
	count := store.policy.sketch.Estimate(store.hasher.hash(5))
	require.True(t, count > 5)

	f, err := os.Create("stest")
	defer os.Remove("stest")
	require.Nil(t, err)
	err = store.Persist(0, f)
	require.Nil(t, err)
	f.Close()

	new := NewStore[int, int](1000, false, nil, nil, nil, 0, 0, nil)
	// manually set deque size of shard
	for _, shard := range new.shards {
		shard.qsize = 10
	}
	f, err = os.Open("stest")
	require.Nil(t, err)
	err = new.Recover(0, f)
	require.Nil(t, err)
	f.Close()
	m := map[int]int{}
	new.Range(func(key, value int) bool {
		m[key] = value
		return true
	})
	require.Equal(t, 25, len(m))
	for k, v := range m {
		require.Equal(t, k, v)
	}
	require.Equal(t, 10, new.policy.slru.protected.Len())
	require.Equal(t, 10, new.policy.slru.probation.Len())

	require.ElementsMatch(t,
		strings.Split("9/8/7/6/5/4/3/2/1/0", "/"),
		strings.Split(store.policy.slru.protected.display(), "/"),
	)
	require.Equal(t, "19/18/17/16/15/14/13/12/11/10", new.policy.slru.probation.display())

	count = new.policy.sketch.Estimate(store.hasher.hash(5))
	require.True(t, count > 5)

}

func TestStorePersistenceTTL(t *testing.T) {
	store := NewStore[int, int](1000, false, nil, nil, nil, 0, 0, nil)
	for i := 0; i < 10; i++ {
		_ = store.Set(i, i, 1, 2*time.Second)
	}
	for i := 10; i < 20; i++ {
		_ = store.Set(i, i, 1, 5*time.Second)
	}
	for i := 20; i < 30; i++ {
		_ = store.Set(i, i, 1, 1*time.Second)
	}
	time.Sleep(200 * time.Millisecond)

	f, err := os.Create("stest")
	defer os.Remove("stest")
	require.Nil(t, err)
	err = store.Persist(0, f)
	require.Nil(t, err)
	f.Close()
	// expire 20-29
	time.Sleep(time.Second)
	new := NewStore[int, int](1000, false, nil, nil, nil, 0, 0, nil)
	f, err = os.Open("stest")
	require.Nil(t, err)
	err = new.Recover(0, f)
	require.Nil(t, err)
	f.Close()
	m := map[int]int{}
	new.Range(func(key, value int) bool {
		m[key] = value
		return true
	})
	require.Equal(t, 20, len(m))
	time.Sleep(2 * time.Second)
	for i := 0; i < 10; i++ {
		_, ok := new.Get(i)
		require.False(t, ok)
	}
	for i := 10; i < 20; i++ {
		_, ok := new.Get(i)
		require.True(t, ok)
	}
	time.Sleep(3 * time.Second)
	for i := 10; i < 20; i++ {
		_, ok := new.Get(i)
		require.False(t, ok)
	}
}

func TestStorePersistenceResize(t *testing.T) {
	store := NewStore[int, int](1000, false, nil, nil, nil, 0, 0, nil)
	for i := 0; i < 1000; i++ {
		_ = store.Set(i, i, 1, 0)
	}
	time.Sleep(200 * time.Millisecond)
	for i := 0; i < 500; i++ {
		_, _ = store.Get(i)
	}
	for _, buf := range store.stripedBuffer {
		store.drainRead(buf.items())
	}
	// now 0-499 in protected and 500-999 in probation
	require.Equal(t, 500, store.policy.slru.protected.Len())
	require.Equal(t, 500, store.policy.slru.probation.Len())

	f, err := os.Create("stest")
	defer os.Remove("stest")
	require.Nil(t, err)
	err = store.Persist(0, f)
	require.Nil(t, err)
	f.Close()

	new := NewStore[int, int](100, false, nil, nil, nil, 0, 0, nil)
	f, err = os.Open("stest")
	require.Nil(t, err)
	err = new.Recover(0, f)
	require.Nil(t, err)
	f.Close()
	// new cache protected size is 80, should contains latest 80 entries of original protected
	require.Equal(t, 80, new.policy.slru.protected.Len())
	// new cache probation size is 20, should contains latest 20 entries of original probation
	require.Equal(t, 20, new.policy.slru.probation.Len())

	for _, i := range strings.Split(new.policy.slru.protected.display(), "/") {
		in, err := strconv.Atoi(i)
		require.Nil(t, err)
		require.True(t, in < 500)
	}

	for _, i := range strings.Split(new.policy.slru.probation.display(), "/") {
		in, err := strconv.Atoi(i)
		require.Nil(t, err)
		require.True(t, in >= 500 && in < 1000)
	}

}
