package internal

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTlfu_Basic(t *testing.T) {
	hasher := NewHasher[string](nil)
	tlfu := NewTinyLfu[string, string](1000, hasher)
	require.Equal(t, 10, int(tlfu.window.capacity))
	require.Equal(t, 990, int(tlfu.slru.probation.capacity))
	require.Equal(t, 792, int(tlfu.slru.protected.capacity))
	require.Equal(t, 0, int(tlfu.slru.probation.Len()))
	require.Equal(t, 0, int(tlfu.slru.protected.Len()))

	entries := []*Entry[string, string]{}
	for i := 0; i < 200; i++ {
		e := NewEntry(fmt.Sprintf("%d", i), "", 1, 0)
		evicted := tlfu.Set(e)
		entries = append(entries, e)
		require.Nil(t, evicted)
	}

	require.Equal(t, 10, int(tlfu.window.Len()))
	require.Equal(t, 190, int(tlfu.slru.probation.Len()))
	require.Equal(t, 0, int(tlfu.slru.protected.Len()))

	// probation -> protected
	tlfu.Access(ReadBufItem[string, string]{entry: entries[11]})
	require.Equal(t, 189, int(tlfu.slru.probation.Len()))
	require.Equal(t, 1, int(tlfu.slru.protected.Len()))
	tlfu.Access(ReadBufItem[string, string]{entry: entries[11]})
	require.Equal(t, 189, int(tlfu.slru.probation.Len()))
	require.Equal(t, 1, int(tlfu.slru.protected.Len()))

	for i := 200; i < 1000; i++ {
		e := NewEntry(fmt.Sprintf("%d", i), "", 1, 0)
		entries = append(entries, e)
		evicted := tlfu.Set(e)
		require.Nil(t, evicted)
	}
	// access protected
	tlfu.Access(ReadBufItem[string, string]{entry: entries[11]})
	require.Equal(t, 989, int(tlfu.slru.probation.Len()))
	require.Equal(t, 1, int(tlfu.slru.protected.Len()))

	evicted := tlfu.Set(NewEntry("0a", "", 1, 0))
	// 990 is the last entry of window
	require.Equal(t, "990", evicted.key)
	require.Equal(t, 989, int(tlfu.slru.probation.Len()))
	require.Equal(t, 1, int(tlfu.slru.protected.Len()))

	victim := tlfu.slru.victim()
	require.Equal(t, "0", victim.key)
	// increase winodw last entry frequency
	// also 991 is move to LRU head, so window tail is 992
	tlfu.Access(ReadBufItem[string, string]{entry: entries[991]})
	tlfu.Access(ReadBufItem[string, string]{entry: entries[991]})
	tlfu.Access(ReadBufItem[string, string]{entry: entries[991]})
	tlfu.Access(ReadBufItem[string, string]{entry: entries[991]})
	evicted = tlfu.Set(NewEntry("1a", "", 1, 0))
	require.Equal(t, "992", evicted.key)
	require.Equal(t, 989, int(tlfu.slru.probation.Len()))

	entries2 := []*Entry[string, string]{}
	for i := 0; i < 1000; i++ {
		e := NewEntry(fmt.Sprintf("%d*", i), "", 1, 0)
		tlfu.Set(e)
		entries2 = append(entries2, e)
	}
	require.Equal(t, 989, int(tlfu.slru.probation.Len()))
	require.Equal(t, 1, int(tlfu.slru.protected.Len()))

}

func TestTlfu_EvictEntries(t *testing.T) {
	hasher := NewHasher[string](nil)
	tlfu := NewTinyLfu[string, string](500, hasher)
	require.Equal(t, 5, int(tlfu.window.capacity))
	require.Equal(t, 495, int(tlfu.slru.probation.capacity))
	require.Equal(t, 396, int(tlfu.slru.protected.capacity))
	require.Equal(t, 0, int(tlfu.slru.probation.Len()))
	require.Equal(t, 0, int(tlfu.slru.protected.Len()))
	em := []*Entry[string, string]{}
	tlfu.removeCallback = func(entry *Entry[string, string]) {
		em = append(em, entry)
	}

	for i := 0; i < 500; i++ {
		tlfu.Set(NewEntry(fmt.Sprintf("%d:1", i), "", 1, 0))
	}
	require.Equal(t, 5, int(tlfu.window.Len()))
	require.Equal(t, 495, int(tlfu.slru.probation.Len()))
	require.Equal(t, 0, int(tlfu.slru.protected.Len()))
	new := NewEntry("l:10", "", 10, 0)
	tlfu.sketch.Addn(hasher.hash(new.key), 10)
	tlfu.Set(new)
	require.Equal(t, 14, int(tlfu.window.Len()))
	require.Equal(t, 495, int(tlfu.slru.probation.Len()))
	require.Equal(t, 0, int(tlfu.slru.protected.Len()))

	// 2. window length is 14,
	// because window only have 5 entries, all will be removed,
	// evicted ones are send to probation, so probation will
	// remove 9 entries
	tlfu.EvictEntries()
	for _, rm := range em {
		require.True(t, strings.HasSuffix(rm.key, ":1"))
	}
	require.Equal(t, 9, len(em))
	require.Equal(t, 0, int(tlfu.window.Len()))
	require.Equal(t, 495, int(tlfu.slru.probation.Len()))
	require.Equal(t, 0, int(tlfu.slru.protected.Len()))
	// reset evicted list
	em = []*Entry[string, string]{}

	// insert l:450 to window, evicted from window to probation,
	// this will remove 1 entry, probation len is 944 now
	// remove 449 entries from probation
	new = NewEntry("l:450", "", 450, 0)
	tlfu.sketch.Addn(hasher.hash(new.key), 10)
	tlfu.Set(new)
	tlfu.EvictEntries()
	require.Equal(t, 449, len(em))
	require.Equal(t, 495, int(tlfu.slru.probation.Len()))
	require.Equal(t, 0, int(tlfu.slru.protected.Len()))
	// reset evicted list
	em = []*Entry[string, string]{}

	// insert l:460 to window, evicted from window to probation,
	// this will remove 1 entry, probation len is 954 now
	// remove all entries except the new l:460 one
	new = NewEntry("l:460", "", 460, 0)
	tlfu.sketch.Addn(hasher.hash(new.key), 10)
	tlfu.Set(new)
	tlfu.EvictEntries()
	require.Equal(t, 36, len(em))
	require.Equal(t, 460, int(tlfu.slru.probation.Len()))
	require.Equal(t, 0, int(tlfu.slru.protected.Len()))
	// reset evicted list
	em = []*Entry[string, string]{}

	// access
	tlfu.Access(ReadBufItem[string, string]{entry: new})
	require.Equal(t, 0, int(tlfu.slru.probation.Len()))
	require.Equal(t, 460, int(tlfu.slru.protected.Len()))
	new.weight.Store(600)
	tlfu.UpdateCost(new, 140)
	tlfu.EvictEntries()
	require.Equal(t, 1, len(em))
	require.Equal(t, 0, int(tlfu.slru.probation.Len()))
	require.Equal(t, 140, int(tlfu.slru.protected.Len()))

}

func TestTlfu_ClimbResize(t *testing.T) {
	hasher := NewHasher[int](nil)
	tlfu := NewTinyLfu[int, int](500, hasher)
	em := map[int]*Entry[int, int]{}

	for i := 0; i < 500; i++ {
		e := NewEntry(i, i, 1, 0)
		em[e.key] = e
		tlfu.Set(e)
	}
	// 495-500 in window, others in probation
	require.Equal(t, 5, int(tlfu.window.capacity))
	require.Equal(t, 495, int(tlfu.slru.probation.capacity))
	require.Equal(t, 396, int(tlfu.slru.protected.capacity))
	require.Equal(t, 495, int(tlfu.slru.maxsize))

	require.Equal(t, 5, tlfu.window.Len())
	require.Equal(t, 495, tlfu.slru.probation.Len())
	require.Equal(t, 0, tlfu.slru.protected.Len())

	for i := 0; i < 380; i++ {
		tlfu.Access(ReadBufItem[int, int]{
			entry: em[i],
			hash:  tlfu.hasher.hash(i),
		})
	}

	require.Equal(t, 5, tlfu.window.Len())
	require.Equal(t, 115, tlfu.slru.probation.Len())
	require.Equal(t, 380, tlfu.slru.protected.Len())
	require.Equal(t, float32(-31.25), tlfu.step)

	// hr 0 -> 0.2, window size decrease from 5 to 1
	tlfu.hits.Add(20)
	tlfu.misses.Add(80)
	tlfu.climb()
	require.Equal(t, 1, int(tlfu.windowSize))
	tlfu.resizeWindow()
	// cap change
	require.Equal(t, 1, int(tlfu.window.capacity))
	require.Equal(t, 499, int(tlfu.slru.probation.capacity))
	require.Equal(t, 400, int(tlfu.slru.protected.capacity))
	require.Equal(t, 499, int(tlfu.slru.maxsize))
	// evicted from window -> insert into probation
	require.Equal(t, 1, tlfu.window.Len())
	require.Equal(t, 380, tlfu.slru.protected.Len())
	require.Equal(t, 119, tlfu.slru.probation.Len())

	// hr 0.2 -> 0.22, window size still 1, resize noop
	tlfu.hits.Add(22)
	tlfu.misses.Add(78)
	tlfu.climb()
	require.Equal(t, 1, int(tlfu.windowSize))
	tlfu.resizeWindow()
	require.Equal(t, 1, int(tlfu.window.capacity))
	require.Equal(t, 499, int(tlfu.slru.probation.capacity))
	require.Equal(t, 400, int(tlfu.slru.protected.capacity))
	require.Equal(t, 499, int(tlfu.slru.maxsize))
	require.Equal(t, 1, tlfu.window.Len())
	require.Equal(t, 380, tlfu.slru.protected.Len())
	require.Equal(t, 119, tlfu.slru.probation.Len())

	// hr 0.22 -> 0.2, window size increase to 31
	// step alread decay twice: 31*0.98*0.98 = 30
	tlfu.hits.Add(20)
	tlfu.misses.Add(80)
	tlfu.climb()
	require.Equal(t, 31, int(tlfu.windowSize))
	tlfu.resizeWindow()
	// cap change
	require.Equal(t, 31, int(tlfu.window.capacity))
	require.Equal(t, 469, int(tlfu.slru.probation.capacity))
	require.Equal(t, 370, int(tlfu.slru.protected.capacity))
	require.Equal(t, 469, int(tlfu.slru.maxsize))
	// evict from protected, insert into probation(1/129/370)
	// evict from probation, insert into window(31/99/370)
	require.Equal(t, 31, tlfu.window.Len())
	require.Equal(t, 370, tlfu.slru.protected.Len())
	require.Equal(t, 99, tlfu.slru.probation.Len())

	// increase window 10 times
	step := tlfu.step
	for i := 1; i < 11; i++ {
		tlfu.hits.Add(uint64(20 + i))
		tlfu.misses.Add(uint64(80 - i))
		tlfu.climb()
		cr := tlfu.step / step
		step = tlfu.step
		require.True(t, cr >= 0.979 && cr <= 0.981)
		tlfu.resizeWindow()
	}
	require.Equal(t, 302, int(tlfu.window.capacity))
	require.Equal(t, 198, int(tlfu.slru.probation.capacity))
	require.Equal(t, 99, int(tlfu.slru.protected.capacity))
	require.Equal(t, 198, int(tlfu.slru.maxsize))
	require.Equal(t, 302, tlfu.window.Len())
	require.Equal(t, 99, tlfu.slru.protected.Len())
	require.Equal(t, 99, tlfu.slru.probation.Len())

	// step reset to default
	tlfu.hits.Add(uint64(20))
	tlfu.misses.Add(uint64(80))
	tlfu.climb()
	tlfu.resizeWindow()
	require.Equal(t, float32(-31.25), tlfu.step)

}
