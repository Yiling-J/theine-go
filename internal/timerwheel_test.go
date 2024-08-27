package internal

import (
	"math/rand"
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
		x, _ := tw.findIndex(tw.clock.NowNano() + (time.Second * time.Duration(i)).Nanoseconds())
		require.Equal(t, 0, x)
	}
	// max 1.22h
	for _, i := range []int{69, 120, 200, 1000, 2500, 4398} {
		x, _ := tw.findIndex(tw.clock.NowNano() + (time.Second * time.Duration(i)).Nanoseconds())
		require.Equal(t, 1, x)
	}
	// max 1.63d
	for _, i := range []int{4399, 8000, 20000, 50000, 140737} {
		x, _ := tw.findIndex(tw.clock.NowNano() + (time.Second * time.Duration(i)).Nanoseconds())
		require.Equal(t, 2, x)
	}
	// max 6.5d
	for _, i := range []int{140738, 200000, 400000, 562949} {
		x, _ := tw.findIndex(tw.clock.NowNano() + (time.Second * time.Duration(i)).Nanoseconds())
		require.Equal(t, 3, x)
	}
	// > 6.5d
	for _, i := range []int{562950, 1562950, 2562950, 3562950} {
		x, _ := tw.findIndex(tw.clock.NowNano() + (time.Second * time.Duration(i)).Nanoseconds())
		require.Equal(t, 4, x)
	}
}

func TestSchedule(t *testing.T) {
	tw := NewTimerWheel[string, string](1000)
	entries := []*Entry[string, string]{
		NewEntry("k1", "", 1, expire(tw.clock.NowNano(), 1)),
		NewEntry("k2", "", 1, expire(tw.clock.NowNano(), 69)),
		NewEntry("k3", "", 1, expire(tw.clock.NowNano(), 4399)),
	}

	for _, entry := range entries {
		tw.schedule(entry)
	}
	var found bool
	for _, l := range tw.wheel[0] {
		if l.Contains(entries[0]) {
			found = true
		}
	}
	require.True(t, found)

	found = false
	for _, l := range tw.wheel[1] {
		if l.Contains(entries[1]) {
			found = true
		}
	}
	require.True(t, found)

	found = false
	for _, l := range tw.wheel[2] {
		if l.Contains(entries[2]) {
			found = true
		}
	}
	require.True(t, found)
}

func TestAdvance(t *testing.T) {
	tw := NewTimerWheel[string, string](1000)
	entries := []*Entry[string, string]{
		NewEntry("k1", "", 1, expire(tw.clock.NowNano(), 1)),
		NewEntry("k2", "", 1, expire(tw.clock.NowNano(), 10)),
		NewEntry("k3", "", 1, expire(tw.clock.NowNano(), 30)),
		NewEntry("k4", "", 1, expire(tw.clock.NowNano(), 120)),
		NewEntry("k5", "", 1, expire(tw.clock.NowNano(), 6500)),
		NewEntry("k6", "", 1, expire(tw.clock.NowNano(), 142000)),
		NewEntry("k7", "", 1, expire(tw.clock.NowNano(), 1420000)),
	}

	for _, entry := range entries {
		tw.schedule(entry)
	}
	evicted := []string{}
	tw.advance(tw.clock.NowNano()+(time.Second*time.Duration(64)).Nanoseconds(), func(entry *Entry[string, string], reason RemoveReason) {
		evicted = append(evicted, entry.key)
	})
	require.ElementsMatch(t, []string{"k1", "k2", "k3"}, evicted)

	tw.advance(tw.clock.NowNano()+(time.Second*time.Duration(200)).Nanoseconds(), func(entry *Entry[string, string], reason RemoveReason) {
		evicted = append(evicted, entry.key)
	})

	require.ElementsMatch(t, []string{"k1", "k2", "k3", "k4"}, evicted)

	tw.advance(tw.clock.NowNano()+(time.Second*time.Duration(12000)).Nanoseconds(), func(entry *Entry[string, string], reason RemoveReason) {
		evicted = append(evicted, entry.key)
	})

	require.ElementsMatch(t, []string{"k1", "k2", "k3", "k4", "k5"}, evicted)

	tw.advance(tw.clock.NowNano()+(time.Second*time.Duration(350000)).Nanoseconds(), func(entry *Entry[string, string], reason RemoveReason) {
		evicted = append(evicted, entry.key)
	})

	require.ElementsMatch(t, []string{"k1", "k2", "k3", "k4", "k5", "k6"}, evicted)

	tw.advance(tw.clock.NowNano()+(time.Second*time.Duration(1520000)).Nanoseconds(), func(entry *Entry[string, string], reason RemoveReason) {
		evicted = append(evicted, entry.key)
	})

	require.ElementsMatch(t, []string{"k1", "k2", "k3", "k4", "k5", "k6", "k7"}, evicted)
}

func TestAdvanceClear(t *testing.T) {
	tw := NewTimerWheel[string, string](100000)
	em := []*Entry[string, string]{}
	for i := 0; i < 50000; i++ {
		ttl := 1 + rand.Intn(12)
		entry := NewEntry("k1", "", 1, expire(tw.clock.NowNano(), int64(ttl)))
		tw.schedule(entry)
		em = append(em, entry)
	}
	for _, entry := range em {
		require.NotNil(t, entry.meta.wheelPrev)
		require.NotNil(t, entry.meta.wheelNext)
	}

	for i := 0; i < 15; i++ {
		tw.advance(tw.clock.NowNano()+time.Duration(float64(i)*1.0*float64(time.Second)).Nanoseconds(), func(entry *Entry[string, string], reason RemoveReason) {
		})
	}

	for _, entry := range em {
		require.Nil(t, entry.meta.wheelPrev)
		require.Nil(t, entry.meta.wheelNext)
	}

	for _, w := range tw.wheel {
		for _, l := range w {
			require.Equal(t, &l.root, l.root.meta.wheelPrev)
			require.Equal(t, &l.root, l.root.meta.wheelNext)
		}
	}
}
