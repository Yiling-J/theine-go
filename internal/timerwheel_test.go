package internal

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestFindBucket(t *testing.T) {
	tw := NewTimerWheel(1000, NewLockedBuf[string, string](64))
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
	tw := NewTimerWheel(1000, NewLockedBuf[string, string](64))
	entries := []*Entry[string, string]{
		{key: "k1", expire: tw.clock.nowNano() + (time.Second * time.Duration(1)).Nanoseconds()},
		{key: "k2", expire: tw.clock.nowNano() + (time.Second * time.Duration(69)).Nanoseconds()},
		{key: "k3", expire: tw.clock.nowNano() + (time.Second * time.Duration(4399)).Nanoseconds()},
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
	q := NewLockedBuf[string, string](64)
	tw := NewTimerWheel(1000, q)
	entries := []*Entry[string, string]{
		{key: "k1", expire: tw.clock.nowNano() + (time.Second * time.Duration(1)).Nanoseconds()},
		{key: "k2", expire: tw.clock.nowNano() + (time.Second * time.Duration(10)).Nanoseconds()},
		{key: "k3", expire: tw.clock.nowNano() + (time.Second * time.Duration(30)).Nanoseconds()},
		{key: "k4", expire: tw.clock.nowNano() + (time.Second * time.Duration(120)).Nanoseconds()},
		{key: "k5", expire: tw.clock.nowNano() + (time.Second * time.Duration(6500)).Nanoseconds()},
		{key: "k6", expire: tw.clock.nowNano() + (time.Second * time.Duration(142000)).Nanoseconds()},
		{key: "k7", expire: tw.clock.nowNano() + (time.Second * time.Duration(1420000)).Nanoseconds()},
	}

	for _, entry := range entries {
		tw.schedule(entry)
	}
	tw.advance(tw.clock.nowNano()+(time.Second*time.Duration(64)).Nanoseconds(), nil)
	keys := []string{}
	for _, k := range q.buf {
		if k.entry != nil {
			keys = append(keys, k.entry.key)
		}
	}
	require.ElementsMatch(t, []string{"k1", "k2", "k3"}, keys)

	tw.advance(tw.clock.nowNano()+(time.Second*time.Duration(200)).Nanoseconds(), nil)
	keys = []string{}
	for _, k := range q.buf {
		if k.entry != nil {
			keys = append(keys, k.entry.key)
		}
	}
	require.ElementsMatch(t, []string{"k1", "k2", "k3", "k4"}, keys)

	tw.advance(tw.clock.nowNano()+(time.Second*time.Duration(12000)).Nanoseconds(), nil)
	keys = []string{}
	for _, k := range q.buf {
		if k.entry != nil {
			keys = append(keys, k.entry.key)
		}
	}
	require.ElementsMatch(t, []string{"k1", "k2", "k3", "k4", "k5"}, keys)

	tw.advance(tw.clock.nowNano()+(time.Second*time.Duration(350000)).Nanoseconds(), nil)
	keys = []string{}
	for _, k := range q.buf {
		if k.entry != nil {
			keys = append(keys, k.entry.key)
		}
	}
	require.ElementsMatch(t, []string{"k1", "k2", "k3", "k4", "k5", "k6"}, keys)

	tw.advance(tw.clock.nowNano()+(time.Second*time.Duration(1520000)).Nanoseconds(), nil)
	keys = []string{}
	for _, k := range q.buf {
		if k.entry != nil {
			keys = append(keys, k.entry.key)
		}
	}
	require.ElementsMatch(t, []string{"k1", "k2", "k3", "k4", "k5", "k6", "k7"}, keys)
}
