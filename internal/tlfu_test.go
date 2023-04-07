package internal

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTlfu(t *testing.T) {
	hasher := NewHasher[string]()
	tlfu := NewTinyLfu[string, string](1000, hasher)
	require.Equal(t, uint(10), tlfu.lru.list.capacity)
	require.Equal(t, uint(990), tlfu.slru.probation.capacity)
	require.Equal(t, uint(792), tlfu.slru.protected.capacity)
	require.Equal(t, 0, tlfu.lru.list.len)
	require.Equal(t, 0, tlfu.slru.probation.len)
	require.Equal(t, 0, tlfu.slru.protected.len)

	entries := []*Entry[string, string]{}
	for i := 0; i < 200; i++ {
		e := NewEntry(fmt.Sprintf("%d", i), "", 1, 0)
		evicted := tlfu.Set(e)
		entries = append(entries, e)
		require.Nil(t, evicted)
	}

	require.Equal(t, "199/198/197/196/195/194/193/192/191/190", tlfu.lru.list.display(LIST))
	require.Equal(t, "190/191/192/193/194/195/196/197/198/199", tlfu.lru.list.displayReverse(LIST))
	require.Equal(t, 10, tlfu.lru.list.len)
	require.Equal(t, 190, tlfu.slru.probation.len)
	require.Equal(t, 0, tlfu.slru.protected.len)

	// probation -> protected
	tlfu.Access(ReadBufItem[string, string]{entry: entries[11]})
	require.Equal(t, 10, tlfu.lru.list.len)
	require.Equal(t, 189, tlfu.slru.probation.len)
	require.Equal(t, 1, tlfu.slru.protected.len)
	tlfu.Access(ReadBufItem[string, string]{entry: entries[11]})
	require.Equal(t, 10, tlfu.lru.list.len)
	require.Equal(t, 189, tlfu.slru.probation.len)
	require.Equal(t, 1, tlfu.slru.protected.len)

	for i := 200; i < 1000; i++ {
		e := NewEntry(fmt.Sprintf("%d", i), "", 1, 0)
		entries = append(entries, e)
		evicted := tlfu.Set(e)
		require.Nil(t, evicted)
	}
	// access protected
	tlfu.Access(ReadBufItem[string, string]{entry: entries[11]})
	require.Equal(t, 10, tlfu.lru.list.len)
	require.Equal(t, 989, tlfu.slru.probation.len)
	require.Equal(t, 1, tlfu.slru.protected.len)

	evicted := tlfu.Set(NewEntry("0a", "", 1, 0))
	require.Equal(t, "990", evicted.key)
	require.Equal(t, 10, tlfu.lru.list.len)
	require.Equal(t, 989, tlfu.slru.probation.len)
	require.Equal(t, 1, tlfu.slru.protected.len)

	victim1 := tlfu.lru.list.Back()
	require.Equal(t, entries[991].key, victim1.key)
	victim := tlfu.slru.victim()
	require.Equal(t, "0", victim.key)
	tlfu.Access(ReadBufItem[string, string]{entry: entries[991]})
	tlfu.Access(ReadBufItem[string, string]{entry: entries[991]})
	tlfu.Access(ReadBufItem[string, string]{entry: entries[991]})
	tlfu.Access(ReadBufItem[string, string]{entry: entries[991]})
	evicted = tlfu.Set(NewEntry("1a", "", 1, 0))
	require.Equal(t, entries[992].key, evicted.key)
	require.Equal(t, 989, tlfu.slru.probation.len)

	entries2 := []*Entry[string, string]{}
	for i := 0; i < 1000; i++ {
		e := NewEntry(fmt.Sprintf("%d*", i), "", 1, 0)
		tlfu.Set(e)
		entries2 = append(entries2, e)
	}
	require.Equal(t, 10, tlfu.lru.list.len)
	require.Equal(t, 989, tlfu.slru.probation.len)
	require.Equal(t, 1, tlfu.slru.protected.len)

	require.Equal(t, "999*/998*/997*/996*/995*/994*/993*/992*/991*/990*", tlfu.lru.list.display(LIST))
	require.Equal(
		t, "990*/991*/992*/993*/994*/995*/996*/997*/998*/999*", tlfu.lru.list.displayReverse(LIST),
	)
	for _, i := range []int{997, 998, 999} {
		tlfu.Remove(entries2[i])
		tlfu.slru.probation.display(LIST)
		tlfu.slru.probation.displayReverse(LIST)
		tlfu.slru.protected.display(LIST)
		tlfu.slru.protected.displayReverse(LIST)
	}

}

func TestEvictEntries(t *testing.T) {
	hasher := NewHasher[string]()
	tlfu := NewTinyLfu[string, string](500, hasher)
	require.Equal(t, uint(5), tlfu.lru.list.capacity)
	require.Equal(t, uint(495), tlfu.slru.probation.capacity)
	require.Equal(t, uint(396), tlfu.slru.protected.capacity)
	require.Equal(t, 0, tlfu.lru.list.len)
	require.Equal(t, 0, tlfu.slru.probation.len)
	require.Equal(t, 0, tlfu.slru.protected.len)

	for i := 0; i < 500; i++ {
		tlfu.Set(NewEntry(fmt.Sprintf("%d:1", i), "", 1, 0))
	}
	require.Equal(t, 5, tlfu.lru.list.len)
	require.Equal(t, 495, tlfu.slru.probation.len)
	require.Equal(t, 0, tlfu.slru.protected.len)
	tlfu.Set(NewEntry("l:10", "", 10, 0))
	require.Equal(t, 14, tlfu.lru.list.len)
	require.Equal(t, 495, tlfu.slru.probation.len)
	require.Equal(t, 0, tlfu.slru.protected.len)
	//  1. put all lru entries to probation, probation is full so 5 entries will be removed
	//  2. probation length is 495 + 9, so remove 9 entries from probation
	// 487 entries left(remove 14, insert 1)
	removed := tlfu.EvictEntries()
	for _, rm := range removed {
		require.True(t, strings.HasSuffix(rm.key, ":1"))
	}
	require.Equal(t, 14, len(removed))
	require.Equal(t, 0, tlfu.lru.list.len)
	require.Equal(t, 495, tlfu.slru.probation.len)
	require.Equal(t, 0, tlfu.slru.protected.len)

	// 1. inset l:450 to lru
	// 2. put l:450 to probation, this will remove 1 entry, probation len is 944 now
	// 3. remove 44 entries from probation
	// 37 entries left(remove 450, insert 1)
	tlfu.Set(NewEntry("l:450", "", 450, 0))
	removed = tlfu.EvictEntries()
	require.Equal(t, 450, len(removed))
	require.Equal(t, 0, tlfu.lru.list.len)
	require.Equal(t, 495, tlfu.slru.probation.len)
	require.Equal(t, 0, tlfu.slru.protected.len)

	// 1. inset l:460 to lru
	// 2. put l:460 to probation, this will remove 1 entry, probation len is 954 now
	// 3. remove all entries except the new l:460 one
	tlfu.Set(NewEntry("l:460", "", 460, 0))
	removed = tlfu.EvictEntries()
	require.Equal(t, 37, len(removed))
	require.Equal(t, 0, tlfu.lru.list.len)
	require.Equal(t, 460, tlfu.slru.probation.len)
	require.Equal(t, 0, tlfu.slru.protected.len)
}
