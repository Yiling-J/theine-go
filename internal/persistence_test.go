package internal

import (
	"math/rand"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestStorePersistence_Simple(t *testing.T) {
	store := NewStore[int, int](1000, false, true, nil, nil, nil, 0, 0, nil)
	for i := 0; i < 20; i++ {
		_ = store.Set(i, i, 1, 0)
	}
	// fill window and move 10-19 to main probation
	for i := 20; i < 30; i++ {
		_ = store.Set(i, i, 1, 0)
	}

	store.Wait()
	for i := 0; i < 10; i++ {
		_, _ = store.Get(i)
	}

	for _, buf := range store.stripedBuffer {
		store.drainRead(buf.items())
	}
	// now 20-29 in window, 0-9 in protected and 10-19 in probation
	require.Equal(t, 10, store.policy.window.Len())
	require.Equal(t, 10, store.policy.slru.protected.Len())
	require.Equal(t, 10, store.policy.slru.probation.Len())
	require.Equal(t, 30, int(store.policy.weightedSize))
	require.ElementsMatch(t,
		strings.Split("9/8/7/6/5/4/3/2/1/0", "/"),
		strings.Split(store.policy.slru.protected.display(), "/"),
	)
	require.ElementsMatch(t,
		strings.Split("19/18/17/16/15/14/13/12/11/10", "/"),
		strings.Split(store.policy.slru.probation.display(), "/"),
	)

	// update sketch
	for i := 0; i < 10; i++ {
		_, _ = store.Get(5)
	}
	for _, buf := range store.stripedBuffer {
		store.drainRead(buf.items())
	}
	count := store.policy.sketch.Estimate(store.hasher.Hash(5))
	require.True(t, count > 5)

	f, err := os.Create("stest")
	defer os.Remove("stest")
	require.Nil(t, err)
	err = store.Persist(0, f)
	require.Nil(t, err)
	f.Close()

	new := NewStore[int, int](1000, false, true, nil, nil, nil, 0, 0, nil)
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
	require.Equal(t, 30, len(m))
	for k, v := range m {
		require.Equal(t, k, v)
	}
	require.Equal(t, 10, new.policy.window.Len())
	require.Equal(t, 10, new.policy.slru.protected.Len())
	require.Equal(t, 10, new.policy.slru.probation.Len())
	require.Equal(t, 30, int(new.policy.weightedSize))

	require.ElementsMatch(t,
		strings.Split("9/8/7/6/5/4/3/2/1/0", "/"),
		strings.Split(store.policy.slru.protected.display(), "/"),
	)
	require.Equal(t, "19/18/17/16/15/14/13/12/11/10", new.policy.slru.probation.display())

	count = new.policy.sketch.Estimate(new.hasher.Hash(5))
	require.True(t, count > 5)

}

func TestStorePersistence_TTL(t *testing.T) {
	store := NewStore[int, int](1000, false, true, nil, nil, nil, 0, 0, nil)
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
	new := NewStore[int, int](1000, false, true, nil, nil, nil, 0, 0, nil)
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

func TestStorePersistence_Resize(t *testing.T) {
	store := NewStore[int, int](1000, false, true, nil, nil, nil, 0, 0, nil)
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
	// now 0-499 in protected and 500-989 in probation, 990-999 in window
	require.Equal(t, 500, store.policy.slru.protected.Len())
	require.Equal(t, 490, store.policy.slru.probation.Len())

	f, err := os.Create("stest")
	defer os.Remove("stest")
	require.Nil(t, err)
	err = store.Persist(0, f)
	require.Nil(t, err)
	f.Close()

	new := NewStore[int, int](100, false, true, nil, nil, nil, 0, 0, nil)
	f, err = os.Open("stest")
	require.Nil(t, err)
	err = new.Recover(0, f)
	require.Nil(t, err)
	f.Close()
	// new cache protected size is 79, should contains latest 80 entries of original protected
	require.Equal(t, 79, new.policy.slru.protected.Len())
	// new cache probation size is 20, should contains latest 20 entries of original probation
	require.Equal(t, 20, new.policy.slru.probation.Len())
	// The original map size is 1000, so the sketch table size is 1024.
	// The new store size is smaller (100), but since the sketch is a memory-efficient data structure,
	// increasing its size should be safe.
	require.Equal(t, 1024, len(new.policy.sketch.Table))

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

type DelayWriter struct {
	f           *os.File
	beforeWrite func()
}

func (dw *DelayWriter) Write(p []byte) (n int, err error) {
	dw.beforeWrite()
	time.Sleep(1 * time.Second)
	return dw.f.Write(p)
}

func TestStorePersistence_Readonly(t *testing.T) {
	store := NewStore[int, int](1000, false, true, nil, nil, nil, 0, 0, nil)
	for i := 0; i < 1000; i++ {
		_ = store.Set(i, i, 1, 0)
	}
	for i := 0; i < 500; i++ {
		_, _ = store.Get(i)
	}
	store.Wait()
	var counter atomic.Uint64
	persistDone := make(chan bool)

	v, ok := store.Get(100)
	require.True(t, ok)
	require.Equal(t, 100, v)

	startCounter := make(chan bool)
	started := false
	go func() {
		done := false
		<-startCounter
		for !done {
			select {
			case <-persistDone:
				done = true
			default:
				shard := store.shards[rand.Intn(len(store.shards))]
				tk := shard.mu.RLock()
				shard.mu.RUnlock(tk)
				counter.Add(1)
			}
		}
	}()

	f, err := os.Create("stest")
	defer os.Remove("stest")
	require.Nil(t, err)
	err = store.Persist(0, &DelayWriter{f: f, beforeWrite: func() {
		if !started {
			close(startCounter)
			started = true
		}
	}})
	require.Nil(t, err)
	f.Close()
	persistDone <- true

	new := NewStore[int, int](1000, false, true, nil, nil, nil, 0, 0, nil)
	f, err = os.Open("stest")
	require.Nil(t, err)
	err = new.Recover(0, f)
	require.Nil(t, err)
	f.Close()

	// read should not be blocked during persistence
	require.Greater(t, counter.Load(), uint64(100))

	for i := 0; i < 1000; i++ {
		new.Get(i)
		new.Set(i, 123, 1, 0)
	}
	new.Wait()

}
