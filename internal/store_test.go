package internal

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDequeExpire(t *testing.T) {
	store := NewStore[int, int](20000)

	expired := map[int]int{}
	store.removalListener = func(key, value int, reason RemoveReason) {
		if reason == EXPIRED {
			expired[key] = value
		}
	}
	_, index := store.index(123)
	expire := store.timerwheel.clock.expireNano(200 * time.Millisecond)
	for i := 0; i < 5; i++ {
		entry := &Entry[int, int]{key: i}
		entry.expire.Store(expire)
		store.shards[index].deque.PushFront(entry)
		store.shards[index].hashmap[i] = entry
	}
	require.True(t, len(expired) == 0)
	time.Sleep(1 * time.Second)
	for i := 0; i < 5; i++ {
		store.Set(123, 123, 1, 1*time.Second)
	}
	require.True(t, len(expired) > 0)
}
