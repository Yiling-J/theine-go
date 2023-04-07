package internal

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func expire(now int64, expire int64) int64 {
	return now + (time.Second * time.Duration(expire)).Nanoseconds()
}

func TestFindBucket(t *testing.T) {
	tw := NewTimerWheel[string, string](1000)
	// max 1.14m
	for _, i := range []int{0, 10, 30, 68} {
		x, _ := tw.findIndex(tw.clock.nowNano() + (time.Second * time.Duration(i)).Nanoseconds())
		require.Equal(t, 0, x)
	}
	// max 1.22h
	for _, i := range []int{69, 120, 200, 1000, 2500, 4398} {
		x, _ := tw.findIndex(tw.clock.nowNano() + (time.Second * time.Duration(i)).Nanoseconds())
		require.Equal(t, 1, x)
	}
	// max 1.63d
	for _, i := range []int{4399, 8000, 20000, 50000, 140737} {
		x, _ := tw.findIndex(tw.clock.nowNano() + (time.Second * time.Duration(i)).Nanoseconds())
		require.Equal(t, 2, x)
	}
	// max 6.5d
	for _, i := range []int{140738, 200000, 400000, 562949} {
		x, _ := tw.findIndex(tw.clock.nowNano() + (time.Second * time.Duration(i)).Nanoseconds())
		require.Equal(t, 3, x)
	}
	// > 6.5d
	for _, i := range []int{562950, 1562950, 2562950, 3562950} {
		x, _ := tw.findIndex(tw.clock.nowNano() + (time.Second * time.Duration(i)).Nanoseconds())
		require.Equal(t, 4, x)
	}
}

func TestSchedule(t *testing.T) {
	tw := NewTimerWheel[string, string](1000)
	entries := []*Entry[string, string]{
		NewEntry[string, string]("k1", "", 1, expire(tw.clock.nowNano(), 1)),
		NewEntry[string, string]("k2", "", 1, expire(tw.clock.nowNano(), 69)),
		NewEntry[string, string]("k3", "", 1, expire(tw.clock.nowNano(), 4399)),
	}

	for _, entry := range entries {
		tw.schedule(entry)
	}
	var found bool
	for _, l := range tw.wheel[0] {
		if l == entries[0].list(WHEEL_LIST) {
			found = true
		}
	}
	require.True(t, found)

	found = false
	for _, l := range tw.wheel[1] {
		if l == entries[1].list(WHEEL_LIST) {
			found = true
		}
	}
	require.True(t, found)

	found = false
	for _, l := range tw.wheel[2] {
		if l == entries[2].list(WHEEL_LIST) {
			found = true
		}
	}
	require.True(t, found)
}

func TestAdvance(t *testing.T) {
	tw := NewTimerWheel[string, string](1000)
	entries := []*Entry[string, string]{
		NewEntry[string, string]("k1", "", 1, expire(tw.clock.nowNano(), 1)),
		NewEntry[string, string]("k2", "", 1, expire(tw.clock.nowNano(), 10)),
		NewEntry[string, string]("k3", "", 1, expire(tw.clock.nowNano(), 30)),
		NewEntry[string, string]("k4", "", 1, expire(tw.clock.nowNano(), 120)),
		NewEntry[string, string]("k5", "", 1, expire(tw.clock.nowNano(), 6500)),
		NewEntry[string, string]("k6", "", 1, expire(tw.clock.nowNano(), 142000)),
		NewEntry[string, string]("k7", "", 1, expire(tw.clock.nowNano(), 1420000)),
	}

	for _, entry := range entries {
		tw.schedule(entry)
	}
	evicted := []string{}
	tw.advance(tw.clock.nowNano()+(time.Second*time.Duration(64)).Nanoseconds(), func(entry *Entry[string, string]) {
		evicted = append(evicted, entry.key)
	})
	require.ElementsMatch(t, []string{"k1", "k2", "k3"}, evicted)

	tw.advance(tw.clock.nowNano()+(time.Second*time.Duration(200)).Nanoseconds(), func(entry *Entry[string, string]) {
		evicted = append(evicted, entry.key)
	})

	require.ElementsMatch(t, []string{"k1", "k2", "k3", "k4"}, evicted)

	tw.advance(tw.clock.nowNano()+(time.Second*time.Duration(12000)).Nanoseconds(), func(entry *Entry[string, string]) {
		evicted = append(evicted, entry.key)
	})

	require.ElementsMatch(t, []string{"k1", "k2", "k3", "k4", "k5"}, evicted)

	tw.advance(tw.clock.nowNano()+(time.Second*time.Duration(350000)).Nanoseconds(), func(entry *Entry[string, string]) {
		evicted = append(evicted, entry.key)
	})

	require.ElementsMatch(t, []string{"k1", "k2", "k3", "k4", "k5", "k6"}, evicted)

	tw.advance(tw.clock.nowNano()+(time.Second*time.Duration(1520000)).Nanoseconds(), func(entry *Entry[string, string]) {
		evicted = append(evicted, entry.key)
	})

	require.ElementsMatch(t, []string{"k1", "k2", "k3", "k4", "k5", "k6", "k7"}, evicted)
}
