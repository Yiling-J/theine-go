package internal

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTlfu(t *testing.T) {
	hasher := NewHasher[string](nil)
	tlfu := NewTinyLfu[string, string](1000, hasher)
	require.Equal(t, uint(1000), tlfu.slru.probation.capacity)
	require.Equal(t, uint(800), tlfu.slru.protected.capacity)
	require.Equal(t, 0, int(int(tlfu.slru.probation.len.Load())))
	require.Equal(t, 0, int(int(tlfu.slru.protected.len.Load())))

	entries := []*Entry[string, string]{}
	for i := 0; i < 200; i++ {
		e := NewEntry(fmt.Sprintf("%d", i), "", 1, 0)
		evicted := tlfu.Set(e)
		entries = append(entries, e)
		require.Nil(t, evicted)
	}

	require.Equal(t, 200, int(tlfu.slru.probation.len.Load()))
	require.Equal(t, 0, int(tlfu.slru.protected.len.Load()))

	// probation -> protected
	tlfu.Access(ReadBufItem[string, string]{entry: entries[11]})
	require.Equal(t, 199, int(tlfu.slru.probation.len.Load()))
	require.Equal(t, 1, int(tlfu.slru.protected.len.Load()))
	tlfu.Access(ReadBufItem[string, string]{entry: entries[11]})
	require.Equal(t, 199, int(tlfu.slru.probation.len.Load()))
	require.Equal(t, 1, int(tlfu.slru.protected.len.Load()))

	for i := 200; i < 1000; i++ {
		e := NewEntry(fmt.Sprintf("%d", i), "", 1, 0)
		entries = append(entries, e)
		evicted := tlfu.Set(e)
		require.Nil(t, evicted)
	}
	// access protected
	tlfu.Access(ReadBufItem[string, string]{entry: entries[11]})
	require.Equal(t, 999, int(tlfu.slru.probation.len.Load()))
	require.Equal(t, 1, int(tlfu.slru.protected.len.Load()))

	evicted := tlfu.Set(NewEntry("0a", "", 1, 0))
	require.Equal(t, "0a", evicted.key)
	require.Equal(t, 999, int(tlfu.slru.probation.len.Load()))
	require.Equal(t, 1, int(tlfu.slru.protected.len.Load()))

	victim := tlfu.slru.victim()
	require.Equal(t, "0", victim.key)
	tlfu.Access(ReadBufItem[string, string]{entry: entries[991]})
	tlfu.Access(ReadBufItem[string, string]{entry: entries[991]})
	tlfu.Access(ReadBufItem[string, string]{entry: entries[991]})
	tlfu.Access(ReadBufItem[string, string]{entry: entries[991]})
	evicted = tlfu.Set(NewEntry("1a", "", 1, 0))
	require.Equal(t, "1a", evicted.key)
	require.Equal(t, 998, int(tlfu.slru.probation.len.Load()))

	entries2 := []*Entry[string, string]{}
	for i := 0; i < 1000; i++ {
		e := NewEntry(fmt.Sprintf("%d*", i), "", 1, 0)
		tlfu.Set(e)
		entries2 = append(entries2, e)
	}
	require.Equal(t, 998, int(tlfu.slru.probation.len.Load()))
	require.Equal(t, 2, int(tlfu.slru.protected.len.Load()))

	for _, i := range []int{997, 998, 999} {
		tlfu.Remove(entries2[i])
		tlfu.slru.probation.display()
		tlfu.slru.probation.displayReverse()
		tlfu.slru.protected.display()
		tlfu.slru.protected.displayReverse()
	}

}

func TestEvictEntries(t *testing.T) {
	hasher := NewHasher[string](nil)
	tlfu := NewTinyLfu[string, string](500, hasher)
	require.Equal(t, uint(500), tlfu.slru.probation.capacity)
	require.Equal(t, uint(400), tlfu.slru.protected.capacity)
	require.Equal(t, 0, int(tlfu.slru.probation.len.Load()))
	require.Equal(t, 0, int(tlfu.slru.protected.len.Load()))

	for i := 0; i < 500; i++ {
		tlfu.Set(NewEntry(fmt.Sprintf("%d:1", i), "", 1, 0))
	}
	require.Equal(t, 500, int(tlfu.slru.probation.len.Load()))
	require.Equal(t, 0, int(tlfu.slru.protected.len.Load()))
	new := NewEntry("l:10", "", 10, 0)
	new.frequency.Store(10)
	tlfu.Set(new)
	require.Equal(t, 509, int(tlfu.slru.probation.len.Load()))
	require.Equal(t, 0, int(tlfu.slru.protected.len.Load()))
	//  2. probation length is 509, so remove 9 entries from probation
	removed := tlfu.EvictEntries()
	for _, rm := range removed {
		require.True(t, strings.HasSuffix(rm.key, ":1"))
	}
	require.Equal(t, 9, len(removed))
	require.Equal(t, 500, int(tlfu.slru.probation.len.Load()))
	require.Equal(t, 0, int(tlfu.slru.protected.len.Load()))

	// put l:450 to probation, this will remove 1 entry, probation len is 949 now
	// remove 449 entries from probation
	new = NewEntry("l:450", "", 450, 0)
	new.frequency.Store(10)
	tlfu.Set(new)
	removed = tlfu.EvictEntries()
	require.Equal(t, 449, len(removed))
	require.Equal(t, 500, int(tlfu.slru.probation.len.Load()))
	require.Equal(t, 0, int(tlfu.slru.protected.len.Load()))

	// put l:460 to probation, this will remove 1 entry, probation len is 959 now
	// remove all entries except the new l:460 one
	new = NewEntry("l:460", "", 460, 0)
	new.frequency.Store(10)
	tlfu.Set(new)
	removed = tlfu.EvictEntries()
	require.Equal(t, 41, len(removed))
	require.Equal(t, 460, int(tlfu.slru.probation.len.Load()))
	require.Equal(t, 0, int(tlfu.slru.protected.len.Load()))

	// access
	tlfu.Access(ReadBufItem[string, string]{entry: new})
	require.Equal(t, 0, int(tlfu.slru.probation.len.Load()))
	require.Equal(t, 460, int(tlfu.slru.protected.len.Load()))
	new.cost = 600
	tlfu.UpdateCost(new, 140)
	removed = tlfu.EvictEntries()
	require.Equal(t, 1, len(removed))
	require.Equal(t, 0, int(tlfu.slru.probation.len.Load()))
	require.Equal(t, 0, int(tlfu.slru.protected.len.Load()))

}
